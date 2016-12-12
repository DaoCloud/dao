package swarm

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type leaveOptions struct {
	force bool
}

func newLeaveCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := leaveOptions{}

	cmd := &cobra.Command{
		Use:   "leave [OPTIONS]",
		Short: "脱离Swarm集群",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLeave(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(&opts.force, "force", false, "强制脱离Swarm集群，忽略所有警告。")

	return cmd
}

func runLeave(dockerCli *client.DockerCli, opts leaveOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	if err := client.SwarmLeave(ctx, opts.force); err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), "节点成功脱离Swarm集群")
	return nil
}
