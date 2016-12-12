package client

import (
	"encoding/json"
	"errors"
	"fmt"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/reference"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/container"
)

// CmdCommit creates a new image from a container's changes.
//
// Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]
func (cli *DockerCli) CmdCommit(args ...string) error {
	cmd := Cli.Subcmd("commit", []string{"CONTAINER [REPOSITORY[:TAG]]"}, Cli.DockerCommands["commit"].Description, true)
	flPause := cmd.Bool([]string{"p", "-pause"}, true, "在容器提交镜像过程中先暂停容器运行")
	flComment := cmd.String([]string{"m", "-message"}, "", "容器提交镜像的消息")
	flAuthor := cmd.String([]string{"a", "-author"}, "", "执行提交操作的作者 (比如, \"张三 <hannibal@a-team.com>\")")
	flChanges := opts.NewListOpts(nil)
	cmd.Var(&flChanges, []string{"c", "-change"}, "在创建的镜像中添加Dockerfile中的指令")
	// FIXME: --run is deprecated, it will be replaced with inline Dockerfile commands.
	flConfig := cmd.String([]string{"#-run"}, "", "该参数已经被废弃，并会在未来的版本中被移除")
	cmd.Require(flag.Max, 2)
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	var (
		name             = cmd.Arg(0)
		repositoryAndTag = cmd.Arg(1)
		repositoryName   string
		tag              string
	)

	//Check if the given image name can be resolved
	if repositoryAndTag != "" {
		ref, err := reference.ParseNamed(repositoryAndTag)
		if err != nil {
			return err
		}

		repositoryName = ref.Name()

		switch x := ref.(type) {
		case reference.Canonical:
			return errors.New("cannot commit to digest reference")
		case reference.NamedTagged:
			tag = x.Tag()
		}
	}

	var config *container.Config
	if *flConfig != "" {
		config = &container.Config{}
		if err := json.Unmarshal([]byte(*flConfig), config); err != nil {
			return err
		}
	}

	options := types.ContainerCommitOptions{
		ContainerID:    name,
		RepositoryName: repositoryName,
		Tag:            tag,
		Comment:        *flComment,
		Author:         *flAuthor,
		Changes:        flChanges.GetAll(),
		Pause:          *flPause,
		Config:         config,
	}

	response, err := cli.client.ContainerCommit(options)
	if err != nil {
		return err
	}

	fmt.Fprintln(cli.out, response.ID)
	return nil
}
