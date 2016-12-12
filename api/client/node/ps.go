package node

import (
	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/idresolver"
	"github.com/docker/docker/api/client/task"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type psOptions struct {
	nodeID    string
	noResolve bool
	filter    opts.FilterOpt
}

func newPSCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := psOptions{filter: opts.NewFilterOpt()}

	cmd := &cobra.Command{
		Use:   "ps [OPTIONS] self|NODE",
		Short: "罗列一个节点上的运行任务，默认指定指定当前节点",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.nodeID = args[0]
			return runPS(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.noResolve, "no-resolve", false, "不将ID解析成名称")
	flags.VarP(&opts.filter, "filter", "f", "通过指定条件过滤命令输出内容")

	return cmd
}

func runPS(dockerCli *client.DockerCli, opts psOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	nodeRef, err := Reference(client, ctx, opts.nodeID)
	if err != nil {
		return nil
	}
	node, _, err := client.NodeInspectWithRaw(ctx, nodeRef)
	if err != nil {
		return err
	}

	filter := opts.filter.Value()
	filter.Add("node", node.ID)
	tasks, err := client.TaskList(
		ctx,
		types.TaskListOptions{Filter: filter})
	if err != nil {
		return err
	}

	return task.Print(dockerCli, ctx, tasks, idresolver.New(client, opts.noResolve))
}
