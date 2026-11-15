package ssh

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestApplyServerPort(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		stop       string
		restart    string
		wantErr    string
		cancelled  bool
		queryError bool
	}{
		{name: "没有 socket 时重启常驻服务", state: "LoadState=not-found\nActiveState=inactive\n", restart: "restart ssh"},
		{name: "未启用 socket 时保留常驻服务", state: "LoadState=loaded\nActiveState=inactive\nUnitFileState=disabled\n", restart: "restart ssh"},
		{name: "masked 但仍运行时保留原监听", state: "LoadState=masked\nActiveState=active\nUnitFileState=masked\nAccept=no\nListen=[::]:2222 (Stream)\n", wantErr: "ssh.socket is masked"},
		{name: "masked 且未运行时保留常驻服务", state: "LoadState=masked\nActiveState=inactive\nUnitFileState=masked\n", restart: "restart ssh"},
		{name: "socket 激活同时更新监听和常驻进程", state: "LoadState=loaded\nActiveState=active\nUnitFileState=enabled\nAccept=no\nListen=[::]:2222 (Stream)\n", stop: "stop ssh.socket ssh", restart: "restart ssh.socket"},
		{name: "临时启用的 socket 保留激活方式", state: "LoadState=loaded\nActiveState=inactive\nUnitFileState=enabled-runtime\nAccept=no\nListen=[::]:2222 (Stream)\n", stop: "stop ssh.socket ssh", restart: "restart ssh.socket"},
		{name: "按连接激活不启动互斥的常驻服务", state: "LoadState=loaded\nActiveState=active\nUnitFileState=disabled\nAccept=yes\nListen=[::]:2222 (Stream)\n", restart: "restart ssh.socket"},
		{name: "配置写入后取消请求仍完成应用", state: "LoadState=not-found\nActiveState=inactive\n", restart: "restart ssh", cancelled: true},
		{name: "查询失败时不改变服务状态", state: "LoadState=loaded\nActiveState=active\n", queryError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "commands")
			must.NoError(t, os.WriteFile(filepath.Join(dir, "sshd"), []byte("#!/bin/sh\nexit 0\n"), 0755))
			must.NoError(t, os.WriteFile(filepath.Join(dir, "systemctl"), []byte(`#!/bin/sh
printf '%s\n' "$*" >> "$SSH_TEST_LOG"
if [ "$1" = show ]; then
    printf '%s' "$SSH_TEST_STATE"
    exit "$SSH_TEST_EXIT"
fi
`), 0755))
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SSH_TEST_LOG", logPath)
			t.Setenv("SSH_TEST_STATE", tt.state)
			t.Setenv("SSH_TEST_EXIT", "0")
			if tt.queryError {
				t.Setenv("SSH_TEST_EXIT", "1")
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.cancelled {
				cancel()
			}
			err := ApplyServerPort(ctx, "ssh", 2222)
			if tt.queryError || tt.wantErr != "" {
				must.Error(t, err)
				if tt.wantErr != "" {
					check.True(t, strings.Contains(err.Error(), tt.wantErr))
				}
			} else {
				must.NoError(t, err)
			}
			log, err := os.ReadFile(logPath)
			must.NoError(t, err)
			var stops, restarts []string
			for line := range strings.SplitSeq(strings.TrimSpace(string(log)), "\n") {
				check.False(t, strings.HasPrefix(line, "enable ") || strings.HasPrefix(line, "disable "))
				if strings.HasPrefix(line, "stop ") {
					stops = append(stops, line)
				}
				if strings.HasPrefix(line, "restart ") {
					restarts = append(restarts, line)
				}
			}
			check.Equal(t, strings.Join(stops, "\n"), tt.stop)
			if tt.queryError || tt.wantErr != "" {
				check.Len(t, restarts, 0)
				check.False(t, strings.Contains(string(log), "daemon-reload"))
			} else {
				check.Equal(t, strings.Join(restarts, "\n"), tt.restart)
			}
		})
	}
}

func TestServerSocketPortChanged(t *testing.T) {
	tests := []struct {
		name    string
		listen  string
		changed bool
		wantErr bool
	}{
		{name: "双栈端口需要更新", listen: "0.0.0.0:22 (Stream)\n[::]:22 (Stream)\n", changed: true},
		{name: "部分监听尚未更新", listen: "127.0.0.1:2222 (Stream)\n[fe80::1%eth0]:22 (Stream)\n", changed: true},
		{name: "generator 已更新端口", listen: "[::]:2222 (Stream)\n"},
		{name: "拒绝空监听", wantErr: true},
		{name: "拒绝 Unix socket", listen: "/run/ssh.sock (Stream)", wantErr: true},
		{name: "拒绝非 TCP 监听", listen: "0.0.0.0:22 (Datagram)", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed, err := serverSocketPortChanged(tt.listen, 2222)
			if tt.wantErr {
				must.Error(t, err)
				return
			}
			must.NoError(t, err)
			check.Equal(t, changed, tt.changed)
		})
	}
}

func TestUpdateServerSocketConfigs(t *testing.T) {
	tests := []struct {
		name    string
		configs []string
		want    []string
		wantErr bool
	}{
		{
			name:    "原地更新裸端口并保留注释和其他配置",
			configs: []string{"# SSH 配置\n[Unit]\nDescription=SSH\n\n[Socket]\n#ListenStream=23\n ListenStream = 22 \nAccept=yes\n\n[Install]\nWantedBy=sockets.target\n"},
			want:    []string{"# SSH 配置\n[Unit]\nDescription=SSH\n\n[Socket]\n#ListenStream=23\n ListenStream = 2222 \nAccept=yes\n\n[Install]\nWantedBy=sockets.target\n"},
		},
		{
			name:    "保留 IPv4 和 IPv6 绑定地址",
			configs: []string{"[Socket]\nListenStream=127.0.0.1:22\nListenStream=[fe80::1%eth0]:22\nFreeBind=yes\n"},
			want:    []string{"[Socket]\nListenStream=127.0.0.1:2222\nListenStream=[fe80::1%eth0]:2222\nFreeBind=yes\n"},
		},
		{
			name:    "已有覆盖清空列表时只改生效文件",
			configs: []string{"[Socket]\nListenStream=22\nAccept=no\n", "[Socket]\nListenStream=\nListenStream=127.0.0.1:2200\n"},
			want:    []string{"[Socket]\nListenStream=22\nAccept=no\n", "[Socket]\nListenStream=\nListenStream=127.0.0.1:2222\n"},
		},
		{
			name:    "跨文件合并重复监听",
			configs: []string{"[Socket]\nListenStream=22\n", "[Socket]\nListenStream=[::]:2200\nListenStream=127.0.0.1:2200\n"},
			want:    []string{"[Socket]\nListenStream=2222\n", "[Socket]\n\nListenStream=127.0.0.1:2222\n"},
		},
		{
			name:    "不改已经被覆盖的监听",
			configs: []string{"[Socket]\nListenStream=/run/old.sock\n", "[Socket]\nListenStream=\nListenStream=2222\n"},
			want:    []string{"[Socket]\nListenStream=/run/old.sock\n", "[Socket]\nListenStream=\nListenStream=2222\n"},
		},
		{
			name:    "解析失败时不写入任何文件",
			configs: []string{"[Socket]\nListenStream=22\n", "[Socket]\nListenStream=/run/ssh.sock\n"},
			wantErr: true,
		},
		{
			name:    "拒绝空监听列表",
			configs: []string{"[Socket]\nListenStream=22\n", "[Socket]\nListenStream=\n"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var paths []string
			for i, config := range tt.configs {
				path := filepath.Join(dir, strconv.Itoa(i)+".conf")
				must.NoError(t, os.WriteFile(path, []byte(config), 0600))
				paths = append(paths, path)
			}
			err := updateServerSocketConfigs(paths, 2222)
			want := tt.want
			if tt.wantErr {
				must.Error(t, err)
				want = tt.configs
			} else {
				must.NoError(t, err)
			}
			for i, path := range paths {
				content, err := os.ReadFile(path)
				must.NoError(t, err)
				check.Equal(t, string(content), want[i])
				info, err := os.Stat(path)
				must.NoError(t, err)
				check.Equal(t, info.Mode().Perm(), os.FileMode(0600))
			}
			entries, err := os.ReadDir(dir)
			must.NoError(t, err)
			check.Len(t, entries, len(tt.configs))
		})
	}
}

func TestParseServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    ServerConfig
	}{
		{
			name:    "全部注释时取默认值",
			content: "#Port 22\n#PermitRootLogin prohibit-password\n#PubkeyAuthentication yes\n#PasswordAuthentication yes\n",
			want:    ServerConfig{Port: 22, PasswordAuth: true, PubkeyAuth: true, RootLogin: "yes"},
		},
		{
			name:    "读取显式配置",
			content: "Include /etc/ssh/sshd_config.d/*.conf\n\nPort 2222\nPermitRootLogin Prohibit-Password\nPubkeyAuthentication no\nPasswordAuthentication No\n",
			want:    ServerConfig{Port: 2222, PasswordAuth: false, PubkeyAuth: false, RootLogin: "prohibit-password"},
		},
		{
			name:    "同名选项以首个为准",
			content: "Port 2222\nPort 22\nPasswordAuthentication no\nPasswordAuthentication yes\n",
			want:    ServerConfig{Port: 2222, PasswordAuth: false, PubkeyAuth: true, RootLogin: "yes"},
		},
		{
			name:    "忽略 Match 块里缩进的选项",
			content: "Match User deploy\n    PasswordAuthentication no\n    PermitRootLogin no\n",
			want:    ServerConfig{Port: 22, PasswordAuth: true, PubkeyAuth: true, RootLogin: "yes"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, parseServerConfig(tt.content), tt.want)
		})
	}
}

func TestSetServerOption(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		value   string
		want    string
	}{
		{
			name:    "替换已有选项",
			content: "Port 22\nPermitRootLogin yes\nPasswordAuthentication yes\n",
			key:     "PermitRootLogin",
			value:   "prohibit-password",
			want:    "Port 22\nPermitRootLogin prohibit-password\nPasswordAuthentication yes\n",
		},
		{
			name:    "启用被注释的选项",
			content: "#Port 22\n#AddressFamily any\n",
			key:     "Port",
			value:   "2222",
			want:    "Port 2222\n#AddressFamily any\n",
		},
		{
			name:    "没有时追加",
			content: "PermitRootLogin yes\n",
			key:     "PubkeyAuthentication",
			value:   "no",
			want:    "PermitRootLogin yes\n\nPubkeyAuthentication no\n",
		},
		{
			name:    "不动其他选项",
			content: "PasswordAuthentication yes\nKbdInteractiveAuthentication no\n",
			key:     "PubkeyAuthentication",
			value:   "yes",
			want:    "PasswordAuthentication yes\nKbdInteractiveAuthentication no\n\nPubkeyAuthentication yes\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Equal(t, setServerOption(tt.content, tt.key, tt.value), tt.want)
		})
	}
}
