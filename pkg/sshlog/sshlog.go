// Package sshlog 解析 sshd 登录日志
package sshlog

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/pkg/types"
)

// 登录记录状态
const (
	StatusAccepted     = "accepted"
	StatusFailed       = "failed"
	StatusInvalidUser  = "invalid_user"
	StatusDisconnected = "disconnected"
)

// journalPattern 供 journalctl -g 预过滤，必须覆盖下面四条规则能识别的全部消息
const journalPattern = `Accepted \S+ for|Failed \S+ for|Invalid user|Disconnected from`

var (
	accepted    = regexp.MustCompile(`Accepted\s+(\S+)\s+for\s+(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)
	failed      = regexp.MustCompile(`Failed\s+(\S+)\s+for\s+(?:invalid user\s+)?(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)
	invalidUser = regexp.MustCompile(`Invalid user\s+(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)
	disconnect  = regexp.MustCompile(`Disconnected from\s+(?:authenticating\s+)?user\s+(\S+)\s+(\S+)\s+port\s+(\d+)`)
)

// ParseMessage 从日志消息中提取 SSH 登录信息，无法识别时返回 nil
func ParseMessage(msg string) *types.SSHLoginLog {
	if m := accepted.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			Method: m[1],
			User:   m[2],
			IP:     m[3],
			Port:   m[4],
			Status: StatusAccepted,
		}
	}
	if m := failed.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			Method: m[1],
			User:   m[2],
			IP:     m[3],
			Port:   m[4],
			Status: StatusFailed,
		}
	}
	if m := invalidUser.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			User:   m[1],
			IP:     m[2],
			Port:   m[3],
			Method: "-",
			Status: StatusInvalidUser,
		}
	}
	if m := disconnect.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			User:   m[1],
			IP:     m[2],
			Port:   m[3],
			Method: "-",
			Status: StatusDisconnected,
		}
	}

	return nil
}

// parseChunk 从连续日志字节中解析 SSH 登录记录
func parseChunk(data []byte) []types.SSHLoginLog {
	if len(data) == 0 {
		return nil
	}

	var logs []types.SSHLoginLog
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		record := ParseMessage(line)
		if record == nil {
			continue
		}
		record.Time = parseTime(line)
		logs = append(logs, *record)
	}

	return logs
}

// parseTime 从日志行首解析时间，失败返回 "-"
func parseTime(line string) string {
	// Debian 12 起 rsyslog 默认写 RFC 3339 时间戳
	if stamp, _, ok := strings.Cut(line, " "); ok {
		if t, err := time.Parse(time.RFC3339, stamp); err == nil {
			return t.Local().Format(time.DateTime)
		}
	}

	// 传统 syslog 时间戳定长且不带年份，用当前年份补全
	if len(line) < len(time.Stamp) {
		return "-"
	}
	t, err := time.Parse(time.Stamp, line[:len(time.Stamp)])
	if err != nil {
		return "-"
	}

	return t.AddDate(time.Now().Year(), 0, 0).Format(time.DateTime)
}
