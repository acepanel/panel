// Package dns 提供 DNS 配置管理功能
package dns

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
)

// Manager 定义了 DNS 管理的类型
type Manager int

const (
	ManagerUnknown Manager = iota
	ManagerNetworkManager
	ManagerNetplan
	ManagerResolvConf
)

// String 返回 Manager 的字符串表示
func (m Manager) String() string {
	switch m {
	case ManagerNetworkManager:
		return "NetworkManager"
	case ManagerNetplan:
		return "netplan"
	case ManagerResolvConf:
		return "resolv.conf"
	default:
		return "unknown"
	}
}

// DetectManager 检测当前系统使用的 DNS 管理方式
func DetectManager() Manager {
	if isNetworkManagerActive() {
		return ManagerNetworkManager
	}
	if isNetplanAvailable() {
		return ManagerNetplan
	}
	return ManagerResolvConf
}

// GetDNS 获取当前 DNS 配置
func GetDNS() ([]string, Manager, error) {
	manager := DetectManager()
	dns, err := getDNSFromResolvConf()
	return dns, manager, err
}

// SetDNS 设置 DNS 服务器
func SetDNS(dns1, dns2 string) error {
	manager := DetectManager()

	switch manager {
	case ManagerNetworkManager:
		return setDNSWithNetworkManager(dns1, dns2)
	case ManagerNetplan:
		return setDNSWithNetplan(dns1, dns2)
	default:
		return setDNSWithResolvConf(dns1, dns2)
	}
}

// isNetworkManagerActive 检查 NetworkManager 是否正在运行
func isNetworkManagerActive() bool {
	active, _ := systemctl.Status("NetworkManager")
	return active
}

// isNetplanAvailable 检查 netplan 是否可用
func isNetplanAvailable() bool {
	if _, err := shell.Execf("command -v netplan"); err != nil {
		return false
	}

	configFiles := []string{
		"/etc/netplan/*.yaml",
		"/etc/netplan/*.yml",
	}
	for _, pattern := range configFiles {
		files, _ := filepath.Glob(pattern)
		if len(files) > 0 {
			return true
		}
	}

	return false
}

// getDNSFromResolvConf 从 /etc/resolv.conf 获取 DNS
func getDNSFromResolvConf() ([]string, error) {
	raw, err := io.Read("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}

	match := regexp.MustCompile(`nameserver\s+(\S+)`).FindAllStringSubmatch(raw, -1)
	dns := make([]string, 0)
	for _, m := range match {
		dns = append(dns, m[1])
	}

	return dns, nil
}

// setDNSWithNetworkManager 使用 NetworkManager 设置 DNS
func setDNSWithNetworkManager(dns1, dns2 string) error {
	// 获取所有活动的连接
	connections, err := getActiveNMConnections()
	if err != nil || len(connections) == 0 {
		// 回退到直接修改 resolv.conf
		return setDNSWithResolvConf(dns1, dns2)
	}

	// 构建 DNS 服务器列表
	dnsServers := dns1
	if dns2 != "" {
		dnsServers = dns1 + "," + dns2
	}

	var lastErr error
	successCount := 0

	// 为所有活动的连接设置 DNS
	for _, conn := range connections {
		connName := conn.name
		// 使用 nmcli 设置 DNS
		if _, err = shell.Execf("nmcli connection modify %s ipv4.dns %s", connName, dnsServers); err != nil {
			lastErr = fmt.Errorf("failed to set DNS for connection %s: %w", connName, err)
			continue
		}
		// 设置 DNS 优先级，确保自定义 DNS 优先
		_, _ = shell.Execf("nmcli connection modify %s ipv4.dns-priority -1", connName)
		// 忽略 DHCP 提供的 DNS
		_, _ = shell.Execf("nmcli connection modify %s ipv4.ignore-auto-dns yes", connName)
		// 重新激活连接以应用更改
		if _, err = shell.Execf("nmcli connection up %s", connName); err != nil {
			lastErr = fmt.Errorf("failed to reactivate connection %s: %w", connName, err)
			continue
		}
		successCount++
	}

	// 只要有一个连接成功设置就算成功
	if successCount == 0 && lastErr != nil {
		return lastErr
	}

	return nil
}

// nmConnection NetworkManager 连接信息
type nmConnection struct {
	name   string // 连接名称（带引号处理空格）
	device string // 设备名
}

// getActiveNMConnections 获取所有活动的 NetworkManager 连接
func getActiveNMConnections() ([]nmConnection, error) {
	output, err := shell.Execf("nmcli -t -f NAME,DEVICE connection show --active")
	if err != nil {
		return nil, err
	}

	var connections []nmConnection
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 格式: NAME:DEVICE
		parts := strings.SplitN(line, ":", 2)
		if len(parts) >= 2 && parts[1] != "" && isValidNetworkInterface(parts[1]) {
			// 返回带引号的连接名称，以处理包含空格的名称
			quotedName := "'" + strings.ReplaceAll(parts[0], "'", "'\"'\"'") + "'"
			connections = append(connections, nmConnection{
				name:   quotedName,
				device: parts[1],
			})
		}
	}

	if len(connections) == 0 {
		return nil, fmt.Errorf("no active NetworkManager connections found")
	}

	return connections, nil
}

// setDNSWithNetplan 使用 netplan 设置 DNS
func setDNSWithNetplan(dns1, dns2 string) error {
	// 查找 netplan 配置文件
	configPath, err := findNetplanConfig()
	if err != nil {
		// 回退到直接修改 resolv.conf
		return setDNSWithResolvConf(dns1, dns2)
	}

	// 读取现有配置
	content, err := io.Read(configPath)
	if err != nil {
		return setDNSWithResolvConf(dns1, dns2)
	}
	// 更新 DNS 配置
	newContent, err := updateNetplanDNS(content, dns1, dns2)
	if err != nil {
		return setDNSWithResolvConf(dns1, dns2)
	}
	// 写入配置文件
	if err = io.Write(configPath, newContent, 0600); err != nil {
		return fmt.Errorf("failed to write netplan config: %w", err)
	}
	// 应用 netplan 配置
	if _, err = shell.Execf("netplan apply"); err != nil {
		return fmt.Errorf("failed to apply netplan config: %w", err)
	}

	return nil
}

// findNetplanConfig 查找 netplan 配置文件
func findNetplanConfig() (string, error) {
	patterns := []string{
		"/etc/netplan/*.yaml",
		"/etc/netplan/*.yml",
	}

	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(files) > 0 {
			// netplan 按文件名字母顺序处理配置文件
			// 返回最后一个文件，因为它的配置会覆盖之前的配置
			return files[len(files)-1], nil
		}
	}

	return "", fmt.Errorf("failed to find netplan config file")
}

// netplanConfig netplan 配置结构
type netplanConfig struct {
	Network netplanNetwork `yaml:"network"`
}

// netplanNetwork 网络配置
type netplanNetwork struct {
	Version   int                          `yaml:"version,omitempty"`
	Renderer  string                       `yaml:"renderer,omitempty"`
	Ethernets map[string]*netplanInterface `yaml:"ethernets,omitempty"`
	Wifis     map[string]*netplanInterface `yaml:"wifis,omitempty"`
	Bonds     map[string]*netplanInterface `yaml:"bonds,omitempty"`
	Bridges   map[string]*netplanInterface `yaml:"bridges,omitempty"`
	Vlans     map[string]*netplanInterface `yaml:"vlans,omitempty"`
}

// netplanInterface 网络接口配置
type netplanInterface struct {
	DHCP4       any                  `yaml:"dhcp4,omitempty"`
	DHCP6       any                  `yaml:"dhcp6,omitempty"`
	Addresses   []string             `yaml:"addresses,omitempty"`
	Gateway4    string               `yaml:"gateway4,omitempty"`
	Gateway6    string               `yaml:"gateway6,omitempty"`
	Routes      []netplanRoute       `yaml:"routes,omitempty"`
	Nameservers *netplanNameservers  `yaml:"nameservers,omitempty"`
	MTU         int                  `yaml:"mtu,omitempty"`
	MACAddress  string               `yaml:"macaddress,omitempty"`
	Optional    any                  `yaml:"optional,omitempty"`
	Match       *netplanMatch        `yaml:"match,omitempty"`
	SetName     string               `yaml:"set-name,omitempty"`
	Interfaces  []string             `yaml:"interfaces,omitempty"`
	Parameters  map[string]any       `yaml:"parameters,omitempty"`
	ID          int                  `yaml:"id,omitempty"`
	Link        string               `yaml:"link,omitempty"`
	Extra       map[string]yaml.Node `yaml:",inline"` // 保留未知字段
}

// netplanRoute 路由配置
type netplanRoute struct {
	To     string `yaml:"to,omitempty"`
	Via    string `yaml:"via,omitempty"`
	Metric int    `yaml:"metric,omitempty"`
}

// netplanNameservers DNS 配置
type netplanNameservers struct {
	Addresses []string `yaml:"addresses,omitempty"`
	Search    []string `yaml:"search,omitempty"`
}

// netplanMatch 网卡匹配规则
type netplanMatch struct {
	MACAddress string `yaml:"macaddress,omitempty"`
	Driver     string `yaml:"driver,omitempty"`
}

// updateNetplanDNS 更新 netplan 配置中的 DNS
func updateNetplanDNS(content, dns1, dns2 string) (string, error) {
	var config netplanConfig
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return "", fmt.Errorf("failed to parse netplan config: %w", err)
	}

	// 构建新的 DNS 地址列表
	dnsAddresses := []string{dns1}
	if dns2 != "" {
		dnsAddresses = append(dnsAddresses, dns2)
	}

	// 更新所有网络接口的 DNS 配置
	updated := false

	// 更新 ethernets
	for _, iface := range config.Network.Ethernets {
		if iface != nil {
			if iface.Nameservers == nil {
				iface.Nameservers = &netplanNameservers{}
			}
			iface.Nameservers.Addresses = dnsAddresses
			updated = true
		}
	}

	// 更新 wifis
	for _, iface := range config.Network.Wifis {
		if iface != nil {
			if iface.Nameservers == nil {
				iface.Nameservers = &netplanNameservers{}
			}
			iface.Nameservers.Addresses = dnsAddresses
			updated = true
		}
	}

	// 更新 bonds
	for _, iface := range config.Network.Bonds {
		if iface != nil {
			if iface.Nameservers == nil {
				iface.Nameservers = &netplanNameservers{}
			}
			iface.Nameservers.Addresses = dnsAddresses
			updated = true
		}
	}

	// 更新 bridges
	for _, iface := range config.Network.Bridges {
		if iface != nil {
			if iface.Nameservers == nil {
				iface.Nameservers = &netplanNameservers{}
			}
			iface.Nameservers.Addresses = dnsAddresses
			updated = true
		}
	}

	// 更新 vlans
	for _, iface := range config.Network.Vlans {
		if iface != nil {
			if iface.Nameservers == nil {
				iface.Nameservers = &netplanNameservers{}
			}
			iface.Nameservers.Addresses = dnsAddresses
			updated = true
		}
	}

	// 如果配置中没有任何接口，尝试检测当前活动的网络接口并添加配置
	if !updated {
		activeIface := detectActiveInterface()
		if activeIface == "" {
			return "", fmt.Errorf("no network interface found in config and failed to detect active interface")
		}

		// 创建 ethernets 配置
		if config.Network.Ethernets == nil {
			config.Network.Ethernets = make(map[string]*netplanInterface)
		}

		// 为检测到的接口添加配置
		config.Network.Ethernets[activeIface] = &netplanInterface{
			Nameservers: &netplanNameservers{
				Addresses: dnsAddresses,
			},
		}

		// 设置默认版本
		if config.Network.Version == 0 {
			config.Network.Version = 2
		}

		updated = true
	}

	// 序列化为 YAML
	output, err := yaml.Marshal(&config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal netplan config: %w", err)
	}

	return string(output), nil
}

// detectActiveInterface 检测当前活动的网络接口名称
// 返回第一个非 lo/docker/veth/br- 的活动接口
func detectActiveInterface() string {
	// 尝试获取默认路由的网络接口
	output, err := shell.Execf("ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -n1")
	if err == nil {
		iface := strings.TrimSpace(output)
		if iface != "" && isValidNetworkInterface(iface) {
			return iface
		}
	}

	// 回退：获取所有 UP 状态的接口
	output, err = shell.Execf("ip -o link show up 2>/dev/null | awk -F': ' '{print $2}'")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			iface := strings.TrimSpace(line)
			if isValidNetworkInterface(iface) {
				return iface
			}
		}
	}

	return ""
}

// isValidNetworkInterface 检查接口名是否为有效的物理/外部网络接口
func isValidNetworkInterface(name string) bool {
	if name == "" {
		return false
	}

	// 排除虚拟接口和回环接口
	excludePrefixes := []string{
		"lo",      // 回环
		"docker",  // Docker
		"veth",    // Docker/容器虚拟网卡
		"br-",     // Docker 桥接
		"virbr",   // libvirt 虚拟桥接
		"vnet",    // 虚拟网络
		"tun",     // VPN 隧道
		"tap",     // TAP 设备
		"flannel", // Kubernetes flannel
		"cni",     // Kubernetes CNI
		"cali",    // Calico
	}

	for _, prefix := range excludePrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}

	return true
}

// setDNSWithResolvConf 直接修改 /etc/resolv.conf 设置 DNS
func setDNSWithResolvConf(dns1, dns2 string) error {
	var dns string
	dns += "nameserver " + dns1 + "\n"
	if dns2 != "" {
		dns += "nameserver " + dns2 + "\n"
	}

	if err := io.Write("/etc/resolv.conf", dns, 0644); err != nil {
		return fmt.Errorf("failed to write /etc/resolv.conf: %w", err)
	}

	return nil
}
