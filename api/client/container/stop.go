package container

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type stopOptions struct {
	time int

	containers []string
}

// NewStopCommand creats a new cobra.Command for `docker stop`
func NewStopCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [OPTIONS] CONTAINER [CONTAINER...]",
		Short: "停止一个或多个运行容器",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runStop(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.IntVarP(&opts.time, "time", "t", 10, "终止容器前等待容器停止的秒数")
	return cmd
}

func runStop(dockerCli *client.DockerCli, opts *stopOptions) error {
	ctx := context.Background()

	var errs []string
	for _, container := range opts.containers {
		timeout := time.Duration(opts.time) * time.Second
		if err := dockerCli.Client().ContainerStop(ctx, container, &timeout); err != nil {
			errs = append(errs, err.Error())
		} else {
			fmt.Fprintf(dockerCli.Out(), "%s\n", container)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
