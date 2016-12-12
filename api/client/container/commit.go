package container

import (
	"encoding/json"
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	dockeropts "github.com/docker/docker/opts"
	"github.com/docker/engine-api/types"
	containertypes "github.com/docker/engine-api/types/container"
	"github.com/spf13/cobra"
)

type commitOptions struct {
	container string
	reference string

	pause   bool
	comment string
	author  string
	changes dockeropts.ListOpts
	config  string
}

// NewCommitCommand creats a new cobra.Command for `docker commit`
func NewCommitCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts commitOptions

	cmd := &cobra.Command{
		Use:   "commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]",
		Short: "从一个容器的变化部分创建一个新的镜像",
		Args:  cli.RequiresRangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			if len(args) > 1 {
				opts.reference = args[1]
			}
			return runCommit(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.SetInterspersed(false)

	flags.BoolVarP(&opts.pause, "pause", "p", true, "在容器提交镜像过程中先暂停容器运行")
	flags.StringVarP(&opts.comment, "message", "m", "", "容器提交镜像的消息")
	flags.StringVarP(&opts.author, "author", "a", "", "执行提交操作的作者 (比如, \"张三 <hannibal@a-team.com>\")")

	opts.changes = dockeropts.NewListOpts(nil)
	flags.VarP(&opts.changes, "change", "c", "在创建的镜像中添加Dockerfile中的指令")

	// FIXME: --run is deprecated, it will be replaced with inline Dockerfile commands.
	flags.StringVar(&opts.config, "run", "", "该参数已经被废弃，并会在未来的版本中被移除")
	flags.MarkDeprecated("run", "此选项将来会被Dockerfile内部的命令所替代")

	return cmd
}

func runCommit(dockerCli *client.DockerCli, opts *commitOptions) error {
	ctx := context.Background()

	name := opts.container
	reference := opts.reference

	var config *containertypes.Config
	if opts.config != "" {
		config = &containertypes.Config{}
		if err := json.Unmarshal([]byte(opts.config), config); err != nil {
			return err
		}
	}

	options := types.ContainerCommitOptions{
		Reference: reference,
		Comment:   opts.comment,
		Author:    opts.author,
		Changes:   opts.changes.GetAll(),
		Pause:     opts.pause,
		Config:    config,
	}

	response, err := dockerCli.Client().ContainerCommit(ctx, name, options)
	if err != nil {
		return err
	}

	fmt.Fprintln(dockerCli.Out(), response.ID)
	return nil
}
