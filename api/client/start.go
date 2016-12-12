package client

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/engine-api/types"
)

func (cli *DockerCli) forwardAllSignals(cid string) chan os.Signal {
	sigc := make(chan os.Signal, 128)
	signal.CatchAll(sigc)
	go func() {
		for s := range sigc {
			if s == signal.SIGCHLD {
				continue
			}
			var sig string
			for sigStr, sigN := range signal.SignalMap {
				if sigN == s {
					sig = sigStr
					break
				}
			}
			if sig == "" {
				fmt.Fprintf(cli.err, "丢弃不支持的信号: %v。\n", s)
				continue
			}

			if err := cli.client.ContainerKill(cid, sig); err != nil {
				logrus.Debugf("发送信号出错: %s", err)
			}
		}
	}()
	return sigc
}

// CmdStart starts one or more containers.
//
// Usage: docker start [OPTIONS] CONTAINER [CONTAINER...]
func (cli *DockerCli) CmdStart(args ...string) error {
	cmd := Cli.Subcmd("start", []string{"CONTAINER [CONTAINER...]"}, Cli.DockerCommands["start"].Description, true)
	attach := cmd.Bool([]string{"a", "-attach"}, false, "附件标准输出/标准错误，同时转发信号")
	openStdin := cmd.Bool([]string{"i", "-interactive"}, false, "附加容器的标准输入")
	detachKeys := cmd.String([]string{"-detach-keys"}, "", "覆盖从一个容器退出附加操作时的按键顺序")
	cmd.Require(flag.Min, 1)

	cmd.ParseFlags(args, true)

	if *attach || *openStdin {
		// We're going to attach to a container.
		// 1. Ensure we only have one container.
		if cmd.NArg() > 1 {
			return fmt.Errorf("您不能一次性启动和附加到多个容器")
		}

		// 2. Attach to the container.
		containerID := cmd.Arg(0)
		c, err := cli.client.ContainerInspect(containerID)
		if err != nil {
			return err
		}

		if !c.Config.Tty {
			sigc := cli.forwardAllSignals(containerID)
			defer signal.StopCatch(sigc)
		}

		if *detachKeys != "" {
			cli.configFile.DetachKeys = *detachKeys
		}

		options := types.ContainerAttachOptions{
			ContainerID: containerID,
			Stream:      true,
			Stdin:       *openStdin && c.Config.OpenStdin,
			Stdout:      true,
			Stderr:      true,
			DetachKeys:  cli.configFile.DetachKeys,
		}

		var in io.ReadCloser
		if options.Stdin {
			in = cli.in
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

		cErr := promise.Go(func() error {
			return cli.holdHijackedConnection(c.Config.Tty, in, cli.out, cli.err, resp)
		})

		// 3. Start the container.
		if err := cli.client.ContainerStart(containerID); err != nil {
			return err
		}

		// 4. Wait for attachment to break.
		if c.Config.Tty && cli.isTerminalOut {
			if err := cli.monitorTtySize(containerID, false); err != nil {
				fmt.Fprintf(cli.err, "监视终端大小出错: %s\n", err)
			}
		}
		if attchErr := <-cErr; attchErr != nil {
			return attchErr
		}
		_, status, err := getExitCode(cli, containerID)
		if err != nil {
			return err
		}
		if status != 0 {
			return Cli.StatusError{StatusCode: status}
		}
	} else {
		// We're not going to attach to anything.
		// Start as many containers as we want.
		return cli.startContainersWithoutAttachments(cmd.Args())
	}

	return nil
}

func (cli *DockerCli) startContainersWithoutAttachments(containerIDs []string) error {
	var failedContainers []string
	for _, containerID := range containerIDs {
		if err := cli.client.ContainerStart(containerID); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			failedContainers = append(failedContainers, containerID)
		} else {
			fmt.Fprintf(cli.out, "%s\n", containerID)
		}
	}

	if len(failedContainers) > 0 {
		return fmt.Errorf("错误: 未能启动容器: %v", strings.Join(failedContainers, ", "))
	}
	return nil
}
