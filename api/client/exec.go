package client

import (
	"fmt"
	"io"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/engine-api/types"
)

// CmdExec runs a command in a running container.
//
// Usage: docker exec [OPTIONS] CONTAINER COMMAND [ARG...]
func (cli *DockerCli) CmdExec(args ...string) error {
	cmd := Cli.Subcmd("exec", []string{"[OPTIONS] CONTAINER COMMAND [ARG...]"}, Cli.DockerCommands["exec"].Description, true)
	detachKeys := cmd.String([]string{"-detach-keys"}, "", "覆盖从容器停止附加的退出键顺序")

	execConfig, err := ParseExec(cmd, args)
	container := cmd.Arg(0)
	// just in case the ParseExec does not exit
	if container == "" || err != nil {
		return Cli.StatusError{StatusCode: 1}
	}

	if *detachKeys != "" {
		cli.configFile.DetachKeys = *detachKeys
	}

	// Send client escape keys
	execConfig.DetachKeys = cli.configFile.DetachKeys

	ctx := context.Background()

	response, err := cli.client.ContainerExecCreate(ctx, container, *execConfig)
	if err != nil {
		return err
	}

	execID := response.ID
	if execID == "" {
		fmt.Fprintf(cli.out, "exec ID为空")
		return nil
	}

	//Temp struct for execStart so that we don't need to transfer all the execConfig
	if !execConfig.Detach {
		if err := cli.CheckTtyInput(execConfig.AttachStdin, execConfig.Tty); err != nil {
			return err
		}
	} else {
		execStartCheck := types.ExecStartCheck{
			Detach: execConfig.Detach,
			Tty:    execConfig.Tty,
		}

		if err := cli.client.ContainerExecStart(ctx, execID, execStartCheck); err != nil {
			return err
		}
		// For now don't print this - wait for when we support exec wait()
		// fmt.Fprintf(cli.out, "%s\n", execID)
		return nil
	}

	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)

	if execConfig.AttachStdin {
		in = cli.in
	}
	if execConfig.AttachStdout {
		out = cli.out
	}
	if execConfig.AttachStderr {
		if execConfig.Tty {
			stderr = cli.out
		} else {
			stderr = cli.err
		}
	}

	resp, err := cli.client.ContainerExecAttach(ctx, execID, *execConfig)
	if err != nil {
		return err
	}
	defer resp.Close()
	errCh = promise.Go(func() error {
		return cli.HoldHijackedConnection(ctx, execConfig.Tty, in, out, stderr, resp)
	})

	if execConfig.Tty && cli.isTerminalIn {
		if err := cli.MonitorTtySize(ctx, execID, true); err != nil {
			fmt.Fprintf(cli.err, "Error monitoring TTY size: %s\n", err)
		}
	}

	if err := <-errCh; err != nil {
		logrus.Debugf("Error hijack: %s", err)
		return err
	}

	var status int
	if _, status, err = cli.getExecExitCode(ctx, execID); err != nil {
		return err
	}

	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}

	return nil
}

// ParseExec parses the specified args for the specified command and generates
// an ExecConfig from it.
// If the minimal number of specified args is not right or if specified args are
// not valid, it will return an error.
func ParseExec(cmd *flag.FlagSet, args []string) (*types.ExecConfig, error) {
	var (
		flStdin      = cmd.Bool([]string{"i", "-interactive"}, false, "即使容器没有被附加标准输出，标准错误，也爆出输出输入的畅通")
		flTty        = cmd.Bool([]string{"t", "-tty"}, false, "分配一个伪终端")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "后台模式: 在后台运行用户指定的命令")
		flUser       = cmd.String([]string{"u", "-user"}, "", "用户名或用户名ID (格式: <用户名|用户名ID>[:<组|组ID>])")
		flPrivileged = cmd.Bool([]string{"-privileged"}, false, "为运行命令授予格外的特权")
		execCmd      []string
	)
	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return nil, err
	}
	parsedArgs := cmd.Args()
	execCmd = parsedArgs[1:]

	execConfig := &types.ExecConfig{
		User:       *flUser,
		Privileged: *flPrivileged,
		Tty:        *flTty,
		Cmd:        execCmd,
		Detach:     *flDetach,
	}

	// If -d is not set, attach to everything by default
	if !*flDetach {
		execConfig.AttachStdout = true
		execConfig.AttachStderr = true
		if *flStdin {
			execConfig.AttachStdin = true
		}
	}

	return execConfig, nil
}
