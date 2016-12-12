package client

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/builder/dockerignore"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	"github.com/docker/docker/pkg/gitutils"
	"github.com/docker/docker/pkg/httputils"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/jsonmessage"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/docker/reference"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
)

type translatorFunc func(reference.NamedTagged) (reference.Canonical, error)

// CmdBuild builds a new image from the source code at a given path.
//
// If '-' is provided instead of a path or URL, Docker will build an image from either a Dockerfile or tar archive read from STDIN.
//
// Usage: docker build [OPTIONS] PATH | URL | -
func (cli *DockerCli) CmdBuild(args ...string) error {
	cmd := Cli.Subcmd("build", []string{"PATH | URL | -"}, Cli.DockerCommands["build"].Description, true)
	flTags := opts.NewListOpts(validateTag)
	cmd.Var(&flTags, []string{"t", "-tag"}, "镜像名称以及可选的标签，若指定标签，格式为：'名称:标签' ")
	//Suppress the build output and print image ID on success
	suppressOutput := cmd.Bool([]string{"q", "-quiet"}, false, "抑制构建输出，打印构建成功的镜像ID")
	//Do not use cache when building the image
	noCache := cmd.Bool([]string{"-no-cache"}, false, "构建镜像过程中，请勿使用缓存")
	//Remove intermediate containers after a successful build
	rm := cmd.Bool([]string{"-rm"}, true, "构建成功后，移除所有中间容器")
	//Always remove intermediate containers
	forceRm := cmd.Bool([]string{"-force-rm"}, false, "移除所有中间容器")
	//Always attempt to pull a newer version of the image
	pull := cmd.Bool([]string{"-pull"}, false, "尝试拉取一个较新版本的镜像")
	dockerfileName := cmd.String([]string{"f", "-file"}, "", "Dockerfile的名称(默认为当前目录下的Dockerfile文件路径)")
	flMemoryString := cmd.String([]string{"m", "-memory"}, "", "内存限制")
	flMemorySwap := cmd.String([]string{"-memory-swap"}, "", "交换内存限制 等于 实际内存 ＋ 交换区内存: '-1' 代表启用不受限的交换区内存")
	flShmSize := cmd.String([]string{"-shm-size"}, "", "共享内存/dev/shm 的大小, 默认值是64MB")
	flCPUShares := cmd.Int64([]string{"#c", "-cpu-shares"}, 0, "CPU计算资源的值（相对值）")
	flCPUPeriod := cmd.Int64([]string{"-cpu-period"}, 0, "限制CPU绝对公平调度算法(CFS)的时间周期")
	flCPUQuota := cmd.Int64([]string{"-cpu-quota"}, 0, "限制CPU绝对公平调度算法(CFS)的时间限额")
	flCPUSetCpus := cmd.String([]string{"-cpuset-cpus"}, "", "允许容器执行的CPU核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flCPUSetMems := cmd.String([]string{"-cpuset-mems"}, "", "允许容器执行的CPU内存所在核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flCgroupParent := cmd.String([]string{"-cgroup-parent"}, "", "容器的可选cgroup父路径")
	flBuildArg := opts.NewListOpts(runconfigopts.ValidateEnv)
	//Set build-time variables
	cmd.Var(&flBuildArg, []string{"-build-arg"}, "设置构建镜像的时间变量")
	isolation := cmd.String([]string{"-isolation"}, "", "设置容器的隔离策略")

	ulimits := make(map[string]*units.Ulimit)
	flUlimits := runconfigopts.NewUlimitOpt(&ulimits)
	cmd.Var(flUlimits, []string{"-ulimit"}, "Ulimit 选项")

	cmd.Require(flag.Exact, 1)

	// For trusted pull on "FROM <image>" instruction.
	addTrustedFlags(cmd, true)

	cmd.ParseFlags(args, true)

	var (
		context  io.ReadCloser
		isRemote bool
		err      error
	)

	specifiedContext := cmd.Arg(0)

	var (
		contextDir    string
		tempDir       string
		relDockerfile string
		progBuff      io.Writer
		buildBuff     io.Writer
	)

	progBuff = cli.out
	buildBuff = cli.out
	if *suppressOutput {
		progBuff = bytes.NewBuffer(nil)
		buildBuff = bytes.NewBuffer(nil)
	}

	switch {
	case specifiedContext == "-":
		context, relDockerfile, err = getContextFromReader(cli.in, *dockerfileName)
	case urlutil.IsGitURL(specifiedContext):
		tempDir, relDockerfile, err = getContextFromGitURL(specifiedContext, *dockerfileName)
	case urlutil.IsURL(specifiedContext):
		context, relDockerfile, err = getContextFromURL(progBuff, specifiedContext, *dockerfileName)
	default:
		contextDir, relDockerfile, err = getContextFromLocalDir(specifiedContext, *dockerfileName)
	}

	if err != nil {
		if *suppressOutput && urlutil.IsURL(specifiedContext) {
			fmt.Fprintln(cli.err, progBuff)
		}
		return fmt.Errorf("准备构建上下文失败: %s", err)
	}

	if tempDir != "" {
		defer os.RemoveAll(tempDir)
		contextDir = tempDir
	}

	if context == nil {
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

		if err := validateContextDirectory(contextDir, excludes); err != nil {
			return fmt.Errorf("检测构建上下文出错: '%s'.", err)
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

		context, err = archive.TarWithOptions(contextDir, &archive.TarOptions{
			Compression:     archive.Uncompressed,
			ExcludePatterns: excludes,
			IncludeFiles:    includes,
		})
		if err != nil {
			return err
		}
	}

	var resolvedTags []*resolvedTag
	if isTrusted() {
		// Wrap the tar archive to replace the Dockerfile entry with the rewritten
		// Dockerfile which uses trusted pulls.
		context = replaceDockerfileTarWrapper(context, relDockerfile, cli.trustedReference, &resolvedTags)
	}

	// Setup an upload progress bar
	progressOutput := streamformatter.NewStreamFormatter().NewProgressOutput(progBuff, true)

	var body io.Reader = progress.NewProgressReader(context, progressOutput, 0, "", "向 Docker 引擎发送构建上下文")

	var memory int64
	if *flMemoryString != "" {
		parsedMemory, err := units.RAMInBytes(*flMemoryString)
		if err != nil {
			return err
		}
		memory = parsedMemory
	}

	var memorySwap int64
	if *flMemorySwap != "" {
		if *flMemorySwap == "-1" {
			memorySwap = -1
		} else {
			parsedMemorySwap, err := units.RAMInBytes(*flMemorySwap)
			if err != nil {
				return err
			}
			memorySwap = parsedMemorySwap
		}
	}

	var shmSize int64
	if *flShmSize != "" {
		shmSize, err = units.RAMInBytes(*flShmSize)
		if err != nil {
			return err
		}
	}

	var remoteContext string
	if isRemote {
		remoteContext = cmd.Arg(0)
	}

	options := types.ImageBuildOptions{
		Context:        body,
		Memory:         memory,
		MemorySwap:     memorySwap,
		Tags:           flTags.GetAll(),
		SuppressOutput: *suppressOutput,
		RemoteContext:  remoteContext,
		NoCache:        *noCache,
		Remove:         *rm,
		ForceRemove:    *forceRm,
		PullParent:     *pull,
		IsolationLevel: container.IsolationLevel(*isolation),
		CPUSetCPUs:     *flCPUSetCpus,
		CPUSetMems:     *flCPUSetMems,
		CPUShares:      *flCPUShares,
		CPUQuota:       *flCPUQuota,
		CPUPeriod:      *flCPUPeriod,
		CgroupParent:   *flCgroupParent,
		Dockerfile:     relDockerfile,
		ShmSize:        shmSize,
		Ulimits:        flUlimits.GetList(),
		BuildArgs:      runconfigopts.ConvertKVStringsToMap(flBuildArg.GetAll()),
		AuthConfigs:    cli.configFile.AuthConfigs,
	}

	response, err := cli.client.ImageBuild(options)
	if err != nil {
		return err
	}

	err = jsonmessage.DisplayJSONMessagesStream(response.Body, buildBuff, cli.outFd, cli.isTerminalOut, nil)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			if *suppressOutput {
				fmt.Fprintf(cli.err, "%s%s", progBuff, buildBuff)
			}
			return Cli.StatusError{Status: jerr.Message, StatusCode: jerr.Code}
		}
	}

	// Windows: show error message about modified file permissions if the
	// daemon isn't running Windows.
	if response.OSType != "windows" && runtime.GOOS == "windows" {
		fmt.Fprintln(cli.err, `安全警告: 您在一个非Windows的Docker引擎上构建Windows机器的Docker的镜像。所有添加到构建上下文的文件和目录将添加'-rwxr-xr-x'权限。推荐您为敏感的文件和目录进行权限的重复查验。`)
	}

	// Everything worked so if -q was provided the output from the daemon
	// should be just the image ID and we'll print that to stdout.
	if *suppressOutput {
		fmt.Fprintf(cli.out, "%s", buildBuff)
	}

	if isTrusted() {
		// Since the build was successful, now we must tag any of the resolved
		// images from the above Dockerfile rewrite.
		for _, resolved := range resolvedTags {
			if err := cli.tagTrusted(resolved.digestRef, resolved.tagRef); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateContextDirectory checks if all the contents of the directory
// can be read and returns an error if some files can't be read
// symlinks which point to non-existing files don't trigger an error
func validateContextDirectory(srcPath string, excludes []string) error {
	contextRoot, err := getContextRoot(srcPath)
	if err != nil {
		return err
	}
	return filepath.Walk(contextRoot, func(filePath string, f os.FileInfo, err error) error {
		// skip this directory/file if it's not in the path, it won't get added to the context
		if relFilePath, err := filepath.Rel(contextRoot, filePath); err != nil {
			return err
		} else if skip, err := fileutils.Matches(relFilePath, excludes); err != nil {
			return err
		} else if skip {
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("can't stat '%s'", filePath)
			}
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// skip checking if symlinks point to non-existing files, such symlinks can be useful
		// also skip named pipes, because they hanging on open
		if f.Mode()&(os.ModeSymlink|os.ModeNamedPipe) != 0 {
			return nil
		}

		if !f.IsDir() {
			currentFile, err := os.Open(filePath)
			if err != nil && os.IsPermission(err) {
				return fmt.Errorf("没有权限读取 '%s'", filePath)
			}
			currentFile.Close()
		}
		return nil
	})
}

// validateTag checks if the given image name can be resolved.
func validateTag(rawRepo string) (string, error) {
	_, err := reference.ParseNamed(rawRepo)
	if err != nil {
		return "", err
	}

	return rawRepo, nil
}

// isUNC returns true if the path is UNC (one starting \\). It always returns
// false on Linux.
func isUNC(path string) bool {
	return runtime.GOOS == "windows" && strings.HasPrefix(path, `\\`)
}

// getDockerfileRelPath uses the given context directory for a `docker build`
// and returns the absolute path to the context directory, the relative path of
// the dockerfile in that context directory, and a non-nil error on success.
func getDockerfileRelPath(givenContextDir, givenDockerfile string) (absContextDir, relDockerfile string, err error) {
	if absContextDir, err = filepath.Abs(givenContextDir); err != nil {
		return "", "", fmt.Errorf("无法获得上下文的绝对目录: %v", err)
	}

	// The context dir might be a symbolic link, so follow it to the actual
	// target directory.
	//
	// FIXME. We use isUNC (always false on non-Windows platforms) to workaround
	// an issue in golang. On Windows, EvalSymLinks does not work on UNC file
	// paths (those starting with \\). This hack means that when using links
	// on UNC paths, they will not be followed.
	if !isUNC(absContextDir) {
		absContextDir, err = filepath.EvalSymlinks(absContextDir)
		if err != nil {
			return "", "", fmt.Errorf("无法评估上下文路径的符号链接: %v", err)
		}
	}

	stat, err := os.Lstat(absContextDir)
	if err != nil {
		return "", "", fmt.Errorf("无法统计上下文目录 %q: %v", absContextDir, err)
	}

	if !stat.IsDir() {
		return "", "", fmt.Errorf("上下文必须是一个目录: %s", absContextDir)
	}

	absDockerfile := givenDockerfile
	if absDockerfile == "" {
		// No -f/--file was specified so use the default relative to the
		// context directory.
		absDockerfile = filepath.Join(absContextDir, api.DefaultDockerfileName)

		// Just to be nice ;-) look for 'dockerfile' too but only
		// use it if we found it, otherwise ignore this check
		if _, err = os.Lstat(absDockerfile); os.IsNotExist(err) {
			altPath := filepath.Join(absContextDir, strings.ToLower(api.DefaultDockerfileName))
			if _, err = os.Lstat(altPath); err == nil {
				absDockerfile = altPath
			}
		}
	}

	// If not already an absolute path, the Dockerfile path should be joined to
	// the base directory.
	if !filepath.IsAbs(absDockerfile) {
		absDockerfile = filepath.Join(absContextDir, absDockerfile)
	}

	// Evaluate symlinks in the path to the Dockerfile too.
	//
	// FIXME. We use isUNC (always false on non-Windows platforms) to workaround
	// an issue in golang. On Windows, EvalSymLinks does not work on UNC file
	// paths (those starting with \\). This hack means that when using links
	// on UNC paths, they will not be followed.
	if !isUNC(absDockerfile) {
		absDockerfile, err = filepath.EvalSymlinks(absDockerfile)
		if err != nil {
			return "", "", fmt.Errorf("无法评估Dockerfile路径的符号链接 : %v", err)
		}
	}

	if _, err := os.Lstat(absDockerfile); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("无法找到Dockerfile: %q", absDockerfile)
		}
		return "", "", fmt.Errorf("无法统计Dockerfile: %v", err)
	}

	if relDockerfile, err = filepath.Rel(absContextDir, absDockerfile); err != nil {
		return "", "", fmt.Errorf("无法获取Dockerfile的相对路径: %v", err)
	}

	if strings.HasPrefix(relDockerfile, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("Dockerfile (%s) 必须在构建上下文里面 (%s)", givenDockerfile, givenContextDir)
	}

	return absContextDir, relDockerfile, nil
}

// writeToFile copies from the given reader and writes it to a file with the
// given filename.
func writeToFile(r io.Reader, filename string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		return fmt.Errorf("无法创建文件: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("无法写入文件: %v", err)
	}

	return nil
}

// getContextFromReader will read the contents of the given reader as either a
// Dockerfile or tar archive. Returns a tar archive used as a context and a
// path to the Dockerfile inside the tar.
func getContextFromReader(r io.ReadCloser, dockerfileName string) (out io.ReadCloser, relDockerfile string, err error) {
	buf := bufio.NewReader(r)

	magic, err := buf.Peek(archive.HeaderSize)
	if err != nil && err != io.EOF {
		return nil, "", fmt.Errorf("从标准输入中获取上下文头字段失败: %v", err)
	}

	if archive.IsArchive(magic) {
		return ioutils.NewReadCloserWrapper(buf, func() error { return r.Close() }), dockerfileName, nil
	}

	// Input should be read as a Dockerfile.
	tmpDir, err := ioutil.TempDir("", "docker-build-context-")
	if err != nil {
		return nil, "", fmt.Errorf("无法创建临时的上下文目录: %v", err)
	}

	f, err := os.Create(filepath.Join(tmpDir, api.DefaultDockerfileName))
	if err != nil {
		return nil, "", err
	}
	_, err = io.Copy(f, buf)
	if err != nil {
		f.Close()
		return nil, "", err
	}

	if err := f.Close(); err != nil {
		return nil, "", err
	}
	if err := r.Close(); err != nil {
		return nil, "", err
	}

	tar, err := archive.Tar(tmpDir, archive.Uncompressed)
	if err != nil {
		return nil, "", err
	}

	return ioutils.NewReadCloserWrapper(tar, func() error {
		err := tar.Close()
		os.RemoveAll(tmpDir)
		return err
	}), api.DefaultDockerfileName, nil

}

// getContextFromGitURL uses a Git URL as context for a `docker build`. The
// git repo is cloned into a temporary directory used as the context directory.
// Returns the absolute path to the temporary context directory, the relative
// path of the dockerfile in that context directory, and a non-nil error on
// success.
func getContextFromGitURL(gitURL, dockerfileName string) (absContextDir, relDockerfile string, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", fmt.Errorf("无法找到'git': %v", err)
	}
	if absContextDir, err = gitutils.Clone(gitURL); err != nil {
		return "", "", fmt.Errorf("无法使用 'git clone' 命令获得临时的上下文目录: %v", err)
	}

	return getDockerfileRelPath(absContextDir, dockerfileName)
}

// getContextFromURL uses a remote URL as context for a `docker build`. The
// remote resource is downloaded as either a Dockerfile or a tar archive.
// Returns the tar archive used for the context and a path of the
// dockerfile inside the tar.
func getContextFromURL(out io.Writer, remoteURL, dockerfileName string) (io.ReadCloser, string, error) {
	response, err := httputils.Download(remoteURL)
	if err != nil {
		return nil, "", fmt.Errorf("无法下载远程的上下文 %s: %v", remoteURL, err)
	}
	progressOutput := streamformatter.NewStreamFormatter().NewProgressOutput(out, true)

	// Pass the response body through a progress reader.
	progReader := progress.NewProgressReader(response.Body, progressOutput, response.ContentLength, "", fmt.Sprintf("从远程url下载构建上下文: %s", remoteURL))

	return getContextFromReader(ioutils.NewReadCloserWrapper(progReader, func() error { return response.Body.Close() }), dockerfileName)
}

// getContextFromLocalDir uses the given local directory as context for a
// `docker build`. Returns the absolute path to the local context directory,
// the relative path of the dockerfile in that context directory, and a non-nil
// error on success.
func getContextFromLocalDir(localDir, dockerfileName string) (absContextDir, relDockerfile string, err error) {
	// When using a local context directory, when the Dockerfile is specified
	// with the `-f/--file` option then it is considered relative to the
	// current directory and not the context directory.
	if dockerfileName != "" {
		if dockerfileName, err = filepath.Abs(dockerfileName); err != nil {
			return "", "", fmt.Errorf("无法获得Dockerfile的绝对路径: %v", err)
		}
	}

	return getDockerfileRelPath(localDir, dockerfileName)
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
func rewriteDockerfileFrom(dockerfile io.Reader, translator translatorFunc) (newDockerfile []byte, resolvedTags []*resolvedTag, err error) {
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
			if ref, ok := ref.(reference.NamedTagged); ok && isTrusted() {
				trustedRef, err := translator(ref)
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
func replaceDockerfileTarWrapper(inputTarStream io.ReadCloser, dockerfileName string, translator translatorFunc, resolvedTags *[]*resolvedTag) io.ReadCloser {
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
				newDockerfile, *resolvedTags, err = rewriteDockerfileFrom(content, translator)
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
