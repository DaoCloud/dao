package client

import (
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/docker/docker/api/client/inspect"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/client"
)

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
}

// CmdInspect displays low-level information on one or more containers or images.
//
// Usage: docker inspect [OPTIONS] CONTAINER|IMAGE [CONTAINER|IMAGE...]
func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := Cli.Subcmd("inspect", []string{"CONTAINER|IMAGE [CONTAINER|IMAGE...]"}, Cli.DockerCommands["inspect"].Description, true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "基于指定的Go语言模版格式化命令输出内容")
	inspectType := cmd.String([]string{"-type"}, "", "为指定的类型返回JSON内容, (例如：镜像或容器)")
	size := cmd.Bool([]string{"s", "-size"}, false, "如果类型为容器，显示所有的文件大小信息")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *inspectType != "" && *inspectType != "container" && *inspectType != "image" {
		return fmt.Errorf("对 --type 而言, %q 不是一个有效的值", *inspectType)
	}

	var elementSearcher inspectSearcher
	switch *inspectType {
	case "container":
		elementSearcher = cli.inspectContainers(*size)
	case "image":
		elementSearcher = cli.inspectImages(*size)
	default:
		elementSearcher = cli.inspectAll(*size)
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), elementSearcher)
}

func (cli *DockerCli) inspectContainers(getSize bool) inspectSearcher {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ContainerInspectWithRaw(ref, getSize)
	}
}

func (cli *DockerCli) inspectImages(getSize bool) inspectSearcher {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ImageInspectWithRaw(ref, getSize)
	}
}

func (cli *DockerCli) inspectAll(getSize bool) inspectSearcher {
	return func(ref string) (interface{}, []byte, error) {
		c, rawContainer, err := cli.client.ContainerInspectWithRaw(ref, getSize)
		if err != nil {
			// Search for image with that id if a container doesn't exist.
			if client.IsErrContainerNotFound(err) {
				i, rawImage, err := cli.client.ImageInspectWithRaw(ref, getSize)
				if err != nil {
					if client.IsErrImageNotFound(err) {
						return nil, nil, fmt.Errorf("错误: 没有此容器, 镜像: %s", ref)
					}
					return nil, nil, err
				}
				return i, rawImage, err
			}
			return nil, nil, err
		}
		return c, rawContainer, err
	}
}

type inspectSearcher func(ref string) (interface{}, []byte, error)

func (cli *DockerCli) inspectElements(tmplStr string, references []string, searchByReference inspectSearcher) error {
	elementInspector, err := cli.newInspectorWithTemplate(tmplStr)
	if err != nil {
		return Cli.StatusError{StatusCode: 64, Status: err.Error()}
	}

	var inspectErr error
	for _, ref := range references {
		element, raw, err := searchByReference(ref)
		if err != nil {
			inspectErr = err
			break
		}

		if err := elementInspector.Inspect(element, raw); err != nil {
			inspectErr = err
			break
		}
	}

	if err := elementInspector.Flush(); err != nil {
		cli.inspectErrorStatus(err)
	}

	if status := cli.inspectErrorStatus(inspectErr); status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

func (cli *DockerCli) inspectErrorStatus(err error) (status int) {
	if err != nil {
		fmt.Fprintf(cli.err, "%s\n", err)
		status = 1
	}
	return
}

func (cli *DockerCli) newInspectorWithTemplate(tmplStr string) (inspect.Inspector, error) {
	elementInspector := inspect.NewIndentedInspector(cli.out)
	if tmplStr != "" {
		tmpl, err := template.New("").Funcs(funcMap).Parse(tmplStr)
		if err != nil {
			return nil, fmt.Errorf("Template parsing error: %s", err)
		}
		elementInspector = inspect.NewTemplateInspector(cli.out, tmpl)
	}
	return elementInspector, nil
}
