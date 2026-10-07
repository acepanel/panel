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

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

// logArchivePattern 轮转归档的文件名，时间戳是该文件内日志的结束时刻
var logArchivePattern = regexp.MustCompile(`^(app|db|http)-(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3})(?:\.\d+)?\.log$`)

// logFile 日志文件及其内日志所属的日期
type logFile struct {
	path    string
	date    string
	current bool
}

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
// level 为日志级别（DEBUG/INFO/WARN/ERROR），空字符串表示不过滤
func (r *logRepo) List(logType string, limit int, date, level string) ([]biz.LogEntry, error) {
	if date == "" {
		date = time.Now().Format(time.DateOnly)
	}
	files, err := r.files(logType)
	if err != nil {
		return nil, err
	}

	// 一天可能因为大小轮转分成多个文件，从最新的往前读，先过滤再计数，凑够 limit 条为止，最新的在前面
	entries := []biz.LogEntry{}
	for i := len(files) - 1; i >= 0 && len(entries) < limit; i-- {
		if files[i].date != date {
			continue
		}
		lines, err := readLines(files[i].path)
		if err != nil {
			return nil, err
		}
		for j := len(lines) - 1; j >= 0 && len(entries) < limit; j-- {
			entry, err := r.parseLine(lines[j], logType)
			if err != nil || (level != "" && entry.Level != level) {
				continue
			}
			entries = append(entries, entry)
		}
	}

	// 如果是app日志，查询用户名
	if logType == biz.LogTypeApp {
		r.fillOperatorNames(entries)
	}

	return entries, nil
}

// ListDates 获取可用的日志日期列表，当天由 List 的空日期覆盖，不在其中
func (r *logRepo) ListDates(logType string) ([]string, error) {
	files, err := r.files(logType)
	if err != nil {
		return nil, err
	}

	today := time.Now().Format(time.DateOnly)
	dates := lo.Uniq(lo.FilterMap(files, func(file logFile, _ int) (string, bool) {
		return file.date, file.date != today
	}))

	// 按日期倒序排列，最新的在前面
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))

	return dates, nil
}

// Clean 清理指定日期及之前的日志，当前文件还在写入，只能清空
func (r *logRepo) Clean(logType string, date string) error {
	files, err := r.files(logType)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.date > date {
			continue
		}
		if file.current {
			err = os.Truncate(file.path, 0)
		} else {
			err = os.Remove(file.path)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// files 按时间先后列出某类日志的归档与当前文件
func (r *logRepo) files(logType string) ([]logFile, error) {
	dir := filepath.Join(app.Root, "panel/storage/logs")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	files := make([]logFile, 0, len(entries))
	for _, entry := range entries {
		matches := logArchivePattern.FindStringSubmatch(entry.Name())
		if matches == nil || matches[1] != logType {
			continue
		}
		end, err := time.ParseInLocation("2006-01-02T15-04-05.000", matches[2], time.Local)
		if err != nil {
			continue
		}
		// 零点轮转的归档时间戳是次日零点，往前退一点才是日志所属的日期
		files = append(files, logFile{
			path: filepath.Join(dir, entry.Name()),
			date: end.Add(-time.Nanosecond).Format(time.DateOnly),
		})
	}

	// 当前文件名里没有时间戳，按最后写入时间算
	current := filepath.Join(dir, logType+".log")
	if info, err := os.Stat(current); err == nil {
		files = append(files, logFile{path: current, date: info.ModTime().Format(time.DateOnly), current: true})
	}

	return files, nil
}

// readLines 读取文件中的非空行
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		// 列出后可能刚被轮转清理掉
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	var lines []string
	scanner := bufio.NewScanner(file)
	// 增加缓冲区大小以处理较长的日志行
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if line := scanner.Text(); strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}

	return lines, scanner.Err()
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
	ids := lo.Keys(userIDs)

	var users []biz.User
	r.db.Select("id", "username").Where("id IN ?", ids).Find(&users)

	// 构建ID到用户名的映射
	userMap := lo.SliceToMap(users, func(user biz.User) (uint, string) {
		return user.ID, user.Username
	})

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
