package client

import (
	"fmt"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdLogout logs a user out from a Docker registry.
//
// If no server is specified, the user will be logged out from the registry's index server.
//
// Usage: docker logout [SERVER]
func (cli *DockerCli) CmdLogout(args ...string) error {
	cmd := Cli.Subcmd("logout", []string{"[SERVER]"}, Cli.DockerCommands["logout"].Description+".\n如果没有定制服务端，Docker引擎会采用默认地址", true)
	cmd.Require(flag.Max, 1)

	cmd.ParseFlags(args, true)

	var serverAddress string
	if len(cmd.Args()) > 0 {
		serverAddress = cmd.Arg(0)
	} else {
		serverAddress = cli.electAuthServer()
	}

	if _, ok := cli.configFile.AuthConfigs[serverAddress]; !ok {
		fmt.Fprintf(cli.out, "不能登录地址 %s\n", serverAddress)
		return nil
	}

	fmt.Fprintf(cli.out, "删除登录认证信息 %s\n", serverAddress)
	delete(cli.configFile.AuthConfigs, serverAddress)
	if err := cli.configFile.Save(); err != nil {
		return fmt.Errorf("保存Docker配置信息失败: %v", err)
	}

	return nil
}
