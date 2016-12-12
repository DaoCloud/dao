package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type rmOptions struct {
	rmVolumes bool
	rmLink    bool
	force     bool

	containers []string
}

// NewRmCommand creates a new cobra.Command for `docker rm`
func NewRmCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts rmOptions

	cmd := &cobra.Command{
		Use:   "rm [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "删除一个或多个容器",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runRm(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.rmVolumes, "volumes", "v", false, "删除容器时同时删除容器相关的存储卷")
	flags.BoolVarP(&opts.rmLink, "link", "l", false, "删除指定的链接")
	flags.BoolVarP(&opts.force, "force", "f", false, "强制删除一个运行的容器(使用信号SIGKILL)")
	return cmd
}

func runRm(dockerCli *client.DockerCli, opts *rmOptions) error {
	ctx := context.Background()

	var errs []string
	for _, name := range opts.containers {
		if name == "" {
			return fmt.Errorf("容器名不能为空")
		}
		name = strings.Trim(name, "/")

		if err := removeContainer(dockerCli, ctx, name, opts.rmVolumes, opts.rmLink, opts.force); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func removeContainer(dockerCli *client.DockerCli, ctx context.Context, container string, removeVolumes, removeLinks, force bool) error {
	options := types.ContainerRemoveOptions{
		RemoveVolumes: removeVolumes,
		RemoveLinks:   removeLinks,
		Force:         force,
	}
	if err := dockerCli.Client().ContainerRemove(ctx, container, options); err != nil {
		return err
	}
	return nil
}
