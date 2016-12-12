package registry

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

// NewLogoutCommand creates a new `docker login` command
func NewLogoutCommand(dockerCli *client.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout [SERVER]",
		Short: "登出Docker镜像仓库.",
		Long:  "登出Docker镜像仓库.\n如果没有制定服务端，Docker引擎会采用默认地址.",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var serverAddress string
			if len(args) > 0 {
				serverAddress = args[0]
			}
			return runLogout(dockerCli, serverAddress)
		},
	}

	return cmd
}

func runLogout(dockerCli *client.DockerCli, serverAddress string) error {
	ctx := context.Background()

	if serverAddress == "" {
		serverAddress = dockerCli.ElectAuthServer(ctx)
	}

	// check if we're logged in based on the records in the config file
	// which means it couldn't have user/pass cause they may be in the creds store
	if _, ok := dockerCli.ConfigFile().AuthConfigs[serverAddress]; !ok {
		fmt.Fprintf(dockerCli.Out(), "不能登陆地址 %s\n", serverAddress)
		return nil
	}

	fmt.Fprintf(dockerCli.Out(), "为Docker镜像仓库 %s 删除登陆认证信息\n", serverAddress)
	if err := client.EraseCredentials(dockerCli.ConfigFile(), serverAddress); err != nil {
		fmt.Fprintf(dockerCli.Err(), "警告: 清理认证信息失败: %v\n", err)
	}

	return nil
}
