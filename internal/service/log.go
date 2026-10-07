package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/sshlog"
	"github.com/acepanel/panel/v3/pkg/types"
)

// SSH 日志反向分块读取参数：首块 512 KB 起，每次翻倍，单块上限 64 MB
const (
	sshLogChunkInitial int64 = 512 * 1024
	sshLogChunkMax     int64 = 64 * 1024 * 1024
)

type LogService struct {
	t       *gotext.Locale
	logRepo *biz.LogUsecase
}

func NewLogService(logUsecase *biz.LogUsecase, t *gotext.Locale) *LogService {
	return &LogService{
		t:       t,
		logRepo: logUsecase,
	}
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

	entries, err := s.logRepo.List(req.Type, req.Limit, req.Date, req.Level)
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

// Clean 清理指定日期及之前的日志
func (s *LogService) Clean(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.LogClean](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.logRepo.Clean(r.Context(), req.Type, req.Date); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// SSH 获取 SSH 登录日志
func (s *LogService) SSH(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	logs, err := s.sshFromJournalctl(r.Context(), limit)
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
func (s *LogService) sshFromJournalctl(ctx context.Context, limit int) ([]types.SSHLoginLog, error) {
	raw, err := shell.Execf(ctx, "journalctl -u sshd -u ssh --no-pager -o json -n %d 2>/dev/null", limit*5)
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

		record := sshlog.ParseMessage(entry.Message)
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
		logs = append(sshlog.ParseChunk(buf), logs...)
		window *= 2
	}

	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	return logs, nil
}
