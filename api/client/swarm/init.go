package swarm

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	generatedSecretEntropyBytes = 16
	generatedSecretBase         = 36
	// floor(log(2^128-1, 36)) + 1
	maxGeneratedSecretLength = 25
)

type initOptions struct {
	swarmOptions
	listenAddr NodeAddrOption
	// Not a NodeAddrOption because it has no default port.
	advertiseAddr   string
	forceNewCluster bool
}

func newInitCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := initOptions{
		listenAddr: NewListenAddrOption(),
	}

	cmd := &cobra.Command{
		Use:   "init [OPTIONS]",
		Short: "初始化Swarm集群",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(dockerCli, cmd.Flags(), opts)
		},
	}

	flags := cmd.Flags()
	flags.Var(&opts.listenAddr, flagListenAddr, "Swarm监听地址 （格式: <IP地址|网卡>[:端口]）")
	flags.StringVar(&opts.advertiseAddr, flagAdvertiseAddr, "", "广播地址 （格式: <IP地址|网卡>[:端口]）")
	flags.BoolVar(&opts.forceNewCluster, "force-new-cluster", false, "从节点当前状态强制创建一个集群。")
	addSwarmFlags(flags, &opts.swarmOptions)
	return cmd
}

func runInit(dockerCli *client.DockerCli, flags *pflag.FlagSet, opts initOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	req := swarm.InitRequest{
		ListenAddr:      opts.listenAddr.String(),
		AdvertiseAddr:   opts.advertiseAddr,
		ForceNewCluster: opts.forceNewCluster,
		Spec:            opts.swarmOptions.ToSpec(),
	}

	nodeID, err := client.SwarmInit(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "could not choose an IP address to advertise") || strings.Contains(err.Error(), "could not find the system's IP address") {
			return errors.New(err.Error() + " - specify one with --advertise-addr")
		}
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "成功初始化Swarm集群: 当前节点 (%s) 现在已经是管理者角色。\n\n", nodeID)

	if err := printJoinCommand(ctx, dockerCli, nodeID, true, false); err != nil {
		return err
	}

	fmt.Fprint(dockerCli.Out(), "在此Swarm集群中添加一个管理者角色, 运行 'docker swarm join-token manager' 并遵循相应的说明。\n\n")
	return nil
}
