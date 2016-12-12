package client

import (
	"github.com/docker/docker/api/client/formatter"
	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
)

// CmdPs outputs a list of Docker containers.
//
// Usage: docker ps [OPTIONS]
func (cli *DockerCli) CmdPs(args ...string) error {
	var (
		err error

		psFilterArgs = filters.NewArgs()

		cmd      = Cli.Subcmd("ps", nil, Cli.DockerCommands["ps"].Description, true)
		quiet    = cmd.Bool([]string{"q", "-quiet"}, false, "仅显示容器ID")
		size     = cmd.Bool([]string{"s", "-size"}, false, "显示所有的文件大小")
		all      = cmd.Bool([]string{"a", "-all"}, false, "显示所有的容器(默认仅显示运行的容器)")
		noTrunc  = cmd.Bool([]string{"-no-trunc"}, false, "不截断输出")
		nLatest  = cmd.Bool([]string{"l", "-latest"}, false, "显示最新创建的容器 (包含所有的状态)")
		since    = cmd.String([]string{"#-since"}, "", "显示自容器创建以来的ID或名称 (包含所有的状态)")
		before   = cmd.String([]string{"#-before"}, "", "仅显示容器创建之前的ID或名称")
		last     = cmd.Int([]string{"n"}, -1, "显示n个最新创建的容器 (包含所有的状态)")
		format   = cmd.String([]string{"-format"}, "", "使用一个Go语言的模版打印容器")
		flFilter = opts.NewListOpts(nil)
	)
	cmd.Require(flag.Exact, 0)

	cmd.Var(&flFilter, []string{"f", "-filter"}, "Filter output based on conditions provided")

	cmd.ParseFlags(args, true)
	if *last == -1 && *nLatest {
		*last = 1
	}

	// Consolidate all filter flags, and sanity check them.
	// They'll get processed in the daemon/server.
	for _, f := range flFilter.GetAll() {
		if psFilterArgs, err = filters.ParseFlag(f, psFilterArgs); err != nil {
			return err
		}
	}

	options := types.ContainerListOptions{
		All:    *all,
		Limit:  *last,
		Since:  *since,
		Before: *before,
		Size:   *size,
		Filter: psFilterArgs,
	}

	containers, err := cli.client.ContainerList(options)
	if err != nil {
		return err
	}

	f := *format
	if len(f) == 0 {
		if len(cli.PsFormat()) > 0 && !*quiet {
			f = cli.PsFormat()
		} else {
			f = "table"
		}
	}

	psCtx := formatter.ContainerContext{
		Context: formatter.Context{
			Output: cli.out,
			Format: f,
			Quiet:  *quiet,
			Trunc:  !*noTrunc,
		},
		Size:       *size,
		Containers: containers,
	}

	psCtx.Write()

	return nil
}
