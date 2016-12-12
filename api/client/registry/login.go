package registry

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	serverAddress string
	user          string
	password      string
	email         string
}

// NewLoginCommand creates a new `docker login` command
func NewLoginCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts loginOptions

	cmd := &cobra.Command{
		Use:   "login [OPTIONS] [SERVER]",
		Short: "登陆一个Docker镜像仓库.",
		Long:  "登陆一个Docker镜像仓库.\n如果没有制定服务器, Docker引擎会采用默认地址.",
		Args:  cli.RequiresMaxArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.serverAddress = args[0]
			}
			return runLogin(dockerCli, opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.user, "username", "u", "", "Username")
	flags.StringVarP(&opts.password, "password", "p", "", "Password")

	// Deprecated in 1.11: Should be removed in docker 1.13
	flags.StringVarP(&opts.email, "email", "e", "", "Email")
	flags.MarkDeprecated("email", "will be removed in 1.13.")

	return cmd
}

func runLogin(dockerCli *client.DockerCli, opts loginOptions) error {
	ctx := context.Background()
	clnt := dockerCli.Client()

	var serverAddress string
	var isDefaultRegistry bool
	if opts.serverAddress != "" {
		serverAddress = opts.serverAddress
	} else {
		serverAddress = dockerCli.ElectAuthServer(ctx)
		isDefaultRegistry = true
	}
	authConfig, err := dockerCli.ConfigureAuth(opts.user, opts.password, serverAddress, isDefaultRegistry)
	if err != nil {
		return err
	}
	response, err := clnt.RegistryLogin(ctx, authConfig)
	if err != nil {
		return err
	}
	if response.IdentityToken != "" {
		authConfig.Password = ""
		authConfig.IdentityToken = response.IdentityToken
	}
	if err := client.StoreCredentials(dockerCli.ConfigFile(), authConfig); err != nil {
		return fmt.Errorf("保存认证信息失败: %v", err)
	}

	if response.Status != "" {
		fmt.Fprintln(dockerCli.Out(), response.Status)
	}
	return nil
}
