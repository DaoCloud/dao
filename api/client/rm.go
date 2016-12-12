package client

import (
	"fmt"
	"strings"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
)

// CmdRm removes one or more containers.
//
// Usage: docker rm [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdRm(args ...string) error {
	cmd := Cli.Subcmd("rm", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["rm"].Description, true)
	v := cmd.Bool([]string{"v", "-volumes"}, false, "删除容器时同时删除容器相关的存储卷")
	link := cmd.Bool([]string{"l", "-link"}, false, "删除制定的链接")
	force := cmd.Bool([]string{"f", "-force"}, false, "强制删除一个运行的容器 (使用信号 SIGKILL)")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var errs []string
	for _, name := range cmd.Args() {
		if name == "" {
			return fmt.Errorf("容器名不能为空")
		}
		name = strings.Trim(name, "/")

		options := types.ContainerRemoveOptions{
			ContainerID:   name,
			RemoveVolumes: *v,
			RemoveLinks:   *link,
			Force:         *force,
		}

		if err := cli.client.ContainerRemove(options); err != nil {
			errs = append(errs, fmt.Sprintf("未能删除容器 (%s): %s", name, err))
		} else {
			fmt.Fprintf(cli.out, "%s\n", name)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
