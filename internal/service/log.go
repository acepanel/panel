package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

// SSH 日志反向分块读取参数：首块 512 KB 起，每次翻倍，单块上限 64 MB
const (
	sshLogChunkInitial int64 = 512 * 1024
	sshLogChunkMax     int64 = 64 * 1024 * 1024
)

// SSH 日志正则
var (
	sshAccepted    = regexp.MustCompile(`Accepted\s+(\S+)\s+for\s+(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)
	sshFailed      = regexp.MustCompile(`Failed\s+(\S+)\s+for\s+(?:invalid user\s+)?(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)
	sshInvalidUser = regexp.MustCompile(`Invalid user\s+(\S+)\s+from\s+(\S+)\s+port\s+(\d+)`)
	sshDisconnect  = regexp.MustCompile(`Disconnected from\s+(?:authenticating\s+)?user\s+(\S+)\s+(\S+)\s+port\s+(\d+)`)
)

type LogService struct {
	t       *gotext.Locale
	logRepo *biz.LogUsecase
}

func NewLogService(i do.Injector) (*LogService, error) {
	return &LogService{
		t:       do.MustInvoke[*gotext.Locale](i),
		logRepo: do.MustInvoke[*biz.LogUsecase](i),
	}, nil
}

// List 获取日志列表
func (s *LogService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.LogList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 默认限制
	if req.Limit == 0 {
		req.Limit = 100
	}

	entries, err := s.logRepo.List(req.Type, req.Limit, req.Date)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, entries)
}

// Dates 获取日志日期列表
func (s *LogService) Dates(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.LogDates](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	dates, err := s.logRepo.ListDates(req.Type)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, dates)
}

// SSH 获取 SSH 登录日志
func (s *LogService) SSH(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	logs, err := s.sshFromJournalctl(limit)
	if err != nil {
		logs, err = s.sshFromLogFile(limit)
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, logs)
}

// sshFromJournalctl 通过 journalctl 获取 SSH 日志
func (s *LogService) sshFromJournalctl(limit int) ([]types.SSHLoginLog, error) {
	raw, err := shell.Execf("journalctl -u sshd -u ssh --no-pager -o json -n %d 2>/dev/null", limit*5)
	if err != nil || raw == "" {
		return nil, errors.New(s.t.Get("journalctl is not available"))
	}

	var logs []types.SSHLoginLog
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		var entry struct {
			RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
			Message           string `json:"MESSAGE"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}

		record := parseSSHMessage(entry.Message)
		if record == nil {
			continue
		}

		// 解析 journalctl 微秒时间戳
		if us, err := strconv.ParseInt(entry.RealtimeTimestamp, 10, 64); err == nil {
			record.Time = time.Unix(0, us*int64(time.Microsecond)).Format("2006-01-02 15:04:05")
		}

		logs = append(logs, *record)
		if len(logs) >= limit {
			break
		}
	}

	if len(logs) == 0 {
		return nil, errors.New(s.t.Get("no SSH log entries found"))
	}

	return logs, nil
}

// sshFromLogFile 从日志文件中解析 SSH 日志
func (s *LogService) sshFromLogFile(limit int) ([]types.SSHLoginLog, error) {
	paths := []string{"/var/log/auth.log", "/var/log/secure"}
	var file *os.File
	for _, p := range paths {
		if f, err := os.Open(p); err == nil {
			file = f
			break
		}
	}
	if file == nil {
		return nil, errors.New(s.t.Get("SSH log file not found"))
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// 从末尾反向分块读取，直到收集满 limit 条或读到文件头
	var logs []types.SSHLoginLog
	offset := stat.Size()
	window := sshLogChunkInitial
	for offset > 0 && len(logs) < limit {
		if window > sshLogChunkMax {
			window = sshLogChunkMax
		}
		readSize := min(window, offset)
		offset -= readSize

		buf := make([]byte, readSize)
		if _, err = file.ReadAt(buf, offset); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		// 若块开头是残行（前一字节不是换行），丢到本块第一个换行为止
		if offset > 0 {
			var prev [1]byte
			if _, err = file.ReadAt(prev[:], offset-1); err == nil && prev[0] != '\n' {
				if idx := bytes.IndexByte(buf, '\n'); idx >= 0 {
					buf = buf[idx+1:]
				} else {
					buf = nil
				}
			}
		}

		// 新读的块时间上更早，前置到已收集 logs 之前
		logs = append(parseSSHChunk(buf), logs...)
		window *= 2
	}

	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	return logs, nil
}

// parseSSHChunk 从连续日志字节中解析 SSH 登录记录
func parseSSHChunk(data []byte) []types.SSHLoginLog {
	if len(data) == 0 {
		return nil
	}
	var logs []types.SSHLoginLog
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "sshd[") {
			continue
		}
		record := parseSSHMessage(line)
		if record == nil {
			continue
		}
		// 从行首解析 syslog 时间戳（如 "Feb 11 08:30:01"）
		record.Time = parseSSHLogTime(line)
		logs = append(logs, *record)
	}
	return logs
}

// parseSSHMessage 从日志消息中提取 SSH 登录信息
func parseSSHMessage(msg string) *types.SSHLoginLog {
	if m := sshAccepted.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			Method: m[1],
			User:   m[2],
			IP:     m[3],
			Port:   m[4],
			Status: "accepted",
		}
	}
	if m := sshFailed.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			Method: m[1],
			User:   m[2],
			IP:     m[3],
			Port:   m[4],
			Status: "failed",
		}
	}
	if m := sshInvalidUser.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			User:   m[1],
			IP:     m[2],
			Port:   m[3],
			Method: "-",
			Status: "invalid_user",
		}
	}
	if m := sshDisconnect.FindStringSubmatch(msg); m != nil {
		return &types.SSHLoginLog{
			User:   m[1],
			IP:     m[2],
			Port:   m[3],
			Method: "-",
			Status: "disconnected",
		}
	}
	return nil
}

// parseSSHLogTime 从 syslog 格式行中解析时间
func parseSSHLogTime(line string) string {
	// syslog 格式：Mon DD HH:MM:SS（前 15 个字符）
	if len(line) < 15 {
		return "-"
	}
	ts := line[:15]
	// 使用当前年份补全
	t, err := time.Parse("Jan  2 15:04:05", ts)
	if err != nil {
		t, err = time.Parse("Jan 2 15:04:05", ts)
		if err != nil {
			return "-"
		}
	}
	t = t.AddDate(time.Now().Year(), 0, 0)
	return t.Format("2006-01-02 15:04:05")
}
