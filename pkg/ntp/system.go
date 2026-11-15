package ntp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
)

// NTPServiceType 表示系统使用的 NTP 服务类型
type NTPServiceType string

const (
	ntpServiceTimesyncd NTPServiceType = "timesyncd" // systemd-timesyncd (Debian/Ubuntu)
	ntpServiceChrony    NTPServiceType = "chrony"    // chrony (RHEL/CentOS/Rocky)
	ntpServiceUnknown   NTPServiceType = "unknown"   // 未知或不支持
)

// timesyncd 配置文件路径
const timesyncdConfigPath = "/etc/systemd/timesyncd.conf"

// chrony 配置文件路径（按优先级排序）
var chronyConfigPaths = []string{
	"/etc/chrony.conf",
	"/etc/chrony/chrony.conf",
}

// SystemNTPConfig 系统 NTP 配置信息
type SystemNTPConfig struct {
	ServiceType NTPServiceType `json:"service_type"` // 服务类型
	Servers     []string       `json:"servers"`      // NTP 服务器列表
}

// GetSystemNTPConfig 获取系统 NTP 配置
func GetSystemNTPConfig(ctx context.Context) (*SystemNTPConfig, error) {
	serviceType := detectNTPService(ctx)
	config := &SystemNTPConfig{
		ServiceType: serviceType,
		Servers:     []string{},
	}

	switch serviceType {
	case ntpServiceTimesyncd:
		servers, err := getTimesyncdServers()
		if err != nil {
			return config, err
		}
		config.Servers = servers
	case ntpServiceChrony:
		servers, err := getChronyServers()
		if err != nil {
			return config, err
		}
		config.Servers = servers
	case ntpServiceUnknown:
	}

	return config, nil
}

// SetSystemNTPServers 设置系统 NTP 服务器
func SetSystemNTPServers(ctx context.Context, servers []string) error {
	serviceType := detectNTPService(ctx)

	switch serviceType {
	case ntpServiceTimesyncd:
		return setTimesyncdServers(ctx, servers)
	case ntpServiceChrony:
		return setChronyServers(ctx, servers)
	default:
		return errors.New("unsupported NTP service type")
	}
}

// detectNTPService 检测系统使用的 NTP 服务类型
func detectNTPService(ctx context.Context) NTPServiceType {
	// 优先检查 chrony
	if _, err := shell.Execf(ctx, "systemctl is-active chronyd 2>/dev/null"); err == nil {
		return ntpServiceChrony
	}
	if _, err := shell.Execf(ctx, "systemctl is-active chrony 2>/dev/null"); err == nil {
		return ntpServiceChrony
	}

	// 检查 systemd-timesyncd
	if _, err := shell.Execf(ctx, "systemctl is-active systemd-timesyncd 2>/dev/null"); err == nil {
		return ntpServiceTimesyncd
	}

	// 检查配置文件是否存在
	if slices.ContainsFunc(chronyConfigPaths, io.Exists) {
		return ntpServiceChrony
	}
	if io.Exists(timesyncdConfigPath) {
		return ntpServiceTimesyncd
	}

	return ntpServiceUnknown
}

// getTimesyncdServers 获取 systemd-timesyncd 的 NTP 服务器配置
func getTimesyncdServers() ([]string, error) {
	if !io.Exists(timesyncdConfigPath) {
		return []string{}, nil
	}

	content, err := io.Read(timesyncdConfigPath)
	if err != nil {
		return nil, err
	}

	// 解析配置文件，查找 NTP= 行
	var servers []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	ntpRegex := regexp.MustCompile(`^\s*NTP\s*=\s*(.+)$`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if matches := ntpRegex.FindStringSubmatch(line); len(matches) > 1 {
			// NTP 服务器以空格分隔
			for server := range strings.FieldsSeq(matches[1]) {
				if server != "" {
					servers = append(servers, server)
				}
			}
		}
	}

	return servers, nil
}

// setTimesyncdServers 设置 systemd-timesyncd 的 NTP 服务器配置
func setTimesyncdServers(ctx context.Context, servers []string) error {
	var content string
	if io.Exists(timesyncdConfigPath) {
		var err error
		content, err = io.Read(timesyncdConfigPath)
		if err != nil {
			return err
		}
	}

	// 构建新的 NTP 配置行
	ntpLine := "NTP=" + strings.Join(servers, " ")

	// 检查是否已有 [Time] 段和 NTP= 行
	hasTimeSection := strings.Contains(content, "[Time]")
	ntpRegex := regexp.MustCompile(`(?m)^\s*#?\s*NTP\s*=.*$`)

	switch {
	case ntpRegex.MatchString(content):
		// 替换现有的 NTP= 行
		content = ntpRegex.ReplaceAllString(content, ntpLine)
	case hasTimeSection:
		// 在 [Time] 段后添加 NTP= 行
		content = strings.Replace(content, "[Time]", "[Time]\n"+ntpLine, 1)
	default:
		// 添加 [Time] 段和 NTP= 行
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "[Time]\n" + ntpLine + "\n"
	}

	// 写入配置文件
	if err := io.Write(timesyncdConfigPath, content, 0644); err != nil {
		return err
	}

	// 重启 systemd-timesyncd 服务
	_ = systemctl.Restart(ctx, "systemd-timesyncd")

	return nil
}

// getChronyServers 获取 chrony 的 NTP 服务器配置
func getChronyServers() ([]string, error) {
	var configPath string
	for _, path := range chronyConfigPaths {
		if io.Exists(path) {
			configPath = path
			break
		}
	}

	if configPath == "" {
		return []string{}, nil
	}

	content, err := io.Read(configPath)
	if err != nil {
		return nil, err
	}

	// 解析配置文件，查找 server 或 pool 行
	var servers []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	serverRegex := regexp.MustCompile(`^\s*(server|pool)\s+(\S+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if matches := serverRegex.FindStringSubmatch(line); len(matches) > 2 {
			servers = append(servers, matches[2])
		}
	}

	return servers, nil
}

// setChronyServers 设置 chrony 的 NTP 服务器配置
func setChronyServers(ctx context.Context, servers []string) error {
	var configPath string
	for _, path := range chronyConfigPaths {
		if io.Exists(path) {
			configPath = path
			break
		}
	}

	if configPath == "" {
		// 如果配置文件不存在，使用默认路径
		configPath = chronyConfigPaths[0]
	}

	var content string
	if io.Exists(configPath) {
		var err error
		content, err = io.Read(configPath)
		if err != nil {
			return err
		}
	}

	// 移除现有的 server 和 pool 行
	var newLines []string
	scanner := bufio.NewScanner(strings.NewReader(content))
	serverRegex := regexp.MustCompile(`^\s*(server|pool)\s+`)

	for scanner.Scan() {
		line := scanner.Text()
		if !serverRegex.MatchString(line) {
			newLines = append(newLines, line)
		}
	}

	// 在文件开头添加新的 server 行
	serverLines := lo.Map(servers, func(server string, _ int) string {
		return fmt.Sprintf("server %s iburst", server)
	})

	// 组合新内容
	newContent := strings.Join(serverLines, "\n")
	if len(newLines) > 0 {
		newContent += "\n" + strings.Join(newLines, "\n")
	}
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	// 写入配置文件
	if err := io.Write(configPath, newContent, 0644); err != nil {
		return err
	}

	// 重启 chrony 服务，两种服务名只会命中一种
	_ = systemctl.Restart(ctx, "chronyd")
	_ = systemctl.Restart(ctx, "chrony")

	return nil
}
