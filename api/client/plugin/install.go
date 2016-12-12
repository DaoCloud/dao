// +build experimental

package plugin

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

type pluginOptions struct {
	name       string
	grantPerms bool
	disable    bool
}

func newInstallCommand(dockerCli *client.DockerCli) *cobra.Command {
	var options pluginOptions
	cmd := &cobra.Command{
		Use:   "install [OPTIONS] PLUGIN",
		Short: "安装指定插件",
		Args:  cli.ExactArgs(1), // TODO: allow for set args
		RunE: func(cmd *cobra.Command, args []string) error {
			options.name = args[0]
			return runInstall(dockerCli, options)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&options.grantPerms, "grant-all-permissions", false, "为运行插件授予所有的必须权限")
	flags.BoolVar(&options.disable, "disable", false, "不在安装过程中启用插件")

	return cmd
}

func runInstall(dockerCli *client.DockerCli, opts pluginOptions) error {
	named, err := reference.ParseNamed(opts.name) // FIXME: validate
	if err != nil {
		return err
	}
	if reference.IsNameOnly(named) {
		named = reference.WithDefaultTag(named)
	}
	ref, ok := named.(reference.NamedTagged)
	if !ok {
		return fmt.Errorf("无效名称: %s", named.String())
	}

	ctx := context.Background()

	repoInfo, err := registry.ParseRepositoryInfo(named)
	if err != nil {
		return err
	}

	authConfig := dockerCli.ResolveAuthConfig(ctx, repoInfo.Index)

	encodedAuth, err := client.EncodeAuthToBase64(authConfig)
	if err != nil {
		return err
	}

	registryAuthFunc := dockerCli.RegistryAuthenticationPrivilegedFunc(repoInfo.Index, "plugin install")

	options := types.PluginInstallOptions{
		RegistryAuth:          encodedAuth,
		Disabled:              opts.disable,
		AcceptAllPermissions:  opts.grantPerms,
		AcceptPermissionsFunc: acceptPrivileges(dockerCli, opts.name),
		// TODO: Rename PrivilegeFunc, it has nothing to do with privileges
		PrivilegeFunc: registryAuthFunc,
	}
	if err := dockerCli.Client().PluginInstall(ctx, ref.String(), options); err != nil {
		return err
	}
	fmt.Fprintln(dockerCli.Out(), opts.name)
	return nil
}

func acceptPrivileges(dockerCli *client.DockerCli, name string) func(privileges types.PluginPrivileges) (bool, error) {
	return func(privileges types.PluginPrivileges) (bool, error) {
		fmt.Fprintf(dockerCli.Out(), "插件 %q 正在请求以下特权:\n", name)
		for _, privilege := range privileges {
			fmt.Fprintf(dockerCli.Out(), " - %s: %v\n", privilege.Name, privilege.Value)
		}

		fmt.Fprint(dockerCli.Out(), "您允许授予以下特权? [y/N] ")
		reader := bufio.NewReader(dockerCli.In())
		line, _, err := reader.ReadLine()
		if err != nil {
			return false, err
		}
		return strings.ToLower(string(line)) == "y", nil
	}
}
