package image

import (
	"errors"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type saveOptions struct {
	images []string
	output string
}

// NewSaveCommand creates a new `docker save` command
func NewSaveCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts saveOptions

	cmd := &cobra.Command{
		Use:   "save [OPTIONS] IMAGE [IMAGE...]",
		Short: "将一个或多个镜像保存至压缩包(默认情况下流传输至标准输出)",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.images = args
			return runSave(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.output, "output", "o", "", "写入一个文件，而不是标准输出")

	return cmd
}

func runSave(dockerCli *client.DockerCli, opts saveOptions) error {
	if opts.output == "" && dockerCli.IsTerminalOut() {
		return errors.New("终端拒绝保存输出内容，请您使用 -o 参数或者重定向。")
	}

	responseBody, err := dockerCli.Client().ImageSave(context.Background(), opts.images)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if opts.output == "" {
		_, err := io.Copy(dockerCli.Out(), responseBody)
		return err
	}

	return client.CopyToFile(opts.output, responseBody)
}
