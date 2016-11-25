package image

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/formatter"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/spf13/cobra"
)

type imagesOptions struct {
	matchName string

	quiet       bool
	all         bool
	noTrunc     bool
	showDigests bool
	format      string
	filter      []string
}

// NewImagesCommand create a new `docker images` command
func NewImagesCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts imagesOptions

	cmd := &cobra.Command{
		Use:   "images [OPTIONS] [REPOSITORY[:TAG]]",
		Short: "罗列所有镜像",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.matchName = args[0]
			}
			return runImages(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "仅显示数字ID")
	flags.BoolVarP(&opts.all, "all", "a", false, "显示所有的镜像(默认情况隐藏中间镜像)")
	flags.BoolVar(&opts.noTrunc, "no-trunc", false, "不截断命令输出内容")
	flags.BoolVar(&opts.showDigests, "digests", false, "显示验证信息")
	flags.StringVar(&opts.format, "format", "", "使用一个Go语言模板打印镜像信息")
	flags.StringSliceVarP(&opts.filter, "filter", "f", []string{}, "基于指定条件过滤命令输出内容")

	return cmd
}

func runImages(dockerCli *client.DockerCli, opts imagesOptions) error {
	ctx := context.Background()

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process in the daemon/server.
	imageFilterArgs := filters.NewArgs()
	for _, f := range opts.filter {
		var err error
		imageFilterArgs, err = filters.ParseFlag(f, imageFilterArgs)
		if err != nil {
			return err
		}
	}

	matchName := opts.matchName

	options := types.ImageListOptions{
		MatchName: matchName,
		All:       opts.all,
		Filters:   imageFilterArgs,
	}

	images, err := dockerCli.Client().ImageList(ctx, options)
	if err != nil {
		return err
	}

	f := opts.format
	if len(f) == 0 {
		if len(dockerCli.ImagesFormat()) > 0 && !opts.quiet {
			f = dockerCli.ImagesFormat()
		} else {
			f = "table"
		}
	}

	imagesCtx := formatter.ImageContext{
		Context: formatter.Context{
			Output: dockerCli.Out(),
			Format: f,
			Quiet:  opts.quiet,
			Trunc:  !opts.noTrunc,
		},
		Digest: opts.showDigests,
		Images: images,
	}

	imagesCtx.Write()

	return nil
}
