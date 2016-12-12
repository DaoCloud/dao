package node

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type inspectOptions struct {
	nodeIds []string
	format  string
	pretty  bool
}

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] self|NODE [NODE...]",
		Short: "显示Swarm集群中一个或多个节点的详细信息",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.nodeIds = args
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "使用指定的Go语言模板格式化命令输出内容。")
	flags.BoolVar(&opts.pretty, "pretty", false, "通过人工可读的格式输出命令信息。")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()
	getRef := func(ref string) (interface{}, []byte, error) {
		nodeRef, err := Reference(client, ctx, ref)
		if err != nil {
			return nil, nil, err
		}
		node, _, err := client.NodeInspectWithRaw(ctx, nodeRef)
		return node, nil, err
	}

	if !opts.pretty {
		return inspect.Inspect(dockerCli.Out(), opts.nodeIds, opts.format, getRef)
	}
	return printHumanFriendly(dockerCli.Out(), opts.nodeIds, getRef)
}

func printHumanFriendly(out io.Writer, refs []string, getRef inspect.GetRefFunc) error {
	for idx, ref := range refs {
		obj, _, err := getRef(ref)
		if err != nil {
			return err
		}
		printNode(out, obj.(swarm.Node))

		// TODO: better way to do this?
		// print extra space between objects, but not after the last one
		if idx+1 != len(refs) {
			fmt.Fprintf(out, "\n\n")
		} else {
			fmt.Fprintf(out, "\n")
		}
	}
	return nil
}

// TODO: use a template
func printNode(out io.Writer, node swarm.Node) {
	fmt.Fprintf(out, "节点ID:\t\t\t%s\n", node.ID)
	ioutils.FprintfIfNotEmpty(out, "名称:\t\t\t%s\n", node.Spec.Name)
	if node.Spec.Labels != nil {
		fmt.Fprintln(out, "标签:")
		for k, v := range node.Spec.Labels {
			fmt.Fprintf(out, " - %s = %s\n", k, v)
		}
	}

	ioutils.FprintfIfNotEmpty(out, "主机名:\t\t%s\n", node.Description.Hostname)
	fmt.Fprintf(out, "加入集群时间:\t\t%s\n", client.PrettyPrint(node.CreatedAt))
	fmt.Fprintln(out, "状态:")
	fmt.Fprintf(out, " 状态:\t\t\t%s\n", client.PrettyPrint(node.Status.State))
	ioutils.FprintfIfNotEmpty(out, " 状态消息:\t\t%s\n", client.PrettyPrint(node.Status.Message))
	fmt.Fprintf(out, " 可达状态:\t\t%s\n", client.PrettyPrint(node.Spec.Availability))

	if node.ManagerStatus != nil {
		fmt.Fprintln(out, "管理角色状态:")
		fmt.Fprintf(out, " 监听地址:\t\t%s\n", node.ManagerStatus.Addr)
		fmt.Fprintf(out, " Raft一致性状态:\t\t%s\n", client.PrettyPrint(node.ManagerStatus.Reachability))
		leader := "No"
		if node.ManagerStatus.Leader {
			leader = "Yes"
		}
		fmt.Fprintf(out, " 领导者:\t\t%s\n", leader)
	}

	fmt.Fprintln(out, "平台信息:")
	fmt.Fprintf(out, " 操作系统:\t%s\n", node.Description.Platform.OS)
	fmt.Fprintf(out, " 机器架构:\t\t%s\n", node.Description.Platform.Architecture)

	fmt.Fprintln(out, "机器资源:")
	fmt.Fprintf(out, " CPU总数量:\t\t\t%d\n", node.Description.Resources.NanoCPUs/1e9)
	fmt.Fprintf(out, " 总内存:\t\t%s\n", units.BytesSize(float64(node.Description.Resources.MemoryBytes)))

	var pluginTypes []string
	pluginNamesByType := map[string][]string{}
	for _, p := range node.Description.Engine.Plugins {
		// append to pluginTypes only if not done previously
		if _, ok := pluginNamesByType[p.Type]; !ok {
			pluginTypes = append(pluginTypes, p.Type)
		}
		pluginNamesByType[p.Type] = append(pluginNamesByType[p.Type], p.Name)
	}

	if len(pluginTypes) > 0 {
		fmt.Fprintln(out, "插件:")
		sort.Strings(pluginTypes) // ensure stable output
		for _, pluginType := range pluginTypes {
			fmt.Fprintf(out, "  %s:\t\t%s\n", pluginType, strings.Join(pluginNamesByType[pluginType], ", "))
		}
	}
	fmt.Fprintf(out, "Docker引擎版本:\t\t%s\n", node.Description.Engine.EngineVersion)

	if len(node.Description.Engine.Labels) != 0 {
		fmt.Fprintln(out, "Docker引擎标签:")
		for k, v := range node.Description.Engine.Labels {
			fmt.Fprintf(out, " - %s = %s\n", k, v)
		}
	}
}
