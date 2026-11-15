package sshlog

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

// 日志文件反向分块读取参数：首块 512 KB 起，每次翻倍，单块上限 64 MB
const (
	chunkInitial int64 = 512 * 1024
	chunkMax     int64 = 64 * 1024 * 1024
)

// ErrNotFound journal 里没有登录记录，也找不到日志文件
var ErrNotFound = errors.New("SSH log file not found")

// Read 读取最近 limit 条登录记录，按时间正序返回，journal 取不到时回落到日志文件
func Read(ctx context.Context, limit int) ([]types.SSHLoginLog, error) {
	logs, err := readJournal(ctx, limit)
	if err != nil {
		logs, err = readFile(limit)
	}

	return logs, err
}

// readJournal 通过 journalctl 获取 SSH 日志
// 登录记录在 journal 里很稀疏，连接关闭、PAM 之类的噪音是它的几十倍，让 journalctl 自己按正则过滤后倒序取 limit 条，
// 面板只收到命中的行，也不用再反序列化整条 journal 记录。
// 按 syslog 标识而不是单元过滤：登录后的会话进程会被 pam_systemd 挪进 session-N.scope，不在 sshd 单元下
func readJournal(ctx context.Context, limit int) ([]types.SSHLoginLog, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipe, err := shell.ExecWithPipe(ctx, fmt.Sprintf("journalctl -t sshd -t sshd-session --no-pager -q -o json --output-fields=MESSAGE,__REALTIME_TIMESTAMP -g %s -n %d -r 2>/dev/null", shell.Quote(journalPattern), limit))
	if err != nil {
		return nil, err
	}
	defer func(pipe io.ReadCloser) { _ = pipe.Close() }(pipe)

	var logs []types.SSHLoginLog
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for len(logs) < limit && scanner.Scan() {
		var entry struct {
			RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
			Message           string `json:"MESSAGE"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}

		record := ParseMessage(entry.Message)
		if record == nil {
			continue
		}

		// 解析 journalctl 微秒时间戳
		if us, err := strconv.ParseInt(entry.RealtimeTimestamp, 10, 64); err == nil {
			record.Time = time.Unix(0, us*int64(time.Microsecond)).Format("2006-01-02 15:04:05")
		}

		logs = append(logs, *record)
	}

	if len(logs) == 0 {
		return nil, errors.New("no SSH log entries found")
	}

	// 倒序读的，翻回时间正序与文件解析保持一致
	slices.Reverse(logs)

	return logs, nil
}

// readFile 从日志文件中解析 SSH 日志
func readFile(limit int) ([]types.SSHLoginLog, error) {
	paths := []string{"/var/log/auth.log", "/var/log/secure"}
	var file *os.File
	for _, p := range paths {
		if f, err := os.Open(p); err == nil {
			file = f
			break
		}
	}
	if file == nil {
		return nil, ErrNotFound
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return readTail(file, stat.Size(), limit)
}

// readTail 从末尾反向分块读取，直到收集满 limit 条或读到文件头
func readTail(file io.ReaderAt, size int64, limit int) ([]types.SSHLoginLog, error) {
	var logs []types.SSHLoginLog
	// 块几乎总是从行中间开始，开头这一行留给更早的一块接到末尾，拼回完整的一行
	var carry []byte
	offset := size
	window := chunkInitial
	for offset > 0 && len(logs) < limit {
		if window > chunkMax {
			window = chunkMax
		}
		readSize := min(window, offset)
		offset -= readSize

		buf := make([]byte, readSize, readSize+int64(len(carry)))
		if _, err := file.ReadAt(buf, offset); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		buf = append(buf, carry...)
		if offset > 0 {
			carry, buf, _ = bytes.Cut(buf, []byte{'\n'})
		}

		// 新读的块时间上更早，前置到已收集 logs 之前
		logs = append(parseChunk(buf), logs...)
		window *= 2
	}

	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	return logs, nil
}
