package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/cli/cobraadaptor"
	cliflags "github.com/docker/docker/cli/flags"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
)

var (
	commonFlags = cliflags.InitCommonFlags()
	clientFlags = initClientFlags(commonFlags)
	flHelp      = flag.Bool([]string{"h", "-help"}, false, "打印用途")
	flVersion   = flag.Bool([]string{"v", "-version"}, false, "打印版本信息并退出")
)

func main() {
	// Set terminal emulation based on platform as required.
	stdin, stdout, stderr := term.StdStreams()

	logrus.SetOutput(stderr)

	flag.Merge(flag.CommandLine, clientFlags.FlagSet, commonFlags.FlagSet)

	cobraAdaptor := cobraadaptor.NewCobraAdaptor(clientFlags)

	flag.Usage = func() {
		fmt.Fprint(stdout, "用途: docker [OPTIONS] COMMAND [arg...]\n       docker [ --help | -v | --version ]\n\n")
		fmt.Fprint(stdout, "一个为容器而生的运行时管理引擎.\n\n选项:\n")

		flag.CommandLine.SetOutput(stdout)
		flag.PrintDefaults()

		help := "\n命令:\n"

		dockerCommands := append(cli.DockerCommandUsage, cobraAdaptor.Usage()...)
		for _, cmd := range sortCommands(dockerCommands) {
			help += fmt.Sprintf("    %-10.10s%s\n", cmd.Name, cmd.Description)
		}

		help += "\n运行 'docker COMMAND --help' 来获取命令的更多详细信息."
		fmt.Fprintf(stdout, "%s\n", help)
	}

	flag.Parse()

	if *flVersion {
		showVersion()
		return
	}

	if *flHelp {
		// if global flag --help is present, regardless of what other options and commands there are,
		// just print the usage.
		flag.Usage()
		return
	}

	clientCli := client.NewDockerCli(stdin, stdout, stderr, clientFlags)

	c := cli.New(clientCli, NewDaemonProxy(), cobraAdaptor)
	if err := c.Run(flag.Args()...); err != nil {
		if sterr, ok := err.(cli.StatusError); ok {
			if sterr.Status != "" {
				fmt.Fprintln(stderr, sterr.Status)
			}
			// StatusError should only be used for errors, and all errors should
			// have a non-zero exit status, so never exit with 0
			if sterr.StatusCode == 0 {
				os.Exit(1)
			}
			os.Exit(sterr.StatusCode)
		}
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Docker引擎版本 %s, 构建 %s, 试验版\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Docker引擎版本 %s, 构建 %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}

func initClientFlags(commonFlags *cliflags.CommonFlags) *cliflags.ClientFlags {
	clientFlags := &cliflags.ClientFlags{FlagSet: new(flag.FlagSet), Common: commonFlags}
	client := clientFlags.FlagSet
	client.StringVar(&clientFlags.ConfigDir, []string{"-config"}, cliconfig.ConfigDir(), "客户端配置文件路径")

	clientFlags.PostParse = func() {
		clientFlags.Common.PostParse()

		if clientFlags.ConfigDir != "" {
			cliconfig.SetConfigDir(clientFlags.ConfigDir)
		}

		if clientFlags.Common.TrustKey == "" {
			clientFlags.Common.TrustKey = filepath.Join(cliconfig.ConfigDir(), cliflags.DefaultTrustKeyFile)
		}

		if clientFlags.Common.Debug {
			utils.EnableDebug()
		}
	}
	return clientFlags
}
