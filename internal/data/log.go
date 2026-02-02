package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
)

var (
	logArchivePatternApp  = regexp.MustCompile(`^app-(\d{4}-\d{2}-\d{2})T.*\.log$`)
	logArchivePatternDB   = regexp.MustCompile(`^db-(\d{4}-\d{2}-\d{2})T.*\.log$`)
	logArchivePatternHTTP = regexp.MustCompile(`^http-(\d{4}-\d{2}-\d{2})T.*\.log$`)
)

type logRepo struct {
	db *gorm.DB
}

func NewLogRepo(db *gorm.DB) biz.LogRepo {
	return &logRepo{
		db: db,
	}
}

// List 获取日志列表
// date 格式为 YYYY-MM-DD，空字符串表示当天日志
func (r *logRepo) List(logType string, limit int, date string) ([]biz.LogEntry, error) {
	logDir := filepath.Join(app.Root, "panel/storage/logs")

	var logPath string
	if date == "" {
		// 无日期参数，读取当前日志文件
		logPath = filepath.Join(logDir, logType+".log")
	} else {
		// 有日期参数，查找对应的归档日志文件
		pattern := getLogArchivePattern(logType)

		entries, err := os.ReadDir(logDir)
		if err != nil {
			if os.IsNotExist(err) {
				return []biz.LogEntry{}, nil
			}
			return nil, err
		}

		found := false
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matches := pattern.FindStringSubmatch(entry.Name())
			if len(matches) == 2 && matches[1] == date {
				logPath = filepath.Join(logDir, entry.Name())
				found = true
				break
			}
		}

		if !found {
			return []biz.LogEntry{}, nil
		}
	}

	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []biz.LogEntry{}, nil
		}
		return nil, err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

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

	// 如果是app日志，查询用户名
	if logType == biz.LogTypeApp {
		r.fillOperatorNames(entries)
	}

	return entries, nil
}

// ListDates 获取可用的日志日期列表
func (r *logRepo) ListDates(logType string) ([]string, error) {
	logDir := filepath.Join(app.Root, "panel/storage/logs")

	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	pattern := getLogArchivePattern(logType)

	dates := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := pattern.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			dates = append(dates, matches[1])
		}
	}

	// 按日期倒序排列，最新的在前面
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	return dates, nil
}

// fillOperatorNames 填充操作员用户名
func (r *logRepo) fillOperatorNames(entries []biz.LogEntry) {
	// 收集所有用户ID
	userIDs := make(map[uint]bool)
	for _, entry := range entries {
		if entry.OperatorID > 0 {
			userIDs[entry.OperatorID] = true
		}
	}

	if len(userIDs) == 0 {
		return
	}

	// 批量查询用户名
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}

	var users []biz.User
	r.db.Select("id", "username").Where("id IN ?", ids).Find(&users)

	// 构建ID到用户名的映射
	userMap := make(map[uint]string)
	for _, user := range users {
		userMap[user.ID] = user.Username
	}

	// 填充用户名
	for i := range entries {
		if entries[i].OperatorID > 0 {
			if username, ok := userMap[entries[i].OperatorID]; ok {
				entries[i].OperatorName = username
			}
		}
	}
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

// getLogArchivePattern 获取归档日志文件名匹配正则表达式
func getLogArchivePattern(logType string) *regexp.Regexp {
	switch logType {
	case biz.LogTypeApp:
		return logArchivePatternApp
	case biz.LogTypeDB:
		return logArchivePatternDB
	case biz.LogTypeHTTP:
		return logArchivePatternHTTP
	default:
		return logArchivePatternApp
	}
}
