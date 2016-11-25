package container

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/spf13/cobra"
)

type unpauseOptions struct {
	containers []string
}

// NewUnpauseCommand creats a new cobra.Command for `docker unpause`
func NewUnpauseCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts unpauseOptions

	cmd := &cobra.Command{
		Use:   "unpause CONTAINER [CONTAINER...]",
		Short: "恢复一个或多个容器中所有被挂起的进程",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.containers = args
			return runUnpause(dockerCli, &opts)
		},
	}

	return cmd
}

func runUnpause(dockerCli *client.DockerCli, opts *unpauseOptions) error {
	ctx := context.Background()

	var errs []string
	for _, container := range opts.containers {
		if err := dockerCli.Client().ContainerUnpause(ctx, container); err != nil {
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
