package swarm

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type joinOptions struct {
	remote     string
	listenAddr NodeAddrOption
	// Not a NodeAddrOption because it has no default port.
	advertiseAddr string
	token         string
}

func newJoinCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := joinOptions{
		listenAddr: NewListenAddrOption(),
	}

	cmd := &cobra.Command{
		Use:   "join [OPTIONS] HOST:PORT",
		Short: "加入作为工作节点(worker)或者管理节点(manager)加入一个Swarm集群",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.remote = args[0]
			return runJoin(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, flagListenAddr, "监听地址 (格式: <IP地址|网卡>[:端口])")
	flags.StringVar(&opts.advertiseAddr, flagAdvertiseAddr, "", "广播地址 (格式: <IP地址|网卡>[:端口])")
	flags.StringVar(&opts.token, flagToken, "", "进入此Swarm集群所需的令牌")
	return cmd
}

func runJoin(dockerCli *client.DockerCli, opts joinOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.JoinRequest{
		JoinToken:     opts.token,
		ListenAddr:    opts.listenAddr.String(),
		AdvertiseAddr: opts.advertiseAddr,
		RemoteAddrs:   []string{opts.remote},
	}
	err := client.SwarmJoin(ctx, req)
	if err != nil {
		return err
	}

	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	_, _, err = client.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	if err != nil {
		// TODO(aaronl): is there a better way to do this?
		if strings.Contains(err.Error(), "This node is not a swarm manager.") {
			fmt.Fprintln(dockerCli.Out(), "此节点已经作为一个工作节点(worker)加入指定的Swarm集群。")
		}
	} else {
		fmt.Fprintln(dockerCli.Out(), "此节点已经作为一个管理节点(manager)加入指定的Swarm集群。")
	}

	return nil
}
