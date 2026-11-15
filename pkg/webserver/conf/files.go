package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// 重定向、代理与上游各写成独立的编号片段文件，按序号范围归类

// ListFiles 列出目录中文件名匹配且序号落在区间内的片段，按文件名排序；目录不存在返回空。
// pattern 的首个捕获组是序号
func ListFiles(dir string, pattern *regexp.Regexp, from, to int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := pattern.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		if num, _ := strconv.Atoi(m[1]); num < from || num > to {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files
}

func ClearFiles(dir string, pattern *regexp.Regexp, from, to int) error {
	for _, file := range ListFiles(dir, pattern, from, to) {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete config: %w", err)
		}
	}
	return nil
}
