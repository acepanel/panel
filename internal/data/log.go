package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
)

type logRepo struct{}

func NewLogRepo() biz.LogRepo {
	return &logRepo{}
}

// List 获取日志列表
func (r *logRepo) List(logType string, limit int) ([]biz.LogEntry, error) {
	var filename string
	switch logType {
	case biz.LogTypeApp:
		filename = "app.log"
	case biz.LogTypeDB:
		filename = "db.log"
	case biz.LogTypeHTTP:
		filename = "http.log"
	default:
		filename = "app.log"
	}

	logPath := filepath.Join(app.Root, "panel/storage/logs", filename)

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []biz.LogEntry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	// 读取所有行
	var lines []string
	scanner := bufio.NewScanner(file)
	// 增加缓冲区大小以处理较长的日志行
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	if err = scanner.Err(); err != nil {
		return nil, err
	}

	// 从末尾取指定数量的行
	start := 0
	if len(lines) > limit {
		start = len(lines) - limit
	}
	lines = lines[start:]

	// 倒序处理，最新的在前面
	entries := make([]biz.LogEntry, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		entry, err := r.parseLine(lines[i], logType)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// parseLine 解析日志行
func (r *logRepo) parseLine(line string, logType string) (biz.LogEntry, error) {
	var rawEntry map[string]any
	if err := json.Unmarshal([]byte(line), &rawEntry); err != nil {
		return biz.LogEntry{}, err
	}

	entry := biz.LogEntry{
		Extra: make(map[string]any),
	}

	// 解析通用字段
	if t, ok := rawEntry["time"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			entry.Time = parsed
		}
	}
	if level, ok := rawEntry["level"].(string); ok {
		entry.Level = level
	}
	if msg, ok := rawEntry["msg"].(string); ok {
		entry.Msg = msg
	}

	// 解析操作日志特有字段
	if logType == biz.LogTypeApp {
		if t, ok := rawEntry["type"].(string); ok {
			entry.Type = t
		}
		if opID, ok := rawEntry["operator_id"].(float64); ok {
			entry.OperatorID = uint(opID)
		}
	}

	// 其他字段放入Extra
	excludeKeys := map[string]bool{
		"time": true, "level": true, "msg": true, "type": true, "operator_id": true,
	}
	for k, v := range rawEntry {
		if !excludeKeys[k] {
			entry.Extra[k] = v
		}
	}

	return entry, nil
}
