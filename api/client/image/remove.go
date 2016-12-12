package image

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	force   bool
	noPrune bool
}

// NewRemoveCommand create a new `docker remove` command
func NewRemoveCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:   "rmi [OPTIONS] IMAGE [IMAGE...]",
		Short: "删除一个或者多个镜像",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, opts, args)
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&opts.force, "force", "f", false, "强制删除镜像")
	flags.BoolVar(&opts.noPrune, "no-prune", false, "不删除没有标签的父镜像")

	return cmd
}

func runRemove(dockerCli *client.DockerCli, opts removeOptions, images []string) error {
	client := dockerCli.Client()
	ctx := context.Background()

	options := types.ImageRemoveOptions{
		Force:         opts.force,
		PruneChildren: !opts.noPrune,
	}

	var errs []string
	for _, image := range images {
		dels, err := client.ImageRemove(ctx, image, options)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			for _, del := range dels {
				if del.Deleted != "" {
					fmt.Fprintf(dockerCli.Out(), "已删除: %s\n", del.Deleted)
				} else {
					fmt.Fprintf(dockerCli.Out(), "去标签: %s\n", del.Untagged)
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
