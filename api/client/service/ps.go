package service

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/idresolver"
	"github.com/docker/docker/api/client/node"
	"github.com/docker/docker/api/client/task"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type psOptions struct {
	serviceID string
	noResolve bool
	filter    opts.FilterOpt
}

func newPSCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] SERVICE",
		Short: "罗列一个服务内的所有任务",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.serviceID = args[0]
			return runPS(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "不将ID解析成名称")
	flags.VarP(&opts.filter, "filter", "f", "基于指定条件过滤命令输出内容。")

	return cmd
}

func runPS(dockerCli *client.DockerCli, opts psOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	service, _, err := client.ServiceInspectWithRaw(ctx, opts.serviceID)
	if err != nil {
		return err
	}

	filter := opts.filter.Value()
	filter.Add("service", service.ID)
	if filter.Include("node") {
		nodeFilters := filter.Get("node")
		for _, nodeFilter := range nodeFilters {
			nodeReference, err := node.Reference(client, ctx, nodeFilter)
			if err != nil {
				return err
			}
			filter.Del("node", nodeFilter)
			filter.Add("node", nodeReference)
		}
	}

	tasks, err := client.TaskList(ctx, types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve))
}
