package node

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	force bool
}

func newRemoveCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := removeOptions{}

	cmd := &cobra.Command{
		Use:     "rm [OPTIONS] NODE [NODE...]",
		Aliases: []string{"remove"},
		Short:   "从Swarm集群中删除一个或多个节点",
		Args:    cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(dockerCli, args, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.force, "force", false, "强制删除一个活跃节点")
	return cmd
}

func runRemove(dockerCli *client.DockerCli, args []string, opts removeOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	for _, nodeID := range args {
		err := client.NodeRemove(ctx, nodeID, types.NodeRemoveOptions{Force: opts.force})
		if err != nil {
			return err
		}
		fmt.Fprintf(dockerCli.Out(), "%s\n", nodeID)
	}
	return nil
}
