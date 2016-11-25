package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/dockerversion"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/reexec"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/docker/utils"
)

var (
	daemonCli = NewDaemonCli()
	flHelp    = flag.Bool([]string{"h", "-help"}, false, "打印用途")
	flVersion = flag.Bool([]string{"v", "-version"}, false, "输出版本信息并退出")
)

func main() {
	if reexec.Init() {
		return
	}

	// Set terminal emulation based on platform as required.
	_, stdout, stderr := term.StdStreams()

	logrus.SetOutput(stderr)

	flag.Merge(flag.CommandLine, daemonCli.commonFlags.FlagSet)

	flag.Usage = func() {
		fmt.Fprint(stdout, "用途: dockerd [OPTIONS]\n\n")
		fmt.Fprint(stdout, "一个为容器而生的运行时管理引擎.\n\n选项:\n")

		flag.CommandLine.SetOutput(stdout)
		flag.PrintDefaults()
	}
	flag.CommandLine.ShortUsage = func() {
		fmt.Fprint(stderr, "\n用途:\tdockerd [OPTIONS]\n")
	}

	if err := flag.CommandLine.ParseFlags(os.Args[1:], false); err != nil {
		os.Exit(1)
	}

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

	// On Windows, this may be launching as a service or with an option to
	// register the service.
	stop, err := initService()
	if err != nil {
		logrus.Fatal(err)
	}

	if !stop {
		err = daemonCli.start()
		notifyShutdown(err)
		if err != nil {
			logrus.Fatal(err)
		}
	}
}

func showVersion() {
	if utils.ExperimentalBuild() {
		fmt.Printf("Docker引擎版本 %s, 构建 %s, 试验版\n", dockerversion.Version, dockerversion.GitCommit)
	} else {
		fmt.Printf("Docker引擎版本 %s, 构建 %s\n", dockerversion.Version, dockerversion.GitCommit)
	}
}
