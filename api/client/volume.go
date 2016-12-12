package client

import (
	"fmt"
	"text/tabwriter"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
)

// CmdVolume is the parent subcommand for all volume commands
//
// Usage: docker volume <COMMAND> <OPTS>
func (cli *DockerCli) CmdVolume(args ...string) error {
	description := Cli.DockerCommands["volume"].Description + "\n\nCommands:\n"
	commands := [][]string{
		{"创建", "创建一个数据存储卷"},
		{"显示", "显示一个存储卷的详细信息"},
		{"罗列", "罗列所有存储卷"},
		{"删除", "删除一个存储卷"},
	}

	for _, cmd := range commands {
		description += fmt.Sprintf("  %-25.25s%s\n", cmd[0], cmd[1])
	}

	description += "\n运行 'docker volume COMMAND --help' 获取更多有关volume命令的信息"
	cmd := Cli.Subcmd("volume", []string{"[COMMAND]"}, description, false)

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdVolumeLs outputs a list of Docker volumes.
//
// Usage: docker volume ls [OPTIONS]
func (cli *DockerCli) CmdVolumeLs(args ...string) error {
	cmd := Cli.Subcmd("volume ls", nil, "罗列所有存储卷", true)

	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "仅显示存储卷名称")
	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "提供过滤信息 (比如 'dangling=true')")

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	volFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		var err error
		volFilterArgs, err = filters.ParseFlag(f, volFilterArgs)
		if err != nil {
			return err
		}
	}

	volumes, err := cli.client.VolumeList(volFilterArgs)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)
	if !*quiet {
		for _, warn := range volumes.Warnings {
			fmt.Fprintln(cli.err, warn)
		}
		fmt.Fprintf(w, "DRIVER \tVOLUME NAME")
		fmt.Fprintf(w, "\n")
	}

	for _, vol := range volumes.Volumes {
		if *quiet {
			fmt.Fprintln(w, vol.Name)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", vol.Driver, vol.Name)
	}
	w.Flush()
	return nil
}

// CmdVolumeInspect displays low-level information on one or more volumes.
//
// Usage: docker volume inspect [OPTIONS] VOLUME [VOLUME...]
func (cli *DockerCli) CmdVolumeInspect(args ...string) error {
	cmd := Cli.Subcmd("volume inspect", []string{"VOLUME [VOLUME...]"}, "显示一个存储卷的底层信息", true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "基于指定的Go语言模版格式化命令输出内容")

	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	if err := cmd.Parse(args); err != nil {
		return nil
	}

	inspectSearcher := func(name string) (interface{}, []byte, error) {
		i, err := cli.client.VolumeInspect(name)
		return i, nil, err
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), inspectSearcher)
}

// CmdVolumeCreate creates a new volume.
//
// Usage: docker volume create [OPTIONS]
func (cli *DockerCli) CmdVolumeCreate(args ...string) error {
	cmd := Cli.Subcmd("volume create", nil, "创建一个数据存储卷", true)
	flDriver := cmd.String([]string{"d", "-driver"}, "local", "指定存储驱动的名称")
	flName := cmd.String([]string{"-name"}, "", "指定存储卷的名称")

	flDriverOpts := opts.NewMapOpts(nil, nil)
	cmd.Var(flDriverOpts, []string{"o", "-opt"}, "设置驱动的指定参数")

	cmd.Require(flag.Exact, 0)
	cmd.ParseFlags(args, true)

	volReq := types.VolumeCreateRequest{
		Driver:     *flDriver,
		DriverOpts: flDriverOpts.GetAll(),
		Name:       *flName,
	}

	vol, err := cli.client.VolumeCreate(volReq)
	if err != nil {
		return err
	}

	fmt.Fprintf(cli.out, "%s\n", vol.Name)
	return nil
}

// CmdVolumeRm removes one or more volumes.
//
// Usage: docker volume rm VOLUME [VOLUME...]
func (cli *DockerCli) CmdVolumeRm(args ...string) error {
	cmd := Cli.Subcmd("volume rm", []string{"VOLUME [VOLUME...]"}, "删除一个存储卷", true)
	cmd.Require(flag.Min, 1)
	cmd.ParseFlags(args, true)

	var status = 0

	for _, name := range cmd.Args() {
		if err := cli.client.VolumeRemove(name); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(cli.out, "%s\n", name)
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}
