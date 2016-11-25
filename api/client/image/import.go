package image

import (
	"io"
	"os"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type importOptions struct {
	source    string
	reference string
	changes   []string
	message   string
}

// NewImportCommand creates a new `docker import` command
func NewImportCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts importOptions

	cmd := &cobra.Command{
		Use:   "import [OPTIONS] file|URL|- [REPOSITORY[:TAG]]",
		Short: "从一个压缩包中导入内容，从而创建一个文件系统镜像",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.source = args[0]
			if len(args) > 1 {
				opts.reference = args[1]
			}
			return runImport(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringSliceVarP(&opts.changes, "change", "c", []string{}, "应用Dockerfile中的指令到新创建出的镜像")
	flags.StringVarP(&opts.message, "message", "m", "", "为导入的镜像设置提交信息")

	return cmd
}

func runImport(dockerCli *client.DockerCli, opts importOptions) error {
	var (
		in      io.Reader
		srcName = opts.source
	)

	if opts.source == "-" {
		in = dockerCli.In()
	} else if !urlutil.IsURL(opts.source) {
		srcName = "-"
		file, err := os.Open(opts.source)
		if err != nil {
			return err
		}
		defer file.Close()
		in = file
	}

	source := types.ImageImportSource{
		Source:     in,
		SourceName: srcName,
	}

	options := types.ImageImportOptions{
		Message: opts.message,
		Changes: opts.changes,
	}

	clnt := dockerCli.Client()

	responseBody, err := clnt.ImageImport(context.Background(), source, opts.reference, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	return jsonmessage.DisplayJSONMessagesStream(responseBody, dockerCli.Out(), dockerCli.OutFd(), dockerCli.IsTerminalOut(), nil)
}
