package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
)

const serverConfigPath = "/etc/ssh/sshd_config"

var (
	serverPortRegex         = regexp.MustCompile(`(?m)^Port\s+(\d+)`)
	serverPasswordAuthRegex = regexp.MustCompile(`(?m)^PasswordAuthentication\s+(\S+)`)
	serverPubkeyAuthRegex   = regexp.MustCompile(`(?m)^PubkeyAuthentication\s+(\S+)`)
	serverRootLoginRegex    = regexp.MustCompile(`(?m)^PermitRootLogin\s+(\S+)`)
)

// ServerConfig sshd_config 中由面板管理的选项
type ServerConfig struct {
	Port         int
	PasswordAuth bool
	PubkeyAuth   bool
	RootLogin    string
}

// ReadServerConfig 读取 sshd_config，没有显式配置的选项取默认值
func ReadServerConfig() (ServerConfig, error) {
	content, err := io.Read(serverConfigPath)
	if err != nil {
		return ServerConfig{}, err
	}

	return parseServerConfig(content), nil
}

// SetServerOption 修改 sshd_config 中的选项，被注释掉的同名行一并替换，没有则追加
func SetServerOption(key, value string) error {
	content, err := io.Read(serverConfigPath)
	if err != nil {
		return err
	}

	return io.Write(serverConfigPath, setServerOption(content, key, value), 0600)
}

// ApplyServerPort 使已写入的 SSH 端口配置生效，保留服务和 socket 的启用状态。
func ApplyServerPort(ctx context.Context, service string, port uint) error {
	// 配置已经写入，后续应用不能因 HTTP 连接断开而半途停止。
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	defer cancel()
	if _, err := shell.Exec(ctx, "sshd -t"); err != nil {
		return err
	}

	socket := service + ".socket"
	state, err := readServerSocket(ctx, socket)
	if err != nil {
		return err
	}
	if state["ActiveState"] != "active" && state["UnitFileState"] != "enabled" && state["UnitFileState"] != "enabled-runtime" {
		_, err = shell.Execf(ctx, "systemctl restart '%s'", service)
		return err
	}

	// Ubuntu 的 generator 在 daemon-reload 时将 sshd_config 转成 ListenStream。
	if _, err = shell.Exec(ctx, "systemctl daemon-reload"); err != nil {
		return err
	}
	state, err = readServerSocket(ctx, socket)
	if err != nil {
		return err
	}
	changed, err := serverSocketPortChanged(state["Listen"], port)
	if err != nil {
		return err
	}
	if changed {
		paths := append([]string{state["FragmentPath"]}, strings.Fields(state["DropInPaths"])...)
		if err = updateServerSocketConfigs(paths, port); err != nil {
			return err
		}
		if _, err = shell.Exec(ctx, "systemctl daemon-reload"); err != nil {
			return err
		}
		state, err = readServerSocket(ctx, socket)
		if err != nil {
			return err
		}
		if changed, err = serverSocketPortChanged(state["Listen"], port); err != nil {
			return err
		}
		if changed {
			return errors.New("SSH socket port did not take effect")
		}
	}

	if state["Accept"] != "yes" {
		// Accept=no 的 sshd 持有旧监听 FD，先停止两者，避免 socket 在服务仍活跃时拒绝启动。
		if _, err = shell.Execf(ctx, "systemctl stop '%s' '%s'", socket, service); err != nil {
			return err
		}
	}
	// 由 socket 按需拉起服务；Accept=yes 的常驻服务可能与 socket 互斥。
	_, err = shell.Execf(ctx, "systemctl restart '%s'", socket)
	return err
}

func parseServerConfig(content string) ServerConfig {
	config := ServerConfig{
		Port:         22,
		PasswordAuth: true,
		PubkeyAuth:   true,
		RootLogin:    "yes",
	}
	if m := serverPortRegex.FindStringSubmatch(content); m != nil {
		config.Port = cast.ToInt(m[1])
	}
	if m := serverPasswordAuthRegex.FindStringSubmatch(content); m != nil {
		config.PasswordAuth = strings.ToLower(m[1]) == "yes"
	}
	if m := serverPubkeyAuthRegex.FindStringSubmatch(content); m != nil {
		config.PubkeyAuth = strings.ToLower(m[1]) == "yes"
	}
	if m := serverRootLoginRegex.FindStringSubmatch(content); m != nil {
		config.RootLogin = strings.ToLower(m[1])
	}

	return config
}

func setServerOption(content, key, value string) string {
	line := regexp.MustCompile(`(?m)^#?\s*` + key + `\s+.*$`)
	if line.MatchString(content) {
		return line.ReplaceAllString(content, key+" "+value)
	}

	return content + "\n" + key + " " + value + "\n"
}

func readServerSocket(ctx context.Context, socket string) (map[string]string, error) {
	output, err := shell.Execf(ctx, "systemctl show '%s' --property=LoadState,ActiveState,UnitFileState,Accept,Listen,FragmentPath,DropInPaths", socket)
	if err != nil {
		return nil, err
	}
	state := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if key == "Listen" {
			state[key] += value + "\n"
		} else {
			state[key] = value
		}
	}
	if state["LoadState"] == "masked" && state["ActiveState"] == "active" {
		return nil, fmt.Errorf("SSH socket %s is masked and still active", socket)
	}
	switch state["LoadState"] {
	case "loaded", "not-found", "masked":
		return state, nil
	default:
		return nil, fmt.Errorf("failed to load %s: %s", socket, state["LoadState"])
	}
}

func serverSocketPortChanged(listen string, port uint) (bool, error) {
	if strings.TrimSpace(listen) == "" {
		return false, errors.New("SSH socket has no listening address")
	}
	newPort := strconv.FormatUint(uint64(port), 10)
	changed := false
	for line := range strings.SplitSeq(strings.TrimSpace(listen), "\n") {
		address, ok := strings.CutSuffix(line, " (Stream)")
		if !ok {
			return false, fmt.Errorf("unsupported SSH socket listener: %s", line)
		}
		_, oldPort, err := net.SplitHostPort(address)
		if err != nil {
			return false, err
		}
		changed = changed || oldPort != newPort
	}
	return changed, nil
}

func updateServerSocketConfigs(paths []string, port uint) error {
	type configFile struct {
		content string
		lines   []string
	}
	type listener struct {
		file int
		line int
	}
	files := make([]configFile, len(paths))
	var listeners []listener
	for i, path := range paths {
		content, err := io.Read(path)
		if err != nil {
			return err
		}
		files[i] = configFile{content: content, lines: strings.Split(content, "\n")}
		section := ""
		for j, line := range files[i].lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				section = line
			}
			key, value, ok := strings.Cut(line, "=")
			if section != "[Socket]" || !ok || strings.TrimSpace(key) != "ListenStream" {
				continue
			}
			// 空赋值会清空此前的监听列表，只修改最后一次清空后仍生效的原始配置。
			if strings.TrimSpace(value) == "" {
				listeners = nil
			} else {
				listeners = append(listeners, listener{file: i, line: j})
			}
		}
	}
	if len(listeners) == 0 {
		return errors.New("SSH socket has no ListenStream configuration")
	}

	newPort := strconv.FormatUint(uint64(port), 10)
	seen := make(map[string]bool)
	for _, item := range listeners {
		line := files[item.file].lines[item.line]
		key, value, _ := strings.Cut(line, "=")
		address := strings.TrimSpace(value)
		updated := newPort
		host := "::"
		if _, err := strconv.ParseUint(address, 10, 16); err != nil {
			host, _, err = net.SplitHostPort(address)
			if err != nil {
				return err
			}
			updated = net.JoinHostPort(host, newPort)
		}
		binding := net.JoinHostPort(host, newPort)
		if seen[binding] {
			files[item.file].lines[item.line] = ""
		} else {
			seen[binding] = true
			files[item.file].lines[item.line] = key + "=" + strings.Replace(value, address, updated, 1)
		}
	}
	// generator 的输出会在 daemon-reload 时重建，不能当成原始配置修改。
	for i, file := range files {
		if strings.HasPrefix(paths[i], "/run/systemd/generator") && strings.Join(file.lines, "\n") != file.content {
			return fmt.Errorf("cannot modify generated SSH socket configuration: %s", paths[i])
		}
	}
	for i, file := range files {
		content := strings.Join(file.lines, "\n")
		if content != file.content {
			if err := io.Write(paths[i], content, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
