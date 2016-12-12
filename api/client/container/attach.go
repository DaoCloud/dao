package container

import (
	"fmt"
	"io"
	"net/http/httputil"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types"
	"github.com/spf13/cobra"
)

type attachOptions struct {
	noStdin    bool
	proxy      bool
	detachKeys string

	container string
}

// NewAttachCommand creats a new cobra.Command for `docker attach`
func NewAttachCommand(dockerCli *client.DockerCli) *cobra.Command {
	var opts attachOptions

	cmd := &cobra.Command{
		Use:   "attach [OPTIONS] CONTAINER",
		Short: "附加到一个运行的容器，包含标准输入，标准输出，标准错误",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.container = args[0]
			return runAttach(dockerCli, &opts)
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&opts.noStdin, "no-stdin", false, "不附加标准输入")
	flags.BoolVar(&opts.proxy, "sig-proxy", true, "代理所有接收到的信号至进程")
	flags.StringVar(&opts.detachKeys, "detach-keys", "", "覆盖从一个容器停止附加的输入键顺序")
	return cmd
}

func runAttach(dockerCli *client.DockerCli, opts *attachOptions) error {
	ctx := context.Background()

	c, err := dockerCli.Client().ContainerInspect(ctx, opts.container)
	if err != nil {
		return err
	}

	if !c.State.Running {
		return fmt.Errorf("您不能附加到一个停止的容器中，请先启动此容器。")
	}

	if c.State.Paused {
		return fmt.Errorf("您不能附加到一个暂停的容器中，请先启动此容器。")
	}

	if err := dockerCli.CheckTtyInput(!opts.noStdin, c.Config.Tty); err != nil {
		return err
	}

	if opts.detachKeys != "" {
		dockerCli.ConfigFile().DetachKeys = opts.detachKeys
	}

	options := types.ContainerAttachOptions{
		Stream:     true,
		Stdin:      !opts.noStdin && c.Config.OpenStdin,
		Stdout:     true,
		Stderr:     true,
		DetachKeys: dockerCli.ConfigFile().DetachKeys,
	}

	var in io.ReadCloser
	if options.Stdin {
		in = dockerCli.In()
	}

	if opts.proxy && !c.Config.Tty {
		sigc := dockerCli.ForwardAllSignals(ctx, opts.container)
		defer signal.StopCatch(sigc)
	}

	resp, errAttach := dockerCli.Client().ContainerAttach(ctx, opts.container, options)
	if errAttach != nil && errAttach != httputil.ErrPersistEOF {
		// ContainerAttach returns an ErrPersistEOF (connection closed)
		// means server met an error and put it in Hijacked connection
		// keep the error and read detailed error message from hijacked connection later
		return errAttach
	}
	defer resp.Close()

	if c.Config.Tty && dockerCli.IsTerminalOut() {
		height, width := dockerCli.GetTtySize()
		// To handle the case where a user repeatedly attaches/detaches without resizing their
		// terminal, the only way to get the shell prompt to display for attaches 2+ is to artificially
		// resize it, then go back to normal. Without this, every attach after the first will
		// require the user to manually resize or hit enter.
		dockerCli.ResizeTtyTo(ctx, opts.container, height+1, width+1, false)

		// After the above resizing occurs, the call to MonitorTtySize below will handle resetting back
		// to the actual size.
		if err := dockerCli.MonitorTtySize(ctx, opts.container, false); err != nil {
			logrus.Debugf("Error monitoring TTY size: %s", err)
		}
	}
	if err := dockerCli.HoldHijackedConnection(ctx, c.Config.Tty, in, dockerCli.Out(), dockerCli.Err(), resp); err != nil {
		return err
	}

	if errAttach != nil {
		return errAttach
	}

	_, status, err := getExitCode(dockerCli, ctx, opts.container)
	if err != nil {
		return err
	}
	if status != 0 {
		return cli.StatusError{StatusCode: status}
	}

	return nil
}
