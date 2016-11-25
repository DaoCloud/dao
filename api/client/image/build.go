package image

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"golang.org/x/net/context"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/reference"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type buildOptions struct {
	context        string
	dockerfileName string
	tags           opts.ListOpts
	labels         opts.ListOpts
	buildArgs      opts.ListOpts
	ulimits        *runconfigopts.UlimitOpt
	memory         string
	memorySwap     string
	shmSize        string
	cpuShares      int64
	cpuPeriod      int64
	cpuQuota       int64
	cpuSetCpus     string
	cpuSetMems     string
	cgroupParent   string
	isolation      string
	quiet          bool
	noCache        bool
	rm             bool
	forceRm        bool
	pull           bool
}

// NewBuildCommand creates a new `docker build` command
func NewBuildCommand(dockerCli *client.DockerCli) *cobra.Command {
	ulimits := make(map[string]*units.Ulimit)
	options := buildOptions{
		tags:      opts.NewListOpts(validateTag),
		buildArgs: opts.NewListOpts(runconfigopts.ValidateEnv),
		ulimits:   runconfigopts.NewUlimitOpt(&ulimits),
		labels:    opts.NewListOpts(runconfigopts.ValidateEnv),
	}

	cmd := &cobra.Command{
		Use:   "build [OPTIONS] PATH | URL | -",
		Short: "从一个Dockerfile构建新的镜像",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.context = args[0]
			return runBuild(dockerCli, options)
		},
	}

	flags := cmd.Flags()

	flags.VarP(&options.tags, "tag", "t", "镜像名称以及可选的标签，若指定标签，格式为：'名称:标签' ")
	flags.Var(&options.buildArgs, "build-arg", "设置构建时的环境变量")
	flags.Var(options.ulimits, "ulimit", "设置Ulimit参数")
	flags.StringVarP(&options.dockerfileName, "file", "f", "", "Dockerfile的名称(默认为当前目录下的Dockerfile文件路径)")
	flags.StringVarP(&options.memory, "memory", "m", "", "内存限制")
	flags.StringVar(&options.memorySwap, "memory-swap", "", "交换内存限制 等于 实际内存 ＋ 交换区内存: '-1' 代表启用不受限的交换区内存")
	flags.StringVar(&options.shmSize, "shm-size", "", "共享内存/dev/shm 的大小, 默认值是64MB")
	flags.Int64VarP(&options.cpuShares, "cpu-shares", "c", 0, "CPU计算资源的值(相对值)")
	flags.Int64Var(&options.cpuPeriod, "cpu-period", 0, "限制CPU绝对公平调度算法(CFS)的时间周期")
	flags.Int64Var(&options.cpuQuota, "cpu-quota", 0, "限制CPU绝对公平调度算法(CFS)的时间限额")
	flags.StringVar(&options.cpuSetCpus, "cpuset-cpus", "", "允许容器执行的CPU核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flags.StringVar(&options.cpuSetMems, "cpuset-mems", "", "允许容器执行的CPU内存所在核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flags.StringVar(&options.cgroupParent, "cgroup-parent", "", "容器的可选cgroup父路径")
	flags.StringVar(&options.isolation, "isolation", "", "设置容器的隔离策略")
	flags.Var(&options.labels, "label", "为一个镜像设置元数据")
	flags.BoolVar(&options.noCache, "no-cache", false, "构建镜像时不使用镜像缓存")
	flags.BoolVar(&options.rm, "rm", true, "成狗构建后删除中间容器")
	flags.BoolVar(&options.forceRm, "force-rm", false, "总是产出中间结果的容器")
	flags.BoolVarP(&options.quiet, "quiet", "q", false, "压缩构建输出，并在构建成功时打印镜像ID")
	flags.BoolVar(&options.pull, "pull", false, "总是尝试下拉最新版本的镜像")

	client.AddTrustedFlags(flags, true)

	return cmd
}

func runBuild(dockerCli *client.DockerCli, options buildOptions) error {

	var (
		buildCtx io.ReadCloser
		err      error
	)

	specifiedContext := options.context

	var (
		contextDir    string
		tempDir       string
		relDockerfile string
		progBuff      io.Writer
		buildBuff     io.Writer
	)

	progBuff = dockerCli.Out()
	buildBuff = dockerCli.Out()
	if options.quiet {
		progBuff = bytes.NewBuffer(nil)
		buildBuff = bytes.NewBuffer(nil)
	}

	switch {
	case specifiedContext == "-":
		buildCtx, relDockerfile, err = builder.GetContextFromReader(dockerCli.In(), options.dockerfileName)
	case urlutil.IsGitURL(specifiedContext):
		tempDir, relDockerfile, err = builder.GetContextFromGitURL(specifiedContext, options.dockerfileName)
	case urlutil.IsURL(specifiedContext):
		buildCtx, relDockerfile, err = builder.GetContextFromURL(progBuff, specifiedContext, options.dockerfileName)
	default:
		contextDir, relDockerfile, err = builder.GetContextFromLocalDir(specifiedContext, options.dockerfileName)
	}

	if err != nil {
		if options.quiet && urlutil.IsURL(specifiedContext) {
			fmt.Fprintln(dockerCli.Err(), progBuff)
		}
		return fmt.Errorf("准备构建上下文失败: %s", err)
	}

	if tempDir != "" {
		defer os.RemoveAll(tempDir)
		contextDir = tempDir
	}

	if buildCtx == nil {
		// And canonicalize dockerfile name to a platform-independent one
		relDockerfile, err = archive.CanonicalTarNameForPath(relDockerfile)
		if err != nil {
			return fmt.Errorf("不能规范Dockerfile路径 %s: %v", relDockerfile, err)
		}

		f, err := os.Open(filepath.Join(contextDir, ".dockerignore"))
		if err != nil && !os.IsNotExist(err) {
			return err
		}

		var excludes []string
		if err == nil {
			excludes, err = dockerignore.ReadAll(f)
			if err != nil {
				return err
			}
		}

		if err := builder.ValidateContextDirectory(contextDir, excludes); err != nil {
			return fmt.Errorf("检验构建上下文出错: '%s'.", err)
		}

		// If .dockerignore mentions .dockerignore or the Dockerfile
		// then make sure we send both files over to the daemon
		// because Dockerfile is, obviously, needed no matter what, and
		// .dockerignore is needed to know if either one needs to be
		// removed. The daemon will remove them for us, if needed, after it
		// parses the Dockerfile. Ignore errors here, as they will have been
		// caught by validateContextDirectory above.
		var includes = []string{"."}
		keepThem1, _ := fileutils.Matches(".dockerignore", excludes)
		keepThem2, _ := fileutils.Matches(relDockerfile, excludes)
		if keepThem1 || keepThem2 {
			includes = append(includes, ".dockerignore", relDockerfile)
		}

		buildCtx, err = archive.TarWithOptions(contextDir, &archive.TarOptions{
			Compression:     archive.Uncompressed,
			ExcludePatterns: excludes,
			IncludeFiles:    includes,
		})
		if err != nil {
			return err
		}
	}

	ctx := context.Background()

	var resolvedTags []*resolvedTag
	if client.IsTrusted() {
		// Wrap the tar archive to replace the Dockerfile entry with the rewritten
		// Dockerfile which uses trusted pulls.
		buildCtx = replaceDockerfileTarWrapper(ctx, buildCtx, relDockerfile, dockerCli.TrustedReference, &resolvedTags)
	}

	// Setup an upload progress bar
	progressOutput := streamformatter.NewStreamFormatter().NewProgressOutput(progBuff, true)

	var body io.Reader = progress.NewProgressReader(buildCtx, progressOutput, 0, "", "Sending build context to Docker daemon")

	var memory int64
	if options.memory != "" {
		parsedMemory, err := units.RAMInBytes(options.memory)
		if err != nil {
			return err
		}
		memory = parsedMemory
	}

	var memorySwap int64
	if options.memorySwap != "" {
		if options.memorySwap == "-1" {
			memorySwap = -1
		} else {
			parsedMemorySwap, err := units.RAMInBytes(options.memorySwap)
			if err != nil {
				return err
			}
			memorySwap = parsedMemorySwap
		}
	}

	var shmSize int64
	if options.shmSize != "" {
		shmSize, err = units.RAMInBytes(options.shmSize)
		if err != nil {
			return err
		}
	}

	buildOptions := types.ImageBuildOptions{
		Memory:         memory,
		MemorySwap:     memorySwap,
		Tags:           options.tags.GetAll(),
		SuppressOutput: options.quiet,
		NoCache:        options.noCache,
		Remove:         options.rm,
		ForceRemove:    options.forceRm,
		PullParent:     options.pull,
		Isolation:      container.Isolation(options.isolation),
		CPUSetCPUs:     options.cpuSetCpus,
		CPUSetMems:     options.cpuSetMems,
		CPUShares:      options.cpuShares,
		CPUQuota:       options.cpuQuota,
		CPUPeriod:      options.cpuPeriod,
		CgroupParent:   options.cgroupParent,
		Dockerfile:     relDockerfile,
		ShmSize:        shmSize,
		Ulimits:        options.ulimits.GetList(),
		BuildArgs:      runconfigopts.ConvertKVStringsToMap(options.buildArgs.GetAll()),
		AuthConfigs:    dockerCli.RetrieveAuthConfigs(),
		Labels:         runconfigopts.ConvertKVStringsToMap(options.labels.GetAll()),
	}

	response, err := dockerCli.Client().ImageBuild(ctx, body, buildOptions)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	err = jsonmessage.DisplayJSONMessagesStream(response.Body, buildBuff, dockerCli.OutFd(), dockerCli.IsTerminalOut(), nil)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			if options.quiet {
				fmt.Fprintf(dockerCli.Err(), "%s%s", progBuff, buildBuff)
			}
			return cli.StatusError{Status: jerr.Message, StatusCode: jerr.Code}
		}
	}

	// Windows: show error message about modified file permissions if the
	// daemon isn't running Windows.
	if response.OSType != "windows" && runtime.GOOS == "windows" {
		fmt.Fprintln(dockerCli.Err(), `安全警告: 您在一个非Windows的Docker引擎上构建Windows机器的Docker的镜像。所有添加到构建上下文的文件和目录将添加'-rwxr-xr-x'权限。推荐您为敏感的文件和目录进行权限的重复查验。`)
	}

	// Everything worked so if -q was provided the output from the daemon
	// should be just the image ID and we'll print that to stdout.
	if options.quiet {
		fmt.Fprintf(dockerCli.Out(), "%s", buildBuff)
	}

	if client.IsTrusted() {
		// Since the build was successful, now we must tag any of the resolved
		// images from the above Dockerfile rewrite.
		for _, resolved := range resolvedTags {
			if err := dockerCli.TagTrusted(ctx, resolved.digestRef, resolved.tagRef); err != nil {
				return err
			}
		}
	}

	return nil
}

type translatorFunc func(context.Context, reference.NamedTagged) (reference.Canonical, error)

// validateTag checks if the given image name can be resolved.
func validateTag(rawRepo string) (string, error) {
	_, err := reference.ParseNamed(rawRepo)
	if err != nil {
		return "", err
	}

	return rawRepo, nil
}

var dockerfileFromLinePattern = regexp.MustCompile(`(?i)^[\s]*FROM[ \f\r\t\v]+(?P<image>[^ \f\r\t\v\n#]+)`)

// resolvedTag records the repository, tag, and resolved digest reference
// from a Dockerfile rewrite.
type resolvedTag struct {
	digestRef reference.Canonical
	tagRef    reference.NamedTagged
}

// rewriteDockerfileFrom rewrites the given Dockerfile by resolving images in
// "FROM <image>" instructions to a digest reference. `translator` is a
// function that takes a repository name and tag reference and returns a
// trusted digest reference.
func rewriteDockerfileFrom(ctx context.Context, dockerfile io.Reader, translator translatorFunc) (newDockerfile []byte, resolvedTags []*resolvedTag, err error) {
	scanner := bufio.NewScanner(dockerfile)
	buf := bytes.NewBuffer(nil)

	// Scan the lines of the Dockerfile, looking for a "FROM" line.
	for scanner.Scan() {
		line := scanner.Text()

		matches := dockerfileFromLinePattern.FindStringSubmatch(line)
		if matches != nil && matches[1] != api.NoBaseImageSpecifier {
			// Replace the line with a resolved "FROM repo@digest"
			ref, err := reference.ParseNamed(matches[1])
			if err != nil {
				return nil, nil, err
			}
			ref = reference.WithDefaultTag(ref)
			if ref, ok := ref.(reference.NamedTagged); ok && client.IsTrusted() {
				trustedRef, err := translator(ctx, ref)
				if err != nil {
					return nil, nil, err
				}

				line = dockerfileFromLinePattern.ReplaceAllLiteralString(line, fmt.Sprintf("FROM %s", trustedRef.String()))
				resolvedTags = append(resolvedTags, &resolvedTag{
					digestRef: trustedRef,
					tagRef:    ref,
				})
			}
		}

		_, err := fmt.Fprintln(buf, line)
		if err != nil {
			return nil, nil, err
		}
	}

	return buf.Bytes(), resolvedTags, scanner.Err()
}

// replaceDockerfileTarWrapper wraps the given input tar archive stream and
// replaces the entry with the given Dockerfile name with the contents of the
// new Dockerfile. Returns a new tar archive stream with the replaced
// Dockerfile.
func replaceDockerfileTarWrapper(ctx context.Context, inputTarStream io.ReadCloser, dockerfileName string, translator translatorFunc, resolvedTags *[]*resolvedTag) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		tarReader := tar.NewReader(inputTarStream)
		tarWriter := tar.NewWriter(pipeWriter)

		defer inputTarStream.Close()

		for {
			hdr, err := tarReader.Next()
			if err == io.EOF {
				// Signals end of archive.
				tarWriter.Close()
				pipeWriter.Close()
				return
			}
			if err != nil {
				pipeWriter.CloseWithError(err)
				return
			}

			var content io.Reader = tarReader
			if hdr.Name == dockerfileName {
				// This entry is the Dockerfile. Since the tar archive was
				// generated from a directory on the local filesystem, the
				// Dockerfile will only appear once in the archive.
				var newDockerfile []byte
				newDockerfile, *resolvedTags, err = rewriteDockerfileFrom(ctx, content, translator)
				if err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				hdr.Size = int64(len(newDockerfile))
				content = bytes.NewBuffer(newDockerfile)
			}

			if err := tarWriter.WriteHeader(hdr); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}

			if _, err := io.Copy(tarWriter, content); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}
	}()

	return pipeReader
}
