package client

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client/inspect"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/client"
)

// CmdInspect displays low-level information on one or more containers, images or tasks.
//
// Usage: docker inspect [OPTIONS] CONTAINER|IMAGE|TASK [CONTAINER|IMAGE|TASK...]
func (cli *DockerCli) CmdInspect(args ...string) error {
	cmd := Cli.Subcmd("inspect", []string{"[OPTIONS] CONTAINER|IMAGE|TASK [CONTAINER|IMAGE|TASK...]"}, Cli.DockerCommands["inspect"].Description, true)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "基于指定的Go语言模板格式化命令输出内容")
	inspectType := cmd.String([]string{"-type"}, "", "为指定的类型返回JSON内容")
	size := cmd.Bool([]string{"s", "-size"}, false, "如果类型为容器，显示所有的文件大小信息")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *inspectType != "" && *inspectType != "container" && *inspectType != "image" && *inspectType != "task" {
		return fmt.Errorf("对 --type 而言，%q 不是一个有效的值", *inspectType)
	}

	ctx := context.Background()

	var elementSearcher inspect.GetRefFunc
	switch *inspectType {
	case "container":
		elementSearcher = cli.inspectContainers(ctx, *size)
	case "image":
		elementSearcher = cli.inspectImages(ctx, *size)
	case "task":
		if *size {
			fmt.Fprintln(cli.err, "警告: --size 被任务所忽略")
		}
		elementSearcher = cli.inspectTasks(ctx)
	default:
		elementSearcher = cli.inspectAll(ctx, *size)
	}

	return inspect.Inspect(cli.out, cmd.Args(), *tmplStr, elementSearcher)
}

func (cli *DockerCli) inspectContainers(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ContainerInspectWithRaw(ctx, ref, getSize)
	}
}

func (cli *DockerCli) inspectImages(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.ImageInspectWithRaw(ctx, ref, getSize)
	}
}

func (cli *DockerCli) inspectTasks(ctx context.Context) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		return cli.client.TaskInspectWithRaw(ctx, ref)
	}
}

func (cli *DockerCli) inspectAll(ctx context.Context, getSize bool) inspect.GetRefFunc {
	return func(ref string) (interface{}, []byte, error) {
		c, rawContainer, err := cli.client.ContainerInspectWithRaw(ctx, ref, getSize)
		if err != nil {
			// Search for image with that id if a container doesn't exist.
			if client.IsErrContainerNotFound(err) {
				i, rawImage, err := cli.client.ImageInspectWithRaw(ctx, ref, getSize)
				if err != nil {
					if client.IsErrImageNotFound(err) {
						// Search for task with that id if an image doesn't exists.
						t, rawTask, err := cli.client.TaskInspectWithRaw(ctx, ref)
						if err != nil {
							return nil, nil, fmt.Errorf("错误：没有次容器，镜像，任务: %s", ref)
						}
						if getSize {
							fmt.Fprintln(cli.err, "警告: --size 被任务所忽略")
						}
						return t, rawTask, nil
					}
					return nil, nil, err
				}
				return i, rawImage, nil
			}
			return nil, nil, err
		}
		return c, rawContainer, nil
	}
}
