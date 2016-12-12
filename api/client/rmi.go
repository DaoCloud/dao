package client

import (
	"fmt"
	"net/url"
	"strings"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/engine-api/types"
)

// CmdRmi removes all images with the specified name(s).
//
// Usage: docker rmi [OPTIONS] IMAGE [IMAGE...]
func (cli *DockerCli) CmdRmi(args ...string) error {
	cmd := Cli.Subcmd("rmi", []string{"IMAGE [IMAGE...]"}, Cli.DockerCommands["rmi"].Description, true)
	force := cmd.Bool([]string{"f", "-force"}, false, "强制删除镜像")
	noprune := cmd.Bool([]string{"-no-prune"}, false, "不删除没有标签的父镜像")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	v := url.Values{}
	if *force {
		v.Set("force", "1")
	}
	if *noprune {
		v.Set("noprune", "1")
	}

	var errs []string
	for _, name := range cmd.Args() {
		options := types.ImageRemoveOptions{
			ImageID:       name,
			Force:         *force,
			PruneChildren: !*noprune,
		}

		dels, err := cli.client.ImageRemove(options)
		if err != nil {
			errs = append(errs, fmt.Sprintf("未能删除镜像 (%s): %s", name, err))
		} else {
			for _, del := range dels {
				if del.Deleted != "" {
					fmt.Fprintf(cli.out, "已删除: %s\n", del.Deleted)
				} else {
					fmt.Fprintf(cli.out, "去标签: %s\n", del.Untagged)
				}
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}
