package client

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/ioutils"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
	"github.com/docker/engine-api/types/swarm"
	"github.com/docker/go-units"
)

// CmdInfo displays system-wide information.
//
// Usage: docker info
func (cli *DockerCli) CmdInfo(args ...string) error {
	cmd := Cli.Subcmd("info", nil, Cli.DockerCommands["info"].Description, true)
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	ctx := context.Background()
	info, err := cli.client.Info(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "容器数: %d\n", info.Containers)
	fmt.Fprintf(cli.out, " 运行: %d\n", info.ContainersRunning)
	fmt.Fprintf(cli.out, " 暂停: %d\n", info.ContainersPaused)
	fmt.Fprintf(cli.out, " 停止: %d\n", info.ContainersStopped)
	fmt.Fprintf(cli.out, "镜像数: %d\n", info.Images)
	ioutils.FprintfIfNotEmpty(cli.out, "Docker引擎版本: %s\n", info.ServerVersion)
	ioutils.FprintfIfNotEmpty(cli.out, "存储驱动: %s\n", info.Driver)
	if info.DriverStatus != nil {
		for _, pair := range info.DriverStatus {
			fmt.Fprintf(cli.out, " %s: %s\n", pair[0], pair[1])

			// print a warning if devicemapper is using a loopback file
			if pair[0] == "Data loop file" {
				fmt.Fprintln(cli.err, " 警告: 环回设备loopback严重不建议在生产环境中使用。详见 `--storage-opt dm.thinpooldev` 来指定一个自定义的块存储设备。")
			}
		}

	}
	if info.SystemStatus != nil {
		for _, pair := range info.SystemStatus {
			fmt.Fprintf(cli.out, "%s: %s\n", pair[0], pair[1])
		}
	}
	ioutils.FprintfIfNotEmpty(cli.out, "日志驱动: %s\n", info.LoggingDriver)
	ioutils.FprintfIfNotEmpty(cli.out, "Cgroup 驱动: %s\n", info.CgroupDriver)

	fmt.Fprintf(cli.out, "插件:\n")
	fmt.Fprintf(cli.out, " 存储卷:")
	fmt.Fprintf(cli.out, " %s", strings.Join(info.Plugins.Volume, " "))
	fmt.Fprintf(cli.out, "\n")
	fmt.Fprintf(cli.out, " 网络:")
	fmt.Fprintf(cli.out, " %s", strings.Join(info.Plugins.Network, " "))
	fmt.Fprintf(cli.out, "\n")

	if len(info.Plugins.Authorization) != 0 {
		fmt.Fprintf(cli.out, " 认证:")
		fmt.Fprintf(cli.out, " %s", strings.Join(info.Plugins.Authorization, " "))
		fmt.Fprintf(cli.out, "\n")
	}

	fmt.Fprintf(cli.out, "Swarm集群: %v\n", info.Swarm.LocalNodeState)
	if info.Swarm.LocalNodeState != swarm.LocalNodeStateInactive {
		fmt.Fprintf(cli.out, " 节点ID: %s\n", info.Swarm.NodeID)
		if info.Swarm.Error != "" {
			fmt.Fprintf(cli.out, " 错误: %v\n", info.Swarm.Error)
		}
		fmt.Fprintf(cli.out, " 是否是管理者: %v\n", info.Swarm.ControlAvailable)
		if info.Swarm.ControlAvailable {
			fmt.Fprintf(cli.out, " 集群ID: %s\n", info.Swarm.Cluster.ID)
			fmt.Fprintf(cli.out, " 管理者数量: %d\n", info.Swarm.Managers)
			fmt.Fprintf(cli.out, " 节点数: %d\n", info.Swarm.Nodes)
			fmt.Fprintf(cli.out, " 编排:\n")
			fmt.Fprintf(cli.out, "  历史任务保留上线: %d\n", info.Swarm.Cluster.Spec.Orchestration.TaskHistoryRetentionLimit)
			fmt.Fprintf(cli.out, " Raft:\n")
			fmt.Fprintf(cli.out, "  快照间隔: %d\n", info.Swarm.Cluster.Spec.Raft.SnapshotInterval)
			fmt.Fprintf(cli.out, "  心跳时钟: %d\n", info.Swarm.Cluster.Spec.Raft.HeartbeatTick)
			fmt.Fprintf(cli.out, "  选举时钟: %d\n", info.Swarm.Cluster.Spec.Raft.ElectionTick)
			fmt.Fprintf(cli.out, " 分发器:\n")
			fmt.Fprintf(cli.out, "  心跳周期: %s\n", units.HumanDuration(time.Duration(info.Swarm.Cluster.Spec.Dispatcher.HeartbeatPeriod)))
			fmt.Fprintf(cli.out, " CA配置:\n")
			fmt.Fprintf(cli.out, "  过期周期: %s\n", units.HumanDuration(info.Swarm.Cluster.Spec.CAConfig.NodeCertExpiry))
			if len(info.Swarm.Cluster.Spec.CAConfig.ExternalCAs) > 0 {
				fmt.Fprintf(cli.out, "  外部CAs:\n")
				for _, entry := range info.Swarm.Cluster.Spec.CAConfig.ExternalCAs {
					fmt.Fprintf(cli.out, "    %s: %s\n", entry.Protocol, entry.URL)
				}
			}
		}
		fmt.Fprintf(cli.out, " 节点地址: %s\n", info.Swarm.NodeAddr)
	}

	if len(info.Runtimes) > 0 {
		fmt.Fprintf(cli.out, "运行时:")
		for name := range info.Runtimes {
			fmt.Fprintf(cli.out, " %s", name)
		}
		fmt.Fprint(cli.out, "\n")
		fmt.Fprintf(cli.out, "默认运行时: %s\n", info.DefaultRuntime)
	}

	fmt.Fprintf(cli.out, "安全选项:")
	ioutils.FprintfIfNotEmpty(cli.out, " %s", strings.Join(info.SecurityOptions, " "))
	fmt.Fprintf(cli.out, "\n")

	ioutils.FprintfIfNotEmpty(cli.out, "内核版本: %s\n", info.KernelVersion)
	ioutils.FprintfIfNotEmpty(cli.out, "操作系统: %s\n", info.OperatingSystem)
	ioutils.FprintfIfNotEmpty(cli.out, "操作系统类型: %s\n", info.OSType)
	ioutils.FprintfIfNotEmpty(cli.out, "机器架构: %s\n", info.Architecture)
	fmt.Fprintf(cli.out, "CPU数量: %d\n", info.NCPU)
	fmt.Fprintf(cli.out, "内存总数: %s\n", units.BytesSize(float64(info.MemTotal)))
	ioutils.FprintfIfNotEmpty(cli.out, "名称: %s\n", info.Name)
	ioutils.FprintfIfNotEmpty(cli.out, "ID: %s\n", info.ID)
	fmt.Fprintf(cli.out, "Docker引擎根目录: %s\n", info.DockerRootDir)
	fmt.Fprintf(cli.out, "调试模式(客户端): %v\n", utils.IsDebugEnabled())
	fmt.Fprintf(cli.out, "调试模式(服务端): %v\n", info.Debug)

	if info.Debug {
		fmt.Fprintf(cli.out, " 文件描述符个数: %d\n", info.NFd)
		fmt.Fprintf(cli.out, " Go协程综述: %d\n", info.NGoroutines)
		fmt.Fprintf(cli.out, " 系统时间: %s\n", info.SystemTime)
		fmt.Fprintf(cli.out, " 事件监听者总数: %d\n", info.NEventsListener)
	}

	ioutils.FprintfIfNotEmpty(cli.out, "Http代理: %s\n", info.HTTPProxy)
	ioutils.FprintfIfNotEmpty(cli.out, "Https代理: %s\n", info.HTTPSProxy)
	ioutils.FprintfIfNotEmpty(cli.out, "No Proxy: %s\n", info.NoProxy)

	if info.IndexServerAddress != "" {
		u := cli.configFile.AuthConfigs[info.IndexServerAddress].Username
		if len(u) > 0 {
			fmt.Fprintf(cli.out, "用户名: %v\n", u)
		}
		fmt.Fprintf(cli.out, "镜像仓库: %v\n", info.IndexServerAddress)
	}

	// Only output these warnings if the server does not support these features
	if info.OSType != "windows" {
		if !info.MemoryLimit {
			fmt.Fprintln(cli.err, "警告: 不支持内存限制")
		}
		if !info.SwapLimit {
			fmt.Fprintln(cli.err, "警告: 不支持交换区内存限制")
		}
		if !info.KernelMemory {
			fmt.Fprintln(cli.err, "警告: 不支持内核内存限制")
		}
		if !info.OomKillDisable {
			fmt.Fprintln(cli.err, "警告: 不支持oom kill 禁用")
		}
		if !info.CPUCfsQuota {
			fmt.Fprintln(cli.err, "警告: 不支持 cpu cfs 限额 ")
		}
		if !info.CPUCfsPeriod {
			fmt.Fprintln(cli.err, "警告: 不支持 cpu cfs 周期 ")
		}
		if !info.CPUShares {
			fmt.Fprintln(cli.err, "警告: 不支持 cpu 时间")
		}
		if !info.CPUSet {
			fmt.Fprintln(cli.err, "警告: 不支持 cpuset")
		}
		if !info.IPv4Forwarding {
			fmt.Fprintln(cli.err, "警告: IPv4转发功能已禁用")
		}
		if !info.BridgeNfIptables {
			fmt.Fprintln(cli.err, "警告: bridge-nf-call-iptables已禁用")
		}
		if !info.BridgeNfIP6tables {
			fmt.Fprintln(cli.err, "警告: bridge-nf-call-ip6tables已禁用")
		}
	}

	if info.Labels != nil {
		fmt.Fprintln(cli.out, "标签:")
		for _, attribute := range info.Labels {
			fmt.Fprintf(cli.out, " %s\n", attribute)
		}
	}

	ioutils.FprintfIfTrue(cli.out, "试验版: %v\n", info.ExperimentalBuild)
	if info.ClusterStore != "" {
		fmt.Fprintf(cli.out, "集群存储: %s\n", info.ClusterStore)
	}

	if info.ClusterAdvertise != "" {
		fmt.Fprintf(cli.out, "集群广播地址: %s\n", info.ClusterAdvertise)
	}

	if info.RegistryConfig != nil && (len(info.RegistryConfig.InsecureRegistryCIDRs) > 0 || len(info.RegistryConfig.IndexConfigs) > 0) {
		fmt.Fprintln(cli.out, "不受信的镜像仓库:")
		for _, registry := range info.RegistryConfig.IndexConfigs {
			if registry.Secure == false {
				fmt.Fprintf(cli.out, " %s\n", registry.Name)
			}
		}

		for _, registry := range info.RegistryConfig.InsecureRegistryCIDRs {
			mask, _ := registry.Mask.Size()
			fmt.Fprintf(cli.out, " %s/%d\n", registry.IP.String(), mask)
		}
	}
	return nil
}
