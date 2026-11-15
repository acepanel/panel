package sshlog

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/types"
)

func TestParseChunk(t *testing.T) {
	local := time.Local
	time.Local = time.FixedZone("CST", 8*3600)
	t.Cleanup(func() { time.Local = local })

	at := func(clock string) string {
		return strconv.Itoa(time.Now().Year()) + "-11-15 " + clock
	}

	tests := []struct {
		name string
		data string
		want []types.SSHLoginLog
	}{
		{
			name: "空数据",
		},
		{
			name: "OpenSSH 9.8 之前由 sshd 记录",
			data: `Nov 15 10:00:01 host sshd[1234]: Accepted publickey for root from 203.0.113.5 port 51234 ssh2: ED25519 SHA256:abc
Nov 15 10:00:02 host sshd[1235]: Failed password for root from 198.51.100.7 port 40022 ssh2
`,
			want: []types.SSHLoginLog{
				{Time: at("10:00:01"), User: "root", IP: "203.0.113.5", Port: "51234", Method: "publickey", Status: StatusAccepted},
				{Time: at("10:00:02"), User: "root", IP: "198.51.100.7", Port: "40022", Method: "password", Status: StatusFailed},
			},
		},
		{
			name: "OpenSSH 9.8 起由 sshd-session 记录",
			data: `Nov 15 10:00:01 host sshd-session[1300]: Accepted password for deploy from 203.0.113.6 port 51235 ssh2
Nov 15 10:00:02 host sshd-session[1301]: Failed password for invalid user admin from 198.51.100.7 port 40022 ssh2
Nov 15 10:00:03 host sshd-session[1302]: Invalid user oracle from 198.51.100.8 port 40023
Nov 15 10:00:04 host sshd-session[1303]: Disconnected from authenticating user root 198.51.100.9 port 40024 [preauth]
`,
			want: []types.SSHLoginLog{
				{Time: at("10:00:01"), User: "deploy", IP: "203.0.113.6", Port: "51235", Method: "password", Status: StatusAccepted},
				{Time: at("10:00:02"), User: "admin", IP: "198.51.100.7", Port: "40022", Method: "password", Status: StatusFailed},
				{Time: at("10:00:03"), User: "oracle", IP: "198.51.100.8", Port: "40023", Method: "-", Status: StatusInvalidUser},
				{Time: at("10:00:04"), User: "root", IP: "198.51.100.9", Port: "40024", Method: "-", Status: StatusDisconnected},
			},
		},
		{
			name: "rsyslog 的 RFC 3339 时间戳换算到本地时区",
			data: `2026-11-15T12:13:10.016750+00:00 fd2b594bd12b sshd[6950]: Invalid user nobodyhere from 127.0.0.1 port 40290
2026-11-15T12:13:12.212528+00:00 fd2b594bd12b sshd[6950]: Failed password for invalid user nobodyhere from 127.0.0.1 port 40290 ssh2
2026-11-15T20:15:44.421638+08:00 fd2b594bd12b sshd[6978]: Accepted password for root from 127.0.0.1 port 57792 ssh2
2026-11-15T20:15:44.494652+08:00 fd2b594bd12b sshd[6978]: Disconnected from user root 127.0.0.1 port 57792
`,
			want: []types.SSHLoginLog{
				{Time: "2026-11-15 20:13:10", User: "nobodyhere", IP: "127.0.0.1", Port: "40290", Method: "-", Status: StatusInvalidUser},
				{Time: "2026-11-15 20:13:12", User: "nobodyhere", IP: "127.0.0.1", Port: "40290", Method: "password", Status: StatusFailed},
				{Time: "2026-11-15 20:15:44", User: "root", IP: "127.0.0.1", Port: "57792", Method: "password", Status: StatusAccepted},
				{Time: "2026-11-15 20:15:44", User: "root", IP: "127.0.0.1", Port: "57792", Method: "-", Status: StatusDisconnected},
			},
		},
		{
			name: "时间戳无法识别",
			data: "sshd[1]: Accepted password for root from 203.0.113.5 port 51234 ssh2\n",
			want: []types.SSHLoginLog{
				{Time: "-", User: "root", IP: "203.0.113.5", Port: "51234", Method: "password", Status: StatusAccepted},
			},
		},
		{
			name: "跳过无关日志",
			data: `Nov 15 10:00:01 host sshd[1000]: Server listening on 0.0.0.0 port 22.
Nov 15 10:00:02 host sshd-session[1300]: Connection closed by authenticating user root 198.51.100.9 port 40024 [preauth]
Nov 15 10:00:03 host sudo:     root : TTY=pts/0 ; PWD=/root ; USER=root ; COMMAND=/usr/bin/ls
Nov 15 10:00:04 host CRON[2000]: pam_unix(cron:session): session opened for user root(uid=0) by (uid=0)
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.DeepEqual(t, parseChunk([]byte(tt.data)), tt.want)
		})
	}
}

func TestReadTail(t *testing.T) {
	// 每行固定 128 字节，块边界正好落在行尾；每 10 行有一条登录记录，端口即记录序号
	const lines, every = 30000, 10
	var aligned strings.Builder
	for i := range lines {
		line := strings.Repeat("x", 127)
		if i%every == 0 {
			line = fmt.Sprintf("%-127s", fmt.Sprintf("Nov 15 10:00:00 h sshd-session[1]: Accepted password for u from 10.0.0.1 port %05d", i/every))
		}
		must.Equal(t, len(line), 127)
		aligned.WriteString(line + "\n")
	}
	const records = lines / every
	// 首块 512 KB 即 4096 行，补 6 行再加一个 short 字节的短行，块边界就落在第 25910 行（登录记录）的第 short 字节
	straddle := func(short int) string {
		return aligned.String() + strings.Repeat(strings.Repeat("x", 127)+"\n", 6) + strings.Repeat("x", short-1) + "\n"
	}

	tests := []struct {
		name    string
		content string
		limit   int
		want    int
	}{
		{name: "首块内取满", content: aligned.String(), limit: 100, want: 100},
		{name: "跨块向前读取", content: aligned.String(), limit: 1000, want: 1000},
		{name: "读到文件头仍不足", content: aligned.String(), limit: 5000, want: records},
		{name: "块从行中间开始", content: "junk\n" + aligned.String(), limit: 100, want: 100},
		{name: "块边界切在登录记录中间", content: straddle(64), limit: 1000, want: 1000},
		{name: "块边界切在端口号中间", content: straddle(81), limit: 1000, want: 1000},
		{name: "整块没有换行", content: aligned.String() + strings.Repeat("x", int(chunkInitial)+1024), limit: 100, want: 100},
		{name: "空文件", limit: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := readTail(bytes.NewReader([]byte(tt.content)), int64(len(tt.content)), tt.limit)
			must.NoError(t, err)
			must.Len(t, logs, tt.want)
			// 取到的是最新的记录，且按时间正序
			for i, log := range logs {
				check.Equal(t, log.Port, fmt.Sprintf("%05d", records-tt.want+i))
			}
		})
	}
}
