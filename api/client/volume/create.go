package volume

import (
	"fmt"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type createOptions struct {
	name       string
	driver     string
	driverOpts opts.MapOpts
	labels     []string
}

func newCreateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := createOptions{
		driverOpts: *opts.NewMapOpts(nil, nil),
	}

	cmd := &cobra.Command{
		Use:   "create [OPTIONS]",
		Short: "创建一个数据存储卷",
		Long:  createDescription,
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreate(dockerCli, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.driver, "driver", "d", "local", "指定存储驱动的名称")
	flags.StringVar(&opts.name, "name", "", "指定存储卷的名称")
	flags.VarP(&opts.driverOpts, "opt", "o", "设置驱动的指定参数")
	flags.StringSliceVar(&opts.labels, "label", []string{}, "为一个存储卷设置元数据")

	return cmd
}

func runCreate(dockerCli *client.DockerCli, opts createOptions) error {
	client := dockerCli.Client()

	volReq := types.VolumeCreateRequest{
		Driver:     opts.driver,
		DriverOpts: opts.driverOpts.GetAll(),
		Name:       opts.name,
		Labels:     runconfigopts.ConvertKVStringsToMap(opts.labels),
	}

	vol, err := client.VolumeCreate(context.Background(), volReq)
	if err != nil {
		return err
	}

	fmt.Fprintf(dockerCli.Out(), "%s\n", vol.Name)
	return nil
}

var createDescription = `
Creates a new volume that containers can consume and store data in. If a name
is not specified, Docker generates a random name. You create a volume and then
configure the container to use it, for example:

    $ docker volume create --name hello
    hello
    $ docker run -d -v hello:/world busybox ls /world

The mount is created inside the container's **/src** directory. Docker doesn't
not support relative paths for mount points inside the container.

Multiple containers can use the same volume in the same time period. This is
useful if two containers need access to shared data. For example, if one
container writes and the other reads the data.

## Driver specific options

Some volume drivers may take options to customize the volume creation. Use the
**-o** or **--opt** flags to pass driver options:

    $ docker volume create --driver fake --opt tardis=blue --opt timey=wimey

These options are passed directly to the volume driver. Options for different
volume drivers may do different things (or nothing at all).

The built-in **local** driver on Windows does not support any options.

The built-in **local** driver on Linux accepts options similar to the linux
**mount** command:

    $ docker volume create --driver local --opt type=tmpfs --opt device=tmpfs --opt o=size=100m,uid=1000

Another example:

    $ docker volume create --driver local --opt type=btrfs --opt device=/dev/sda2

`
