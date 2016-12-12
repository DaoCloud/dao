package client

import (
	"fmt"
	"io"

	"github.com/Sirupsen/logrus"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types"
)

// CmdAttach attaches to a running container.
//
// Usage: docker attach [OPTIONS] CONTAINER
func (cli *DockerCli) CmdAttach(args ...string) error {
	cmd := Cli.Subcmd("attach", []string{"CONTAINER"}, Cli.DockerCommands["attach"].Description,true)
	noStdin := cmd.Bool([]string{"-no-stdin"}, false, "不附加标准输入")
	proxy := cmd.Bool([]string{"-sig-proxy"}, true, "代理所有接受到的信号至进程")
	detachKeys := cmd.String([]string{"-detach-keys"}, "", "覆盖从一个容器停止附加的输入键顺序")

	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	c, err := cli.client.ContainerInspect(cmd.Arg(0))
	if err != nil {
		return err
	}

	if !c.State.Running {
		return fmt.Errorf("您不能附加到一个停止的容器，请先启动此容器。")
	}

	if c.State.Paused {
		return fmt.Errorf("您不能附加到一个暂停的容器中，请先启动此容器")
	}

	if err := cli.CheckTtyInput(!*noStdin, c.Config.Tty); err != nil {
		return err
	}

	if c.Config.Tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(cmd.Arg(0), false); err != nil {
			logrus.Debugf("监视终端大小出错: %s", err)
		}
	}

	if *detachKeys != "" {
		cli.configFile.DetachKeys = *detachKeys
	}

	options := types.ContainerAttachOptions{
		ContainerID: cmd.Arg(0),
		Stream:      true,
		Stdin:       !*noStdin && c.Config.OpenStdin,
		Stdout:      true,
		Stderr:      true,
		DetachKeys:  cli.configFile.DetachKeys,
	}

	var in io.ReadCloser
	if options.Stdin {
		in = cli.in
	}

	if *proxy && !c.Config.Tty {
		sigc := cli.forwardAllSignals(options.ContainerID)
		defer signal.StopCatch(sigc)
	}

	resp, err := cli.client.ContainerAttach(options)
	if err != nil {
		return err
	}
	defer resp.Close()
	if in != nil && c.Config.Tty {
		if err := cli.setRawTerminal(); err != nil {
			return err
		}
		defer cli.restoreTerminal(in)
	}

	if err := cli.holdHijackedConnection(c.Config.Tty, in, cli.out, cli.err, resp); err != nil {
		return err
	}

	_, status, err := getExitCode(cli, options.ContainerID)
	if err != nil {
		return err
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}
