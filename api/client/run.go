package client

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
	Cli "github.com/docker/docker/cli"
	derr "github.com/docker/docker/errors"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/signal"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/libnetwork/resolvconf/dns"
)

func (cid *cidFile) Close() error {
	cid.file.Close()

	if !cid.written {
		if err := os.Remove(cid.path); err != nil {
			return fmt.Errorf("删除容器ID文件失败 '%s': %s \n", cid.path, err)
		}
	}

	return nil
}

func (cid *cidFile) Write(id string) error {
	if _, err := cid.file.Write([]byte(id)); err != nil {
		return fmt.Errorf("将容器ID写入文件失败: %s", err)
	}
	cid.written = true
	return nil
}

// if container start fails with 'command not found' error, return 127
// if container start fails with 'command cannot be invoked' error, return 126
// return 125 for generic docker daemon failures
func runStartContainerErr(err error) error {
	trimmedErr := strings.Trim(err.Error(), "Error response from daemon: ")
	statusError := Cli.StatusError{}
	derrCmdNotFound := derr.ErrorCodeCmdNotFound.Message()
	derrCouldNotInvoke := derr.ErrorCodeCmdCouldNotBeInvoked.Message()
	derrNoSuchImage := derr.ErrorCodeNoSuchImageHash.Message()
	derrNoSuchImageTag := derr.ErrorCodeNoSuchImageTag.Message()
	switch trimmedErr {
	case derrCmdNotFound:
		statusError = Cli.StatusError{StatusCode: 127}
	case derrCouldNotInvoke:
		statusError = Cli.StatusError{StatusCode: 126}
	case derrNoSuchImage, derrNoSuchImageTag:
		statusError = Cli.StatusError{StatusCode: 125}
	default:
		statusError = Cli.StatusError{StatusCode: 125}
	}
	return statusError
}

// CmdRun runs a command in a new container.
//
// Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]
func (cli *DockerCli) CmdRun(args ...string) error {
	cmd := Cli.Subcmd("run", []string{"IMAGE [COMMAND] [ARG...]"}, Cli.DockerCommands["run"].Description, true)
	addTrustedFlags(cmd, true)

	// These are flags not stored in Config/HostConfig
	var (
		flAutoRemove = cmd.Bool([]string{"-rm"}, false, "当容器退出时自动删除容器")
		flDetach     = cmd.Bool([]string{"d", "-detach"}, false, "在后台运行容器并打印容器ID")
		flSigProxy   = cmd.Bool([]string{"-sig-proxy"}, true, "代理指定的信号到容器运行进程")
		flName       = cmd.String([]string{"-name"}, "", "为容器赋予一个名称")
		flDetachKeys = cmd.String([]string{"-detach-keys"}, "", "覆盖从容器停止附加时的按键值顺序")
		flAttach     *opts.ListOpts

		ErrConflictAttachDetach               = fmt.Errorf("选项冲突: -a and -d")
		ErrConflictRestartPolicyAndAutoRemove = fmt.Errorf("选项冲突: --restart and --rm")
		ErrConflictDetachAutoRemove           = fmt.Errorf("选项冲突: --rm and -d")
	)

	config, hostConfig, networkingConfig, cmd, err := runconfigopts.Parse(cmd, args)

	// just in case the Parse does not exit
	if err != nil {
		cmd.ReportError(err.Error(), true)
		os.Exit(125)
	}

	if hostConfig.OomKillDisable != nil && *hostConfig.OomKillDisable && hostConfig.Memory == 0 {
		fmt.Fprintf(cli.err, "警告: 在容器上禁用OOM killer时没有设定 '-m/--memory' 限制将带来危险。\n")
	}

	if len(hostConfig.DNS) > 0 {
		// check the DNS settings passed via --dns against
		// localhost regexp to warn if they are trying to
		// set a DNS to a localhost address
		for _, dnsIP := range hostConfig.DNS {
			if dns.IsLocalhost(dnsIP) {
				fmt.Fprintf(cli.err, "警告: 本地的DNS设定(--dns=%s) 在容器内有可能失效。\n", dnsIP)
				break
			}
		}
	}
	if config.Image == "" {
		cmd.Usage()
		return nil
	}

	config.ArgsEscaped = false

	if !*flDetach {
		if err := cli.CheckTtyInput(config.AttachStdin, config.Tty); err != nil {
			return err
		}
	} else {
		if fl := cmd.Lookup("-attach"); fl != nil {
			flAttach = fl.Value.(*opts.ListOpts)
			if flAttach.Len() != 0 {
				return ErrConflictAttachDetach
			}
		}
		if *flAutoRemove {
			return ErrConflictDetachAutoRemove
		}

		config.AttachStdin = false
		config.AttachStdout = false
		config.AttachStderr = false
		config.StdinOnce = false
	}

	// Disable flSigProxy when in TTY mode
	sigProxy := *flSigProxy
	if config.Tty {
		sigProxy = false
	}

	// Telling the Windows daemon the initial size of the tty during start makes
	// a far better user experience rather than relying on subsequent resizes
	// to cause things to catch up.
	if runtime.GOOS == "windows" {
		hostConfig.ConsoleSize[0], hostConfig.ConsoleSize[1] = cli.getTtySize()
	}

	createResponse, err := cli.createContainer(config, hostConfig, networkingConfig, hostConfig.ContainerIDFile, *flName)
	if err != nil {
		cmd.ReportError(err.Error(), true)
		return runStartContainerErr(err)
	}
	if sigProxy {
		sigc := cli.forwardAllSignals(createResponse.ID)
		defer signal.StopCatch(sigc)
	}
	var (
		waitDisplayID chan struct{}
		errCh         chan error
	)
	if !config.AttachStdout && !config.AttachStderr {
		// Make this asynchronous to allow the client to write to stdin before having to read the ID
		waitDisplayID = make(chan struct{})
		go func() {
			defer close(waitDisplayID)
			fmt.Fprintf(cli.out, "%s\n", createResponse.ID)
		}()
	}
	if *flAutoRemove && (hostConfig.RestartPolicy.IsAlways() || hostConfig.RestartPolicy.IsOnFailure()) {
		return ErrConflictRestartPolicyAndAutoRemove
	}

	if config.AttachStdin || config.AttachStdout || config.AttachStderr {
		var (
			out, stderr io.Writer
			in          io.ReadCloser
		)
		if config.AttachStdin {
			in = cli.in
		}
		if config.AttachStdout {
			out = cli.out
		}
		if config.AttachStderr {
			if config.Tty {
				stderr = cli.out
			} else {
				stderr = cli.err
			}
		}

		if *flDetachKeys != "" {
			cli.configFile.DetachKeys = *flDetachKeys
		}

		options := types.ContainerAttachOptions{
			ContainerID: createResponse.ID,
			Stream:      true,
			Stdin:       config.AttachStdin,
			Stdout:      config.AttachStdout,
			Stderr:      config.AttachStderr,
			DetachKeys:  cli.configFile.DetachKeys,
		}

		resp, err := cli.client.ContainerAttach(options)
		if err != nil {
			return err
		}
		if in != nil && config.Tty {
			if err := cli.setRawTerminal(); err != nil {
				return err
			}
			defer cli.restoreTerminal(in)
		}
		errCh = promise.Go(func() error {
			return cli.holdHijackedConnection(config.Tty, in, out, stderr, resp)
		})
	}

	defer func() {
		if *flAutoRemove {
			options := types.ContainerRemoveOptions{
				ContainerID:   createResponse.ID,
				RemoveVolumes: true,
			}
			if err := cli.client.ContainerRemove(options); err != nil {
				fmt.Fprintf(cli.err, "删除容器出错: %s\n", err)
			}
		}
	}()

	//start the container
	if err := cli.client.ContainerStart(createResponse.ID); err != nil {
		cmd.ReportError(err.Error(), false)
		return runStartContainerErr(err)
	}

	if (config.AttachStdin || config.AttachStdout || config.AttachStderr) && config.Tty && cli.isTerminalOut {
		if err := cli.monitorTtySize(createResponse.ID, false); err != nil {
			fmt.Fprintf(cli.err, "监视终端大小出错: %s\n", err)
		}
	}

	if errCh != nil {
		if err := <-errCh; err != nil {
			logrus.Debugf("Error hijack: %s", err)
			return err
		}
	}

	// Detached mode: wait for the id to be displayed and return.
	if !config.AttachStdout && !config.AttachStderr {
		// Detached mode
		<-waitDisplayID
		return nil
	}

	var status int

	// Attached mode
	if *flAutoRemove {
		// Autoremove: wait for the container to finish, retrieve
		// the exit code and remove the container
		if status, err = cli.client.ContainerWait(createResponse.ID); err != nil {
			return runStartContainerErr(err)
		}
		if _, status, err = getExitCode(cli, createResponse.ID); err != nil {
			return err
		}
	} else {
		// No Autoremove: Simply retrieve the exit code
		if !config.Tty {
			// In non-TTY mode, we can't detach, so we must wait for container exit
			if status, err = cli.client.ContainerWait(createResponse.ID); err != nil {
				return err
			}
		} else {
			// In TTY mode, there is a race: if the process dies too slowly, the state could
			// be updated after the getExitCode call and result in the wrong exit code being reported
			if _, status, err = getExitCode(cli, createResponse.ID); err != nil {
				return err
			}
		}
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}
