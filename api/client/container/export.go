package container

import (
	"errors"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type exportOptions struct {
	container string
	output    string
}

// NewExportCommand creates a new `docker export` command
func NewExportCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts exportOptions

	cmd := &cobra.Command{
		Use:   "export [OPTIONS] CONTAINER",
		Short: "以一个压缩包的形式导出一个容器的文件系统",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runExport(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.output, "output", "o", "", "写入一个本地文件，而不是标准输出STDOUT")

	return cmd
}

func runExport(dockerCli *client.DockerCli, opts exportOptions) error {
	if opts.output == "" && dockerCli.IsTerminalOut() {
		return errors.New("终端拒绝保存导出内容，请使用 -o 参数或者重定向。")
	}

	clnt := dockerCli.Client()

	responseBody, err := clnt.ContainerExport(context.Background(), opts.container)
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
