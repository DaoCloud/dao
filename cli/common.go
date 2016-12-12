package cli

import (
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/go-connections/tlsconfig"
)

// CommonFlags represents flags that are common to both the client and the daemon.
type CommonFlags struct {
	FlagSet   *flag.FlagSet
	PostParse func()

	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	TrustKey   string
}

// Command is the struct contains command name and description
type Command struct {
	Name        string
	Description string
}

var dockerCommands = []Command{
	{"attach", "进入运行容器内部"},
	{"build", "从一个Dockerfile构建新的镜像"},
	{"commit", "从一个容器的变化部分创建一个新的镜像"},
	{"cp", "在容器和宿主机本地文件系统间拷贝文件"},
	{"create", "创建一个新的容器"},
	{"diff", "查看容器文件系统的变化差异"},
	{"events", "获取Docker引擎的实时事件"},
	{"exec", "在运行容器中运行指定命令"},
	{"export", "以一个压缩包的形式导出一个容器的文件系统"},
	{"history", "显示一个镜像的历史信息"},
	{"images", "罗列所有镜像"},
	{"import", "从一个压缩包中导入内容，从而创建一个文件系统镜像"},
	{"info", "显示Docker引擎系统级别的信息"},
	{"inspect", "返回容器、镜像的底层信息"},
	{"kill", "终止一个运行的容器"},
	{"load", "从一个压缩包或者标准输入加载一个镜像"},
	{"login", "登录一个Docker镜像仓库"},
	{"logout", "登出一个Docker镜像仓库"},
	{"logs", "获取一个容器的运行日志"},
	{"network", "管理Docker网络"},
	{"pause", "停止一个容器内部的所有进程的运行"},
	{"port", "罗列所有端口映射信息或为容器指定的端口映射信息"},
	{"ps", "罗列所有容器"},
	{"pull", "从一个镜像仓库下拉一个镜像"},
	{"push", "上传一个镜像到镜像仓库"},
	{"rename", "重命名一个容器"},
	{"restart", "重启一个容器"},
	{"rm", "删除一个或多个容器"},
	{"rmi", "删除一个多多个镜像"},
	{"run", "在一个新的容器中运行一条命令"},
	{"save", "将一个或多个镜像保存至压缩包"},
	{"search", "在 Docker Hub(Docker官方镜像仓库)中搜索镜像"},
	{"start", "启动一个或多个停止的容器"},
	{"stats", "显示容器资源使用情况的实时流"},
	{"stop", "停止一个运行的容器"},
	{"tag", "为一个镜像指定一个标签"},
	{"top", "显示一个运行容器中运行的所有进程信息"},
	{"unpause", "恢复一个容器中所有被挂起的进程"},
	{"update", "更新容器的资源信息"},
	{"version", "显示Docker的版本信息"},
	{"volume", "管理Docker存储卷"},
	{"wait", "阻塞直到一个容器停止运行，并打印它们的容器退出码"},
}

// DockerCommands stores all the docker command
var DockerCommands = make(map[string]Command)

func init() {
	for _, cmd := range dockerCommands {
		DockerCommands[cmd.Name] = cmd
	}
}
