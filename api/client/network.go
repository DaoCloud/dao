package client

import (
	"fmt"
	"net"
	"strings"
	"text/tabwriter"

	Cli "github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/stringid"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"github.com/docker/engine-api/types/network"
)

// CmdNetwork is the parent subcommand for all network commands
//
// Usage: docker network <COMMAND> [OPTIONS]
func (cli *DockerCli) CmdNetwork(args ...string) error {
	cmd := Cli.Subcmd("network", []string{"COMMAND [OPTIONS]"}, networkUsage(), false)
	cmd.Require(flag.Min, 1)
	err := cmd.ParseFlags(args, true)
	cmd.Usage()
	return err
}

// CmdNetworkCreate creates a new network with a given name
//
// Usage: docker network create [OPTIONS] <NETWORK-NAME>
func (cli *DockerCli) CmdNetworkCreate(args ...string) error {
	//Creates a new network with a name specified by the user
	cmd := Cli.Subcmd("network create", []string{"NETWORK-NAME"}, "创建一个新的网络，并且由用户指定一个网络名称", false)
	flDriver := cmd.String([]string{"d", "-driver"}, "bridge", "管理网络所使用的网络驱动")
	flOpts := opts.NewMapOpts(nil, nil)

	flIpamDriver := cmd.String([]string{"-ipam-driver"}, "default", "IP地址管理驱动")
	flIpamSubnet := opts.NewListOpts(nil)
	flIpamIPRange := opts.NewListOpts(nil)
	flIpamGateway := opts.NewListOpts(nil)
	flIpamAux := opts.NewMapOpts(nil, nil)
	flIpamOpt := opts.NewMapOpts(nil, nil)

	cmd.Var(&flIpamSubnet, []string{"-subnet"}, "CIDR格式的子网信息，代表一个网络段")
	cmd.Var(&flIpamIPRange, []string{"-ip-range"}, "从一个子网范围内分配容器IP")
	cmd.Var(&flIpamGateway, []string{"-gateway"}, "为主子网设置的IPv4或IPv6网关")
	cmd.Var(flIpamAux, []string{"-aux-address"}, "被网络驱动使用的辅助IPv4或IPv6网关")
	cmd.Var(flOpts, []string{"o", "-opt"}, "设置指定驱动的选项")
	cmd.Var(flIpamOpt, []string{"-ipam-opt"}, "设置IP地址管理器的特定参数")

	flInternal := cmd.Bool([]string{"-internal"}, false, "限制外界对网络的访问能力")

	cmd.Require(flag.Exact, 1)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Set the default driver to "" if the user didn't set the value.
	// That way we can know whether it was user input or not.
	driver := *flDriver
	if !cmd.IsSet("-driver") && !cmd.IsSet("d") {
		driver = ""
	}

	ipamCfg, err := consolidateIpam(flIpamSubnet.GetAll(), flIpamIPRange.GetAll(), flIpamGateway.GetAll(), flIpamAux.GetAll())
	if err != nil {
		return err
	}

	// Construct network create request body
	nc := types.NetworkCreate{
		Name:           cmd.Arg(0),
		Driver:         driver,
		IPAM:           network.IPAM{Driver: *flIpamDriver, Config: ipamCfg, Options: flIpamOpt.GetAll()},
		Options:        flOpts.GetAll(),
		CheckDuplicate: true,
		Internal:       *flInternal,
	}

	resp, err := cli.client.NetworkCreate(nc)
	if err != nil {
		return err
	}
	fmt.Fprintf(cli.out, "%s\n", resp.ID)
	return nil
}

// CmdNetworkRm deletes one or more networks
//
// Usage: docker network rm NETWORK-NAME|NETWORK-ID [NETWORK-NAME|NETWORK-ID...]
func (cli *DockerCli) CmdNetworkRm(args ...string) error {
	cmd := Cli.Subcmd("network rm", []string{"NETWORK [NETWORK...]"}, "删除一个或多个网络", false)
	cmd.Require(flag.Min, 1)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	status := 0
	for _, net := range cmd.Args() {
		if err := cli.client.NetworkRemove(net); err != nil {
			fmt.Fprintf(cli.err, "%s\n", err)
			status = 1
			continue
		}
	}
	if status != 0 {
		return Cli.StatusError{StatusCode: status}
	}
	return nil
}

// CmdNetworkConnect connects a container to a network
//
// Usage: docker network connect [OPTIONS] <NETWORK> <CONTAINER>
func (cli *DockerCli) CmdNetworkConnect(args ...string) error {
	cmd := Cli.Subcmd("network connect", []string{"NETWORK CONTAINER"}, "连接一个容器到一个网络", false)
	flIPAddress := cmd.String([]string{"-ip"}, "", "IP 地址")
	flIPv6Address := cmd.String([]string{"-ip6"}, "", "IPv6 地址")
	flLinks := opts.NewListOpts(runconfigopts.ValidateLink)
	cmd.Var(&flLinks, []string{"-link"}, "为另一个容器添加链接")
	flAliases := opts.NewListOpts(nil)
	cmd.Var(&flAliases, []string{"-alias"}, "为容器添加网络范围的别名")
	cmd.Require(flag.Min, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}
	epConfig := &network.EndpointSettings{
		IPAMConfig: &network.EndpointIPAMConfig{
			IPv4Address: *flIPAddress,
			IPv6Address: *flIPv6Address,
		},
		Links:   flLinks.GetAll(),
		Aliases: flAliases.GetAll(),
	}
	return cli.client.NetworkConnect(cmd.Arg(0), cmd.Arg(1), epConfig)
}

// CmdNetworkDisconnect disconnects a container from a network
//
// Usage: docker network disconnect <NETWORK> <CONTAINER>
func (cli *DockerCli) CmdNetworkDisconnect(args ...string) error {
	cmd := Cli.Subcmd("network disconnect", []string{"NETWORK CONTAINER"}, "断开一个容器和一个网络的链接", false)
	force := cmd.Bool([]string{"f", "-force"}, false, "强制容器与网络端口连接")
	cmd.Require(flag.Exact, 2)
	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	return cli.client.NetworkDisconnect(cmd.Arg(0), cmd.Arg(1), *force)
}

// CmdNetworkLs lists all the networks managed by docker daemon
//
// Usage: docker network ls [OPTIONS]
func (cli *DockerCli) CmdNetworkLs(args ...string) error {
	cmd := Cli.Subcmd("network ls", nil, "罗列所有网络", true)
	quiet := cmd.Bool([]string{"q", "-quiet"}, false, "仅显示网络ID")
	noTrunc := cmd.Bool([]string{"-no-trunc"}, false, "不截断命令输出内容")

	flFilter := opts.NewListOpts(nil)
	cmd.Var(&flFilter, []string{"f", "-filter"}, "根据情况提供一些过滤值")

	cmd.Require(flag.Exact, 0)
	err := cmd.ParseFlags(args, true)
	if err != nil {
		return err
	}

	// Consolidate all filter flags, and sanity check them early.
	// They'll get process after get response from server.
	netFilterArgs := filters.NewArgs()
	for _, f := range flFilter.GetAll() {
		if netFilterArgs, err = filters.ParseFlag(f, netFilterArgs); err != nil {
			return err
		}
	}

	options := types.NetworkListOptions{
		Filters: netFilterArgs,
	}

	networkResources, err := cli.client.NetworkList(options)
	if err != nil {
		return err
	}

	wr := tabwriter.NewWriter(cli.out, 20, 1, 3, ' ', 0)

	// unless quiet (-q) is specified, print field titles
	if !*quiet {
		fmt.Fprintln(wr, "网络ID\t名称\t范围")
	}

	for _, networkResource := range networkResources {
		ID := networkResource.ID
		netName := networkResource.Name
		if !*noTrunc {
			ID = stringid.TruncateID(ID)
		}
		if *quiet {
			fmt.Fprintln(wr, ID)
			continue
		}
		driver := networkResource.Driver
		fmt.Fprintf(wr, "%s\t%s\t%s\t",
			ID,
			netName,
			driver)
		fmt.Fprint(wr, "\n")
	}
	wr.Flush()
	return nil
}

// CmdNetworkInspect inspects the network object for more details
//
// Usage: docker network inspect [OPTIONS] <NETWORK> [NETWORK...]
func (cli *DockerCli) CmdNetworkInspect(args ...string) error {
	cmd := Cli.Subcmd("network inspect", []string{"NETWORK [NETWORK...]"}, "显示一个或多个网络的详细信息", false)
	tmplStr := cmd.String([]string{"f", "-format"}, "", "根据指定的Go语言模版格式化命令输出内容")
	cmd.Require(flag.Min, 1)

	if err := cmd.ParseFlags(args, true); err != nil {
		return err
	}

	inspectSearcher := func(name string) (interface{}, []byte, error) {
		i, err := cli.client.NetworkInspect(name)
		return i, nil, err
	}

	return cli.inspectElements(*tmplStr, cmd.Args(), inspectSearcher)
}

// Consolidates the ipam configuration as a group from different related configurations
// user can configure network with multiple non-overlapping subnets and hence it is
// possible to correlate the various related parameters and consolidate them.
// consoidateIpam consolidates subnets, ip-ranges, gateways and auxiliary addresses into
// structured ipam data.
func consolidateIpam(subnets, ranges, gateways []string, auxaddrs map[string]string) ([]network.IPAMConfig, error) {
	if len(subnets) < len(ranges) || len(subnets) < len(gateways) {
		return nil, fmt.Errorf("每一个IP范围或网关必须拥有一个相应的子网地址")
	}
	iData := map[string]*network.IPAMConfig{}

	// Populate non-overlapping subnets into consolidation map
	for _, s := range subnets {
		for k := range iData {
			ok1, err := subnetMatches(s, k)
			if err != nil {
				return nil, err
			}
			ok2, err := subnetMatches(k, s)
			if err != nil {
				return nil, err
			}
			if ok1 || ok2 {
				return nil, fmt.Errorf("多个子网配置信息有重叠的情况，Docker引擎暂不支持")
			}
		}
		iData[s] = &network.IPAMConfig{Subnet: s, AuxAddress: map[string]string{}}
	}

	// Validate and add valid ip ranges
	for _, r := range ranges {
		match := false
		for _, s := range subnets {
			ok, err := subnetMatches(s, r)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if iData[s].IPRange != "" {
				return nil, fmt.Errorf("不能在同一个子网 (%s) 中配置多范围 (%s, %s)", s, r, iData[s].IPRange)
			}
			d := iData[s]
			d.IPRange = r
			match = true
		}
		if !match {
			return nil, fmt.Errorf("没有匹配的子网地址 %s", r)
		}
	}

	// Validate and add valid gateways
	for _, g := range gateways {
		match := false
		for _, s := range subnets {
			ok, err := subnetMatches(s, g)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if iData[s].Gateway != "" {
				return nil, fmt.Errorf("不能在同一个子网 (%s) 中配置多个网关(%s, %s)", s, g, iData[s].Gateway)
			}
			d := iData[s]
			d.Gateway = g
			match = true
		}
		if !match {
			return nil, fmt.Errorf("没有匹配的子网地址 %s", g)
		}
	}

	// Validate and add aux-addresses
	for key, aa := range auxaddrs {
		match := false
		for _, s := range subnets {
			ok, err := subnetMatches(s, aa)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			iData[s].AuxAddress[key] = aa
			match = true
		}
		if !match {
			return nil, fmt.Errorf("没有找到符合辅助网络地址的子网 %s", aa)
		}
	}

	idl := []network.IPAMConfig{}
	for _, v := range iData {
		idl = append(idl, *v)
	}
	return idl, nil
}

func subnetMatches(subnet, data string) (bool, error) {
	var (
		ip net.IP
	)

	_, s, err := net.ParseCIDR(subnet)
	if err != nil {
		return false, fmt.Errorf("无效的子网地址 %s : %v", s, err)
	}

	if strings.Contains(data, "/") {
		ip, _, err = net.ParseCIDR(data)
		if err != nil {
			return false, fmt.Errorf("无效的CIDR %s : %v", data, err)
		}
	} else {
		ip = net.ParseIP(data)
	}

	return s.Contains(ip), nil
}

func networkUsage() string {
	networkCommands := map[string]string{
		"create":     "创建一个网络",
		"connect":    "连接一个容器到一个网络",
		"disconnect": "断开一个容器和一个网络的链接",
		"inspect":    "显示一个或多个网络的详细信息",
		"ls":         "罗列所有网络",
		"rm":         "删除一个网络",
	}

	help := "Commands:\n"

	for cmd, description := range networkCommands {
		help += fmt.Sprintf("  %-25.25s%s\n", cmd, description)
	}

	help += fmt.Sprintf("\n运行 'docker network COMMAND --help' 获取更多关于命令的信息")
	return help
}
