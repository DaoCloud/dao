package cli

// Command is the struct containing the command name and description
type Command struct {
	Name        string
	Description string
}

// DockerCommandUsage lists the top level docker commands and their short usage
var DockerCommandUsage = []Command{
	{"exec", "在运行容器中运行指定命令"},
	{"info", "显示Docker引擎系统级别的信息"},
	{"inspect", "返回容器、镜像或任务的底层想相信信息"},
	{"update", "更新一个或者多个容器的配置信息"},
}

// DockerCommands stores all the docker command
var DockerCommands = make(map[string]Command)

func init() {
	for _, cmd := range DockerCommandUsage {
		DockerCommands[cmd.Name] = cmd
	}
}
