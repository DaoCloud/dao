package system

import (
	"runtime"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/utils/templates"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

var versionTemplate = `Client:
 版本:      {{.Client.Version}}
 API版本:  {{.Client.APIVersion}}
 Go 版本:   {{.Client.GoVersion}}
 Git提交号:   {{.Client.GitCommit}}
 构建:        {{.Client.BuildTime}}
 操作系统/架构:      {{.Client.Os}}/{{.Client.Arch}}{{if .Client.Experimental}}
 试验版: {{.Client.Experimental}}{{end}}{{if .ServerOK}}
Server:
 版本:      {{.Server.Version}}
 API 版本:  {{.Server.APIVersion}}
 Go 版本:   {{.Server.GoVersion}}
 Git提交号:   {{.Server.GitCommit}}
 构建:        {{.Server.BuildTime}}
 操作系统/架构:      {{.Server.Os}}/{{.Server.Arch}}{{if .Server.Experimental}}
 试验版: {{.Server.Experimental}}{{end}}{{end}}`

type versionOptions struct {
	format string
}

// NewVersionCommand creats a new cobra.Command for `docker version`
func NewVersionCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts versionOptions

	cmd := &cobra.Command{
		Use:   "version [OPTIONS]",
		Short: "显示Docker的版本信息",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.format, "format", "f", "", "基于指定的Go语言模板格式化命令输出内容")

	return cmd
}

func runVersion(dockerCli *client.DockerCli, opts *versionOptions) error {
	ctx := context.Background()

	templateFormat := versionTemplate
	if opts.format != "" {
		templateFormat = opts.format
	}

	tmpl, err := templates.Parse(templateFormat)
	if err != nil {
		return cli.StatusError{StatusCode: 64,
			Status: "Template parsing error: " + err.Error()}
	}

	vd := types.VersionResponse{
		Client: &types.Version{
			Version:      dockerversion.Version,
			APIVersion:   dockerCli.Client().ClientVersion(),
			GoVersion:    runtime.Version(),
			GitCommit:    dockerversion.GitCommit,
			BuildTime:    dockerversion.BuildTime,
			Os:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			Experimental: utils.ExperimentalBuild(),
		},
	}

	serverVersion, err := dockerCli.Client().ServerVersion(ctx)
	if err == nil {
		vd.Server = &serverVersion
	}

	// first we need to make BuildTime more human friendly
	t, errTime := time.Parse(time.RFC3339Nano, vd.Client.BuildTime)
	if errTime == nil {
		vd.Client.BuildTime = t.Format(time.ANSIC)
	}

	if vd.ServerOK() {
		t, errTime = time.Parse(time.RFC3339Nano, vd.Server.BuildTime)
		if errTime == nil {
			vd.Server.BuildTime = t.Format(time.ANSIC)
		}
	}

	if err2 := tmpl.Execute(dockerCli.Out(), vd); err2 != nil && err == nil {
		err = err2
	}
	dockerCli.Out().Write([]byte{'\n'})
	return err
}
