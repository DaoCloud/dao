package client

import (
	"fmt"
	"strings"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdRename renames a container.
//
// Usage: docker rename OLD_NAME NEW_NAME
func (cli *DockerCli) CmdRename(args ...string) error {
	cmd := Cli.Subcmd("rename", []string{"OLD_NAME NEW_NAME"}, Cli.DockerCommands["rename"].Description, true)
	cmd.Require(flag.Exact, 2)

	cmd.ParseFlags(args, true)

	oldName := strings.TrimSpace(cmd.Arg(0))
	newName := strings.TrimSpace(cmd.Arg(1))

	if oldName == "" || newName == "" {
		return fmt.Errorf("错误: 新名称和旧名称均不能为空")
	}

	if err := cli.client.ContainerRename(oldName, newName); err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		return fmt.Errorf("错误: 重命名容器 %s 失败", oldName)
	}
	return nil
}
