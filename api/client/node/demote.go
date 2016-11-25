package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
)

func newDemoteCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "demote NODE [NODE...]",
		Short: "将Swarm集群中的一个或多个节点从管理者降级为工作者",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDemote(dockerCli, args)
		},
	}
}

func runDemote(dockerCli *client.DockerCli, nodes []string) error {
	demote := func(node *swarm.Node) error {
		node.Spec.Role = swarm.NodeRoleWorker
		return nil
	}
	success := func(nodeID string) {
		fmt.Fprintf(dockerCli.Out(), "在Swarm集群中成功将节点 %s 降级为工作者.\n", nodeID)
	}
	return updateNodes(dockerCli, nodes, demote, success)
}
