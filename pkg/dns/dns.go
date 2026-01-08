// Package dns 提供 DNS 配置管理功能
// 支持 NetworkManager (RHEL 9.x/10.x)、netplan (Debian 12+/Ubuntu 22+) 和直接修改 resolv.conf
package dns

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
)

// Manager 定义了 DNS 管理的类型
type Manager int

const (
	// ManagerUnknown 未知的 DNS 管理方式
	ManagerUnknown Manager = iota
	// ManagerNetworkManager 使用 NetworkManager 管理 DNS
	ManagerNetworkManager
	// ManagerNetplan 使用 netplan 管理 DNS
	ManagerNetplan
	// ManagerResolvConf 直接修改 /etc/resolv.conf
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
	// 检查 NetworkManager 是否正在运行
	if isNetworkManagerActive() {
		return ManagerNetworkManager
	}

	// 检查 netplan 是否存在且有配置文件
	if isNetplanAvailable() {
		return ManagerNetplan
	}

	// 回退到直接修改 resolv.conf
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
	output, _ := shell.Execf("systemctl is-active NetworkManager")
	return output == "active"
}

// isNetplanAvailable 检查 netplan 是否可用
func isNetplanAvailable() bool {
	// 检查 netplan 命令是否存在
	if _, err := shell.Execf("command -v netplan"); err != nil {
		return false
	}

	// 检查是否有 netplan 配置文件
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
	// 获取当前活动的连接
	connName, err := getActiveNMConnection()
	if err != nil {
		// 如果获取失败，回退到直接修改 resolv.conf
		return setDNSWithResolvConf(dns1, dns2)
	}

	// 构建 DNS 服务器列表
	dnsServers := dns1
	if dns2 != "" {
		dnsServers = dns1 + "," + dns2
	}

	// 使用 nmcli 设置 DNS
	if _, err := shell.Execf("nmcli connection modify %s ipv4.dns %s", connName, dnsServers); err != nil {
		return fmt.Errorf("设置 DNS 失败: %w", err)
	}

	// 设置 DNS 优先级，确保自定义 DNS 优先
	if _, err := shell.Execf("nmcli connection modify %s ipv4.dns-priority -1", connName); err != nil {
		// 非致命错误，继续执行
	}

	// 忽略 DHCP 提供的 DNS
	if _, err := shell.Execf("nmcli connection modify %s ipv4.ignore-auto-dns yes", connName); err != nil {
		// 非致命错误，继续执行
	}

	// 重新激活连接以应用更改
	if _, err := shell.Execf("nmcli connection up %s", connName); err != nil {
		return fmt.Errorf("重新激活网络连接失败: %w", err)
	}

	return nil
}

// getActiveNMConnection 获取当前活动的 NetworkManager 连接名称
func getActiveNMConnection() (string, error) {
	output, err := shell.Execf("nmcli -t -f NAME,DEVICE connection show --active")
	if err != nil {
		return "", err
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 格式: NAME:DEVICE
		parts := strings.SplitN(line, ":", 2)
		if len(parts) >= 2 && parts[1] != "" && parts[1] != "lo" {
			// 返回带引号的连接名称，以处理包含空格的名称
			return escapeShellArg(parts[0]), nil
		}
	}

	return "", fmt.Errorf("未找到活动的网络连接")
}

// escapeShellArg 安全转义 shell 参数
// 使用单引号包裹参数，并对参数中的单引号进行转义
func escapeShellArg(arg string) string {
	// 单引号内的内容不会被 shell 解析，除了单引号本身
	// 对于单引号，需要使用 '\'' 来转义：结束单引号、转义单引号、重新开始单引号
	escaped := strings.ReplaceAll(arg, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

// setDNSWithNetplan 使用 netplan 设置 DNS
func setDNSWithNetplan(dns1, dns2 string) error {
	// 查找 netplan 配置文件
	configPath, err := findNetplanConfig()
	if err != nil {
		// 如果找不到配置文件，回退到直接修改 resolv.conf
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
	if err := io.Write(configPath, newContent, 0600); err != nil {
		return fmt.Errorf("写入 netplan 配置失败: %w", err)
	}

	// 应用 netplan 配置
	if _, err := shell.Execf("netplan apply"); err != nil {
		return fmt.Errorf("应用 netplan 配置失败: %w", err)
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

	return "", fmt.Errorf("未找到 netplan 配置文件")
}

// updateNetplanDNS 更新 netplan 配置中的 DNS
func updateNetplanDNS(content, dns1, dns2 string) (string, error) {
	// netplan 配置是 YAML 格式
	// 我们需要在网络接口下添加或更新 nameservers 配置
	// 由于 YAML 解析复杂，这里使用简单的正则替换策略

	lines := strings.Split(content, "\n")
	var result []string
	var insideEthernets bool
	var insideInterface bool
	var interfaceIndent int
	var dnsAdded bool

	dnsConfig := buildNetplanDNSConfig(dns1, dns2)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// 计算当前行的缩进
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))

		// 检测 ethernets 或 wifis 部分
		if strings.HasPrefix(trimmedLine, "ethernets:") || strings.HasPrefix(trimmedLine, "wifis:") {
			insideEthernets = true
			result = append(result, line)
			continue
		}

		// 如果在 ethernets 部分内
		if insideEthernets {
			// 检测接口名（如 eth0:, ens3: 等）
			if strings.HasSuffix(trimmedLine, ":") && !strings.HasPrefix(trimmedLine, "nameservers:") &&
				!strings.HasPrefix(trimmedLine, "addresses:") && !strings.HasPrefix(trimmedLine, "routes:") &&
				!strings.HasPrefix(trimmedLine, "dhcp4:") && !strings.HasPrefix(trimmedLine, "dhcp6:") {
				insideInterface = true
				interfaceIndent = currentIndent
				dnsAdded = false
				result = append(result, line)
				continue
			}

			// 在接口内部处理 nameservers
			if insideInterface && currentIndent > interfaceIndent {
				// 跳过现有的 nameservers 配置块
				if strings.HasPrefix(trimmedLine, "nameservers:") {
					// 跳过 nameservers 及其子项
					for i+1 < len(lines) {
						nextLine := lines[i+1]
						nextTrimmed := strings.TrimSpace(nextLine)
						nextIndent := len(nextLine) - len(strings.TrimLeft(nextLine, " "))
						if nextIndent > currentIndent && (strings.HasPrefix(nextTrimmed, "addresses:") ||
							strings.HasPrefix(nextTrimmed, "-") || strings.HasPrefix(nextTrimmed, "search:")) {
							i++
							continue
						}
						break
					}
					// 添加新的 DNS 配置
					if !dnsAdded {
						result = append(result, strings.Repeat(" ", currentIndent)+dnsConfig)
						dnsAdded = true
					}
					continue
				}
				result = append(result, line)
				continue
			}

			// 离开当前接口，如果还没添加 DNS 配置，在这里添加
			if insideInterface && currentIndent <= interfaceIndent && !dnsAdded {
				// 在接口块末尾添加 DNS 配置
				result = append(result, strings.Repeat(" ", interfaceIndent+2)+dnsConfig)
				dnsAdded = true
				insideInterface = false
			}

			// 检测是否离开 ethernets 部分
			if currentIndent == 0 && trimmedLine != "" {
				insideEthernets = false
			}
		}

		result = append(result, line)
	}

	// 如果在文件末尾还在接口内且没添加 DNS
	if insideInterface && !dnsAdded {
		result = append(result, strings.Repeat(" ", interfaceIndent+2)+dnsConfig)
	}

	return strings.Join(result, "\n"), nil
}

// netplan YAML 配置的标准缩进级别
const (
	netplanIndentSize     = 2  // 每级缩进的空格数
	netplanAddressIndent  = 10 // addresses 行的缩进空格数（nameservers 下一级）
)

// buildNetplanDNSConfig 构建 netplan DNS 配置
func buildNetplanDNSConfig(dns1, dns2 string) string {
	addressIndent := strings.Repeat(" ", netplanAddressIndent)
	if dns2 != "" {
		return fmt.Sprintf("nameservers:\n%saddresses: [%s, %s]", addressIndent, dns1, dns2)
	}
	return fmt.Sprintf("nameservers:\n%saddresses: [%s]", addressIndent, dns1)
}

// setDNSWithResolvConf 直接修改 /etc/resolv.conf 设置 DNS
func setDNSWithResolvConf(dns1, dns2 string) error {
	var dns string
	dns += "nameserver " + dns1 + "\n"
	if dns2 != "" {
		dns += "nameserver " + dns2 + "\n"
	}

	if err := io.Write("/etc/resolv.conf", dns, 0644); err != nil {
		return fmt.Errorf("写入 /etc/resolv.conf 失败: %w", err)
	}

	return nil
}
