package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type renameOptions struct {
	oldName string
	newName string
}

// NewRenameCommand creats a new cobra.Command for `docker rename`
func NewRenameCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts renameOptions

	cmd := &cobra.Command{
		Use:   "rename CONTAINER NEW_NAME",
		Short: "重命名一个容器",
		Args:  cli.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.oldName = args[0]
			opts.newName = args[1]
			return runRename(dockerCli, &opts)
		},
	}

	return cmd
}

func runRename(dockerCli *client.DockerCli, opts *renameOptions) error {
	ctx := context.Background()

	oldName := strings.TrimSpace(opts.oldName)
	newName := strings.TrimSpace(opts.newName)

	if oldName == "" || newName == "" {
		return fmt.Errorf("错误: 新名称和旧名称均不能为空")
	}

	if err := dockerCli.Client().ContainerRename(ctx, oldName, newName); err != nil {
		fmt.Fprintf(dockerCli.Err(), "%s\n", err)
		return fmt.Errorf("错误: 重命名容器名称到 %s 失败", oldName)
	}
	return nil
}
