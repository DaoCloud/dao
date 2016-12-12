package network

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/docker/opts"
	runconfigopts "github.com/docker/docker/runconfig/opts"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/network"
	"github.com/spf13/cobra"
)

type createOptions struct {
	name       string
	driver     string
	driverOpts opts.MapOpts
	labels     []string
	internal   bool
	ipv6       bool

	ipamDriver  string
	ipamSubnet  []string
	ipamIPRange []string
	ipamGateway []string
	ipamAux     opts.MapOpts
	ipamOpt     opts.MapOpts
}

func newCreateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := createOptions{
		driverOpts: *opts.NewMapOpts(nil, nil),
		ipamAux:    *opts.NewMapOpts(nil, nil),
		ipamOpt:    *opts.NewMapOpts(nil, nil),
	}

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] NETWORK",
		Short: "创建一个网络",
		Args:  cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			return runCreate(dockerCli, opts)
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&opts.driver, "driver", "d", "bridge", "管理网络所使用的网络驱动")
	flags.VarP(&opts.driverOpts, "opt", "o", "设置指定驱动的选项")
	flags.StringSliceVar(&opts.labels, "label", []string{}, "在一个网络设置元数据")
	flags.BoolVar(&opts.internal, "internal", false, "限制外界对网络的访问能力")
	flags.BoolVar(&opts.ipv6, "ipv6", false, "启用 IPv6 网络")

	flags.StringVar(&opts.ipamDriver, "ipam-driver", "default", "IP地址管理驱动")
	flags.StringSliceVar(&opts.ipamSubnet, "subnet", []string{}, "CIDR格式的子网信息，代表一个网络段")
	flags.StringSliceVar(&opts.ipamIPRange, "ip-range", []string{}, "从一个子网范围内分配容器IP")
	flags.StringSliceVar(&opts.ipamGateway, "gateway", []string{}, "为主子网设置的IPv4或IPv6网关")

	flags.Var(&opts.ipamAux, "aux-address", "被网络驱动使用的辅助IPv4或IPv6地址")
	flags.Var(&opts.ipamOpt, "ipam-opt", "设置IP地址管理器的特定参数")

	return cmd
}

func runCreate(dockerCli *client.DockerCli, opts createOptions) error {
	client := dockerCli.Client()

	ipamCfg, err := consolidateIpam(opts.ipamSubnet, opts.ipamIPRange, opts.ipamGateway, opts.ipamAux.GetAll())
	if err != nil {
		return err
	}

	// Construct network create request body
	nc := types.NetworkCreate{
		Driver:  opts.driver,
		Options: opts.driverOpts.GetAll(),
		IPAM: network.IPAM{
			Driver:  opts.ipamDriver,
			Config:  ipamCfg,
			Options: opts.ipamOpt.GetAll(),
		},
		CheckDuplicate: true,
		Internal:       opts.internal,
		EnableIPv6:     opts.ipv6,
		Labels:         runconfigopts.ConvertKVStringsToMap(opts.labels),
	}

	resp, err := client.NetworkCreate(context.Background(), opts.name, nc)
	if err != nil {
		return err
	}
	fmt.Fprintf(dockerCli.Out(), "%s\n", resp.ID)
	return nil
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
				return nil, fmt.Errorf("不能配置在同一个子网 (%s)中配置多范围(%s, %s)", s, r, iData[s].IPRange)
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
				return nil, fmt.Errorf("不能配置在同一个子网 (%s)中配置多个网关(%s, %s)", s, g, iData[s].Gateway)
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
			return false, fmt.Errorf("无效的 CIDR %s : %v", data, err)
		}
	} else {
		ip = net.ParseIP(data)
	}

	return s.Contains(ip), nil
}
