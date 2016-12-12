package container

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/archive"
	"github.com/spf13/cobra"
)

type diffOptions struct {
	container string
}

// NewDiffCommand creates a new cobra.Command for `docker diff`
func NewDiffCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts diffOptions

	cmd := &cobra.Command{
		Use:   "diff CONTAINER",
		Short: "查看容器文件系统的变化差异",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runDiff(dockerCli, &opts)
		},
	}

	return cmd
}

func runDiff(dockerCli *client.DockerCli, opts *diffOptions) error {
	if opts.container == "" {
		return fmt.Errorf("容器名称不能为空")
	}
	ctx := context.Background()

	changes, err := dockerCli.Client().ContainerDiff(ctx, opts.container)
	if err != nil {
		return err
	}

	for _, change := range changes {
		var kind string
		switch change.Kind {
		case archive.ChangeModify:
			kind = "C"
		case archive.ChangeAdd:
			kind = "A"
		case archive.ChangeDelete:
			kind = "D"
		}
		fmt.Fprintf(dockerCli.Out(), "%s %s\n", kind, change.Path)
	}

	return nil
}
