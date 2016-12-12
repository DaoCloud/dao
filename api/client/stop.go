package client

import (
	"fmt"
	"strings"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdStop stops one or more containers.
//
// A running container is stopped by first sending SIGTERM and then SIGKILL if the container fails to stop within a grace period (the default is 10 seconds).
//
// Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStop(args ...string) error {
	cmd := Cli.Subcmd("stop", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["stop"].Description+".\n发送 SIGTERM 信号，在容器自动停止的宽限期后发送 SIGKILL 信号", true)
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "终止容器前等待容器停止的秒数")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errs []string
	for _, name := range cmd.Args() {
		if err := cli.client.ContainerStop(name, *nSeconds); err != nil {
			errs = append(errs, fmt.Sprintf("未能停止容器 (%s): %s", name, err))
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
