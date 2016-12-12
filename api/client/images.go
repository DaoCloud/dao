package client

import (
	"github.com/docker/docker/api/client/formatter"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
)

// CmdImages lists the images in a specified repository, or all top-level images if no repository is specified.
//
// Usage: docker images [OPTIONS] [REPOSITORY]
func (cli *DockerCli) CmdImages(args ...string) error {
	cmd := Cli.Subcmd("images", []string{"[REPOSITORY[:TAG]]"}, Cli.DockerCommands["images"].Description, true)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "仅显示数字ID")
	all := cmd.Bool([]string{"a", "-all"}, false, "显示所有的镜像(默认情况隐藏中间镜像)")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "不截断命令输出内容")
	showDigests := cmd.Bool([]string{"-digests"}, false, "显示验证信息")
	format := cmd.String([]string{"-format"}, "", "使用一个Go语言模版打印镜像信息")

	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "基于指定条件过滤命令输出内容")
	cmd.Require(flag.Max, 1)

	cmd.ParseFlags(args, true)

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	imageFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		var err error
		imageFilterArgs, err = filters.ParseFlag(f, imageFilterArgs)
		if err != nil {
			return err
		}
	}

	var matchName string
	if cmd.NArg() == 1 {
		matchName = cmd.Arg(0)
	}

	options := types.ImageListOptions{
		MatchName: matchName,
		All:       *all,
		Filters:   imageFilterArgs,
	}

	images, err := cli.client.ImageList(options)
	if err != nil {
		return err
	}

	f := *format
	if len(f) == 0 {
		if len(cli.ImagesFormat()) > 0 && !*quiet {
			f = cli.ImagesFormat()
		} else {
			f = "table"
		}
	}

	imagesCtx := formatter.ImageContext{
		Context: formatter.Context{
			Output: cli.out,
			Format: f,
			Quiet:  *quiet,
			Trunc:  !*noTrunc,
		},
		Digest: *showDigests,
		Images: images,
	}

	imagesCtx.Write()

	return nil
}
