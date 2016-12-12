package service

import (
	"fmt"
	"io"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/api/client/inspect"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	apiclient "github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
	"github.com/spf13/cobra"
)

type inspectOptions struct {
	refs   []string
	format string
	pretty bool
}

func newInspectCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts inspectOptions

	cmd := &cobra.Command{
		Use:   "inspect [OPTIONS] SERVICE [SERVICE...]",
		Short: "显示一个或多个服务的详细信息",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.refs = args

			if opts.pretty && len(opts.format) > 0 {
				return fmt.Errorf("--format 参数和人工可读格式不兼容")
			}
			return runInspect(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.format, "format", "f", "", "通过指定的Go语言模板格式化命令输出内容。")
	flags.BoolVar(&opts.pretty, "pretty", false, "通过人工可读的格式输出命令信息。")
	return cmd
}

func runInspect(dockerCli *client.DockerCli, opts inspectOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	getRef := func(ref string) (interface{}, []byte, error) {
		service, _, err := client.ServiceInspectWithRaw(ctx, ref)
		if err == nil || !apiclient.IsErrServiceNotFound(err) {
			return service, nil, err
		}
		return nil, nil, fmt.Errorf("错误: 没有该服务 %s", ref)
	}

	if !opts.pretty {
		return inspect.Inspect(dockerCli.Out(), opts.refs, opts.format, getRef)
	}

	return printHumanFriendly(dockerCli.Out(), opts.refs, getRef)
}

func printHumanFriendly(out io.Writer, refs []string, getRef inspect.GetRefFunc) error {
	for idx, ref := range refs {
		obj, _, err := getRef(ref)
		if err != nil {
			return err
		}
		printService(out, obj.(swarm.Service))

		// TODO: better way to do this?
		// print extra space between objects, but not after the last one
		if idx+1 != len(refs) {
			fmt.Fprintf(out, "\n\n")
		}
	}
	return nil
}

// TODO: use a template
func printService(out io.Writer, service swarm.Service) {
	fmt.Fprintf(out, "ID:\t\t%s\n", service.ID)
	fmt.Fprintf(out, "名称:\t\t%s\n", service.Spec.Name)
	if service.Spec.Labels != nil {
		fmt.Fprintln(out, "标签:")
		for k, v := range service.Spec.Labels {
			fmt.Fprintf(out, " - %s=%s\n", k, v)
		}
	}

	if service.Spec.Mode.Global != nil {
		fmt.Fprintln(out, "模式:\t\t全局")
	} else {
		fmt.Fprintln(out, "模式:\t\t副本")
		if service.Spec.Mode.Replicated.Replicas != nil {
			fmt.Fprintf(out, " 副本个数:\t%d\n", *service.Spec.Mode.Replicated.Replicas)
		}
	}

	if service.UpdateStatus.State != "" {
		fmt.Fprintln(out, "更新状态:")
		fmt.Fprintf(out, " 状态:\t\t%s\n", service.UpdateStatus.State)
		fmt.Fprintf(out, " 启动时间:\t%s 之前\n", strings.ToLower(units.HumanDuration(time.Since(service.UpdateStatus.StartedAt))))
		if service.UpdateStatus.State == swarm.UpdateStateCompleted {
			fmt.Fprintf(out, " 完成时间:\t%s 之前\n", strings.ToLower(units.HumanDuration(time.Since(service.UpdateStatus.CompletedAt))))
		}
		fmt.Fprintf(out, " 更新消息:\t%s\n", service.UpdateStatus.Message)
	}

	fmt.Fprintln(out, "放置策略:")
	if service.Spec.TaskTemplate.Placement != nil && len(service.Spec.TaskTemplate.Placement.Constraints) > 0 {
		ioutils.FprintfIfNotEmpty(out, " 限制\t: %s\n", strings.Join(service.Spec.TaskTemplate.Placement.Constraints, ", "))
	}
	if service.Spec.UpdateConfig != nil {
		fmt.Fprintf(out, "更新配置:\n")
		fmt.Fprintf(out, " 并行数:\t%d\n", service.Spec.UpdateConfig.Parallelism)
		if service.Spec.UpdateConfig.Delay.Nanoseconds() > 0 {
			fmt.Fprintf(out, " 更新延迟:\t\t%s\n", service.Spec.UpdateConfig.Delay)
		}
		fmt.Fprintf(out, " 出错重启策略:\t%s\n", service.Spec.UpdateConfig.FailureAction)
	}

	fmt.Fprintf(out, "容器配置:\n")
	printContainerSpec(out, service.Spec.TaskTemplate.ContainerSpec)

	resources := service.Spec.TaskTemplate.Resources
	if resources != nil {
		fmt.Fprintln(out, "资源:")
		printResources := func(out io.Writer, requirement string, r *swarm.Resources) {
			if r == nil || (r.MemoryBytes == 0 && r.NanoCPUs == 0) {
				return
			}
			fmt.Fprintf(out, " %s:\n", requirement)
			if r.NanoCPUs != 0 {
				fmt.Fprintf(out, "  CPU资源:\t\t%g\n", float64(r.NanoCPUs)/1e9)
			}
			if r.MemoryBytes != 0 {
				fmt.Fprintf(out, "  内存资源:\t%s\n", units.BytesSize(float64(r.MemoryBytes)))
			}
		}
		printResources(out, "资源预留", resources.Reservations)
		printResources(out, "资源限制", resources.Limits)
	}
	if len(service.Spec.Networks) > 0 {
		fmt.Fprintf(out, "网络:")
		for _, n := range service.Spec.Networks {
			fmt.Fprintf(out, " %s", n.Target)
		}
		fmt.Fprintln(out, "")
	}

	if len(service.Endpoint.Ports) > 0 {
		fmt.Fprintln(out, "端口:")
		for _, port := range service.Endpoint.Ports {
			ioutils.FprintfIfNotEmpty(out, " 名称 = %s\n", port.Name)
			fmt.Fprintf(out, " 协议 = %s\n", port.Protocol)
			fmt.Fprintf(out, " 目标端口 = %d\n", port.TargetPort)
			fmt.Fprintf(out, " 暴露端口 = %d\n", port.PublishedPort)
		}
	}
}

func printContainerSpec(out io.Writer, containerSpec swarm.ContainerSpec) {
	fmt.Fprintf(out, " 镜像:\t\t%s\n", containerSpec.Image)
	if len(containerSpec.Args) > 0 {
		fmt.Fprintf(out, " 参数:\t\t%s\n", strings.Join(containerSpec.Args, " "))
	}
	if len(containerSpec.Env) > 0 {
		fmt.Fprintf(out, " 环境变量:\t\t%s\n", strings.Join(containerSpec.Env, " "))
	}
	ioutils.FprintfIfNotEmpty(out, " 目录\t\t%s\n", containerSpec.Dir)
	ioutils.FprintfIfNotEmpty(out, " 用户\t\t%s\n", containerSpec.User)
	if len(containerSpec.Mounts) > 0 {
		fmt.Fprintln(out, " 挂载:")
		for _, v := range containerSpec.Mounts {
			fmt.Fprintf(out, "  目标地址 = %s\n", v.Target)
			fmt.Fprintf(out, "  源地址 = %s\n", v.Source)
			fmt.Fprintf(out, "  只读 = %v\n", v.ReadOnly)
			fmt.Fprintf(out, "  挂载类型 = %v\n", v.Type)
		}
	}
}
