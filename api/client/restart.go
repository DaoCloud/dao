package client

import (
	"fmt"
	"strings"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRestart restarts one or more containers.
//
// Usage: docker restart [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdRestart(args ...string) error {
	cmd := Cli.Subcmd("restart", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["restart"].Description, true)
	nSeconds := cmd.Int([]string{"t", "-time"}, 10, "在终止一个容器前，等待容器停止的秒杀")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errs []string
	for _, name := range cmd.Args() {
		if err := cli.client.ContainerRestart(name, *nSeconds); err != nil {
			errs = append(errs, fmt.Sprintf("未能终止容器 (%s): %s", name, err))
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
