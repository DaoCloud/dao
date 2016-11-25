package service

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func newCreateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := newServiceOptions()

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "创建一个新的服务",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.image = args[0]
			if len(args) > 1 {
				opts.args = args[1:]
			}
			return runCreate(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.mode, flagMode, "replicated", "服务类型: 副本(replicated)/全局(global)")
	addServiceFlags(cmd, opts)

	flags.VarP(&opts.labels, flagLabel, "l", "服务自身标签")
	flags.Var(&opts.containerLabels, flagContainerLabel, "服务中容器的标签")
	flags.VarP(&opts.env, flagEnv, "e", "设置服务环境变量")
	flags.Var(&opts.mounts, flagMount, "为服务添加一个挂载项")
	flags.StringSliceVar(&opts.constraints, flagConstraint, []string{}, "服务节点安放的限制条件")
	flags.StringSliceVar(&opts.networks, flagNetwork, []string{}, "网络附加信息")
	flags.VarP(&opts.endpoint.ports, flagPublish, "p", "将服务的一个端口暴露为一个节点端口")

	flags.SetInterspersed(false)
	return cmd
}

func runCreate(dockerCli *client.DockerCli, opts *serviceOptions) error {
	apiClient := dockerCli.Client()
	createOpts := types.ServiceCreateOptions{}

	service, err := opts.ToService()
	if err != nil {
		return err
	}

	ctx := context.Background()

	// only send auth if flag was set
	if opts.registryAuth {
		// Retrieve encoded auth token from the image reference
		encodedAuth, err := dockerCli.RetrieveAuthTokenFromImage(ctx, opts.image)
		if err != nil {
			return err
		}
		createOpts.EncodedRegistryAuth = encodedAuth
	}

	response, err := apiClient.ServiceCreate(ctx, service, createOpts)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", response.ID)
	return nil
}
