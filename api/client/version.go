package client

import (
	"runtime"
	"text/template"
	"time"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/utils"
	"github.com/docker/engine-api/types"
)

var versionTemplate = `Client:
 版本:      {{.Client.Version}}
 API版本:  {{.Client.APIVersion}}
 Go版本:   {{.Client.GoVersion}}
 Git提交号:   {{.Client.GitCommit}}
 构建:        {{.Client.BuildTime}}
 操作系统/架构:      {{.Client.Os}}/{{.Client.Arch}}{{if .Client.Experimental}}
 试验版: {{.Client.Experimental}}{{end}}{{if .ServerOK}}

Server:
 版本:      {{.Server.Version}}
 API版本:  {{.Server.APIVersion}}
 Go版本:   {{.Server.GoVersion}}
 Git提交号:   {{.Server.GitCommit}}
 构建:        {{.Server.BuildTime}}
 操作系统/架构:      {{.Server.Os}}/{{.Server.Arch}}{{if .Server.Experimental}}
 试验版: {{.Server.Experimental}}{{end}}{{end}}`

// CmdVersion shows Docker version information.
//
// Available version information is shown for: client Docker version, client API version, client Go version, client Git commit, client OS/Arch, server Docker version, server API version, server Go version, server Git commit, and server OS/Arch.
//
// Usage: docker version
func (cli *DockerCli) CmdVersion(args ...string) (err error) {
	cmd := Cli.Subcmd("version", nil, Cli.DockerCommands["version"].Description, true)
	tmplStr := cmd.String([]string{"f", "#format", "-format"}, "", "基于指定的Go语言模版格式化命令输出内容")
	cmd.Require(flag.Exact, 0)

	cmd.ParseFlags(args, true)

	templateFormat := versionTemplate
	if *tmplStr != "" {
		templateFormat = *tmplStr
	}

	var tmpl *template.Template
	if tmpl, err = template.New("").Funcs(funcMap).Parse(templateFormat); err != nil {
		return Cli.StatusError{StatusCode: 64,
			Status: "Template parsing error: " + err.Error()}
	}

	vd := types.VersionResponse{
		Client: &types.Version{
			Version:      dockerversion.Version,
			APIVersion:   cli.client.ClientVersion(),
			GoVersion:    runtime.Version(),
			GitCommit:    dockerversion.GitCommit,
			BuildTime:    dockerversion.BuildTime,
			Os:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			Experimental: utils.ExperimentalBuild(),
		},
	}

	serverVersion, err := cli.client.ServerVersion()
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

	if err2 := tmpl.Execute(cli.out, vd); err2 != nil && err == nil {
		err = err2
	}
	cli.out.Write([]byte{'\n'})
	return err
}
