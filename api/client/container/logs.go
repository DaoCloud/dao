package container

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

var validDrivers = map[string]bool{
	"json-file": true,
	"journald":  true,
}

type logsOptions struct {
	follow     bool
	since      string
	timestamps bool
	details    bool
	tail       string

	container string
}

// NewLogsCommand creats a new cobra.Command for `docker logs`
func NewLogsCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts logsOptions

	cmd := &cobra.Command{
		Use:   "logs [OPTIONS] CONTAINER",
		Short: "获取一个容器的运行日志",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runLogs(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(&opts.follow, "follow", "f", false, "跟踪容器日志输出")
	flags.StringVar(&opts.since, "since", "", "从某一个时间戳开始获取日志")
	flags.BoolVarP(&opts.timestamps, "timestamps", "t", false, "显示日志的时间戳")
	flags.BoolVar(&opts.details, "details", false, "显示提供给日志的额外细节")
	flags.StringVar(&opts.tail, "tail", "all", "从日志尾部往上显示制定数量行数的日志，all代表显示所有日志")
	return cmd
}

func runLogs(dockerCli *client.DockerCli, opts *logsOptions) error {
	ctx := context.Background()

	c, err := dockerCli.Client().ContainerInspect(ctx, opts.container)
	if err != nil {
		return err
	}

	if !validDrivers[c.HostConfig.LogConfig.Type] {
		return fmt.Errorf("\"logs\" 命令只支持\"json-file\"和\"journald\"日志驱动(当前日志驱动为: %s)", c.HostConfig.LogConfig.Type)
	}

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      opts.since,
		Timestamps: opts.timestamps,
		Follow:     opts.follow,
		Tail:       opts.tail,
		Details:    opts.details,
	}
	responseBody, err := dockerCli.Client().ContainerLogs(ctx, opts.container, options)
	if err != nil {
		return err
	}
	defer responseBody.Close()

	if c.Config.Tty {
		_, err = io.Copy(dockerCli.Out(), responseBody)
	} else {
		_, err = stdcopy.StdCopy(dockerCli.Out(), dockerCli.Err(), responseBody)
	}
	return err
}
