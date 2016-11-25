package service

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

func newScaleCommand(dockerCli *client.DockerCli) *cobra.Command {
	return &cobra.Command{
		Use:   "scale SERVICE=REPLICAS [SERVICE=REPLICAS...]",
		Short: "扩展一个或多个服务",
		Args:  scaleArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScale(dockerCli, args)
		},
	}
}

func scaleArgs(cmd *cobra.Command, args []string) error {
	if err := cli.RequiresMinArgs(1)(cmd, args); err != nil {
		return err
	}
	for _, arg := range args {
		if parts := strings.SplitN(arg, "=", 2); len(parts) != 2 {
			return fmt.Errorf(
				"无效的扩展指定项 '%s'.\n查看 '%s --help'.\n\n用途:  %s\n\n%s",
				arg,
				cmd.CommandPath(),
				cmd.UseLine(),
				cmd.Short,
			)
		}
	}
	return nil
}

func runScale(dockerCli *client.DockerCli, args []string) error {
	var errors []string
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		serviceID, scale := parts[0], parts[1]
		if err := runServiceScale(dockerCli, serviceID, scale); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", serviceID, err.Error()))
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf(strings.Join(errors, "\n"))
}

func runServiceScale(dockerCli *client.DockerCli, serviceID string, scale string) error {
	client := dockerCli.Client()
	ctx := context.Background()

	service, _, err := client.ServiceInspectWithRaw(ctx, serviceID)

	if err != nil {
		return err
	}

	serviceMode := &service.Spec.Mode
	if serviceMode.Replicated == nil {
		return fmt.Errorf("扩展只能支持副本(replicated)模式的服务")
	}
	uintScale, err := strconv.ParseUint(scale, 10, 64)
	if err != nil {
		return fmt.Errorf("无效的副本数量值 %s: %s", scale, err.Error())
	}
	serviceMode.Replicated.Replicas = &uintScale

	err = client.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s 已经扩展至 %s\n", serviceID, scale)

	return nil
}
