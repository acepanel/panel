package biz

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/sshlog"
)

// checkSSH 增量检查 sshd 日志，上报登录成功与爆破尝试
func (uc *AlertUsecase) checkSSH(ctx context.Context) {
	const (
		// sshFailThreshold 单次检查窗口内触发爆破告警的失败次数
		sshFailThreshold uint = 5
		// sshCursorPrefix journalctl --show-cursor 在输出末尾打印的游标行前缀
		sshCursorPrefix = "-- cursor: "
	)

	// 没人接收就不必读
	if !uc.notify.EventEnabled(NotifyEventSSHLogin, NotifyEventSSHBruteforce) {
		return
	}

	now := time.Now()
	raw, err := uc.readSSHLog(ctx)
	if err != nil || raw == "" {
		return
	}

	failures := make(map[string]uint)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		// 游标行由 journalctl 的 --show-cursor 在末尾单独打印
		if next, ok := strings.CutPrefix(line, sshCursorPrefix); ok {
			uc.mu.Lock()
			uc.sshCursor = next
			uc.mu.Unlock()
			continue
		}

		record := sshlog.ParseMessage(line)
		if record == nil {
			continue
		}

		switch record.Status {
		case sshlog.StatusAccepted:
			uc.notify.SendEvent(NotifyEventSSHLogin, uc.t.Get("[AcePanel] SSH Login"), NotifyBody(uc.t.Get("SSH login succeeded"), [][2]string{
				{uc.t.Get("Username"), record.User},
				{uc.t.Get("IP"), record.IP},
				{uc.t.Get("Method"), record.Method},
				{uc.t.Get("Time"), now.Format(time.DateTime)},
			}))
		case sshlog.StatusFailed, sshlog.StatusInvalidUser:
			failures[record.IP]++
		}
	}

	for ip, count := range failures {
		if count < sshFailThreshold || !uc.sshShouldFire(ip, now) {
			continue
		}
		uc.notify.SendEvent(NotifyEventSSHBruteforce, uc.t.Get("[AcePanel] SSH Brute-force Attempts"), NotifyBody(uc.t.Get("too many failed SSH login attempts"), [][2]string{
			{uc.t.Get("IP"), ip},
			{uc.t.Get("Failed Attempts"), cast.ToString(count)},
			{uc.t.Get("Time"), now.Format(time.DateTime)},
		}))
	}
}

// readSSHLog 增量读取 sshd 日志
// 优先读文本日志：journald 按单位过滤要沿匹配链遍历，日志量大时取最后一条都要几十秒
func (uc *AlertUsecase) readSSHLog(ctx context.Context) (string, error) {
	for _, path := range []string{"/var/log/auth.log", "/var/log/secure"} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return uc.readSSHFile(path, info.Size())
		}
	}

	return uc.readSSHJournal(ctx)
}

// readSSHFile 按字节偏移读取文本日志新增的部分
func (uc *AlertUsecase) readSSHFile(path string, size int64) (string, error) {
	// sshScanBytes 单轮最多读取的文本日志字节数，防止停机过久后一次读入过多
	const sshScanBytes int64 = 8 << 20

	uc.mu.Lock()
	offset, known := uc.sshOffset, uc.sshLog == path
	uc.sshLog, uc.sshOffset = path, size
	uc.mu.Unlock()

	// 首轮只记录位置，避免面板启动时把历史日志全推一遍
	if !known {
		return "", nil
	}

	switch {
	case offset > size:
		offset = 0 // 日志已轮转，从新文件开头读
	case size-offset > sshScanBytes:
		offset = size - sshScanBytes // 停机过久时只补最近一段，首行可能被截断解析不出
	case offset == size:
		return "", nil
	}

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	buffer := make([]byte, size-offset)
	n, err := file.ReadAt(buffer, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	return string(buffer[:n]), nil
}

// readSSHJournal 无文本日志时回退到 journald，按游标增量读取
func (uc *AlertUsecase) readSSHJournal(ctx context.Context) (string, error) {
	const (
		// sshScanTimeout 单轮 sshd 日志扫描超时，避免 journalctl 卡死拖垮整个告警评估
		sshScanTimeout = 30 * time.Second
		// sshScanLimit 单轮最多读取的日志条数，防止爆破期间读入过多日志
		sshScanLimit = 5000
	)

	now := time.Now()

	uc.mu.Lock()
	since, cursor := uc.sshAt, uc.sshCursor
	uc.sshAt = now
	uc.mu.Unlock()

	limit, position := sshScanLimit, fmt.Sprintf("--after-cursor %q", cursor)
	switch {
	case since.IsZero():
		limit, position = 0, ""
	case cursor == "":
		position = fmt.Sprintf(`--since "@%d"`, since.Unix())
	}

	// 只用到 MESSAGE，-o cat 免去 journald 侧序列化几十个无关字段和这边的逐行反序列化
	scanCtx, cancel := context.WithTimeout(ctx, sshScanTimeout)
	defer cancel()
	raw, err := shell.Execf(scanCtx, `journalctl -t sshd -t sshd-session --no-pager -q -o cat --show-cursor -n %d %s 2>/dev/null`, limit, position)
	if err != nil {
		// 读取失败（含游标因日志轮转失效）则回退检查点并丢弃游标，下一轮按时间窗口重扫
		uc.mu.Lock()
		uc.sshAt = since
		uc.sshCursor = ""
		uc.mu.Unlock()
		return "", err
	}

	return raw, nil
}

// sshShouldFire 判断某来源是否已过静默期，并顺带清理过期记录
func (uc *AlertUsecase) sshShouldFire(ip string, now time.Time) bool {
	// sshFailSilence 同一来源两次爆破告警的最小间隔
	const sshFailSilence = 30 * time.Minute

	uc.mu.Lock()
	defer uc.mu.Unlock()

	for key, at := range uc.sshFired {
		if now.Sub(at) > sshFailSilence {
			delete(uc.sshFired, key)
		}
	}

	if at, ok := uc.sshFired[ip]; ok && now.Sub(at) < sshFailSilence {
		return false
	}
	uc.sshFired[ip] = now

	return true
}
