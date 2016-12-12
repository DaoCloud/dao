package client

import (
	"fmt"
	"io"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/engine-api/types"
)

var validDrivers = map[string]bool{
	"json-file": true,
	"journald":  true,
}

// CmdLogs fetches the logs of a given container.
//
// docker logs [OPTIONS] CONTAINER
func (cli *DockerCli) CmdLogs(args ...string) error {
	cmd := Cli.Subcmd("logs", []string{"CONTAINER"}, Cli.DockerCommands["logs"].Description, true)
	follow := cmd.Bool([]string{"f", "-follow"}, false, "跟踪容器日志输出")
	since := cmd.String([]string{"-since"}, "", "从某一个时间戳开始获取日志")
	times := cmd.Bool([]string{"t", "-timestamps"}, false, "显示日志的时间戳")
	tail := cmd.String([]string{"-tail"}, "all", "从日志尾部往上显示制定数量行数的日志, all代表显示所有日志")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	name := cmd.Arg(0)

	c, err := cli.client.ContainerInspect(name)
	if err != nil {
		return err
	}

	if !validDrivers[c.HostConfig.LogConfig.Type] {
		return fmt.Errorf("\"logs\" 命令只支持 \"json-file\" and \"journald\" 日志驱动 (当前日志驱动为: %s)", c.HostConfig.LogConfig.Type)
	}

	options := types.ContainerLogsOptions{
		ContainerID: name,
		ShowStdout:  true,
		ShowStderr:  true,
		Since:       *since,
		Timestamps:  *times,
		Follow:      *follow,
		Tail:        *tail,
	}
	responseBody, err := cli.client.ContainerLogs(options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if c.Config.Tty {
		_, err = io.Copy(cli.out, responseBody)
	} else {
		_, err = stdcopy.StdCopy(cli.out, cli.err, responseBody)
	}
	return err
}
