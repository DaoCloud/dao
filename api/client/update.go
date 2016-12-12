package client

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types/container"
	"github.com/docker/go-units"
)

// CmdUpdate updates resources of one or more containers.
//
// Usage: docker update [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdUpdate(args ...string) error {
	cmd := Cli.Subcmd("update", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["update"].Description, true)
	flBlkioWeight := cmd.Uint16([]string{"-blkio-weight"}, 0, "磁盘IO设置(相对值),从10到1000")
	flCPUPeriod := cmd.Int64([]string{"-cpu-period"}, 0, "限制CPU绝对公平调度算法（CFS）的时间周期")
	flCPUQuota := cmd.Int64([]string{"-cpu-quota"}, 0, "限制CPU绝对公平调度算法（CFS）的时间限额")
	flCpusetCpus := cmd.String([]string{"-cpuset-cpus"}, "", "允许容器执行的CPU核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flCpusetMems := cmd.String([]string{"-cpuset-mems"}, "", "允许容器执行的CPU内存所在核指定(0-3,0,1): 0-3代表运行运行在0,1,2,3这4个核上")
	flCPUShares := cmd.Int64([]string{"c", "-cpu-shares"}, 0, "CPU计算资源的值(相对值)")
	flMemoryString := cmd.String([]string{"m", "-memory"}, "", "内存限制")
	flMemoryReservation := cmd.String([]string{"-memory-reservation"}, "", "内存软限制")
	flMemorySwap := cmd.String([]string{"-memory-swap"}, "", "交换内存限制 等于 实际内存 ＋ 交换区内存: '-1' 代表启用不受限的交换区内存")
	flKernelMemory := cmd.String([]string{"-kernel-memory"}, "", "内核内存限制")
	flRestartPolicy := cmd.String([]string{"-restart"}, "", "当容器退出时应用在容器上的重启策略")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)
	if cmd.NFlag() == 0 {
		return fmt.Errorf("当使用此命令时，您必须提供一个或多个命令参数。")
	}

	var err error
	var flMemory int64
	if *flMemoryString != "" {
		flMemory, err = units.RAMInBytes(*flMemoryString)
		if err != nil {
			return err
		}
	}

	var memoryReservation int64
	if *flMemoryReservation != "" {
		memoryReservation, err = units.RAMInBytes(*flMemoryReservation)
		if err != nil {
			return err
		}
	}

	var memorySwap int64
	if *flMemorySwap != "" {
		if *flMemorySwap == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(*flMemorySwap)
			if err != nil {
				return err
			}
		}
	}

	var kernelMemory int64
	if *flKernelMemory != "" {
		kernelMemory, err = units.RAMInBytes(*flKernelMemory)
		if err != nil {
			return err
		}
	}

	var restartPolicy container.RestartPolicy
	if *flRestartPolicy != "" {
		restartPolicy, err = opts.ParseRestartPolicy(*flRestartPolicy)
		if err != nil {
			return err
		}
	}

	resources := container.Resources{
		BlkioWeight:       *flBlkioWeight,
		CpusetCpus:        *flCpusetCpus,
		CpusetMems:        *flCpusetMems,
		CPUShares:         *flCPUShares,
		Memory:            flMemory,
		MemoryReservation: memoryReservation,
		MemorySwap:        memorySwap,
		KernelMemory:      kernelMemory,
		CPUPeriod:         *flCPUPeriod,
		CPUQuota:          *flCPUQuota,
	}

	updateConfig := container.UpdateConfig{
		Resources:     resources,
		RestartPolicy: restartPolicy,
	}

	ctx := context.Background()

	names := cmd.Args()
	var errs []string

	for _, name := range names {
		if err := cli.client.ContainerUpdate(ctx, name, updateConfig); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil
}
