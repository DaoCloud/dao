package client

import (
	"errors"
	"io"
	"os"

	Cli "github.com/docker/docker/cli"
	flag "github.com/docker/docker/pkg/mflag"
)

// CmdExport exports a filesystem as a tar archive.
//
// The tar archive is streamed to STDOUT by default or written to a file.
//
// Usage: docker export [OPTIONS] CONTAINER
func (cli *DockerCli) CmdExport(args ...string) error {
	cmd := Cli.Subcmd("export", []string{"CONTAINER"}, Cli.DockerCommands["export"].Description, true)
	outfile := cmd.String([]string{"o", "-output"}, "", "写入一个本地文件，而不是标准输出STDOUT")
	cmd.Require(flag.Exact, 1)

	cmd.ParseFlags(args, true)

	var (
		output = cli.out
		err    error
	)
	if *outfile != "" {
		output, err = os.Create(*outfile)
		if err != nil {
			return err
		}
	} else if cli.isTerminalOut {
		return errors.New("终端拒绝保存导出内容，请使用 -o 参数或者重定向。")
	}

	responseBody, err := cli.client.ContainerExport(cmd.Arg(0))
	if err != nil {
		return err
	}
	defer responseBody.Close()

	_, err = io.Copy(output, responseBody)
	return err
}
