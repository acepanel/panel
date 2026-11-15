package tamper

import (
	"path/filepath"
	"strings"
)

// MatchExt 判断文件是否命中受保护后缀(exts 为空表示全部文件)
func MatchExt(path string, exts []string) bool {
	if len(exts) == 0 {
		return true
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, e := range exts {
		if normExt(e) == ext {
			return true
		}
	}
	return false
}

// ExcludeMatches 判断单个排除项是否命中路径(支持绝对路径前缀与路径段名)
func ExcludeMatches(ex, path string) bool {
	ex = strings.TrimSpace(ex)
	if ex == "" {
		return false
	}
	if filepath.IsAbs(ex) {
		return path == ex || strings.HasPrefix(path, strings.TrimRight(ex, "/")+"/")
	}
	// 相对名:匹配任意位置上连续的路径段,可以是多段(如 cache/tmp)
	return strings.Contains(path+"/", "/"+strings.Trim(ex, "/")+"/")
}

// UnderRoot 判断路径是否位于根目录内(含根自身)
func UnderRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, strings.TrimRight(root, "/")+"/")
}

// Covered 判断路径在规则集下是否处于保护范围
// 目录按覆盖范围计(位于规则内且未排除),文件还需命中后缀
func Covered(rules []Rule, path string, isDir bool) bool {
	for _, r := range rules {
		for _, root := range r.Paths {
			if !UnderRoot(path, root) {
				continue
			}
			if isExcluded(path, r.Excludes) {
				continue
			}
			if isDir || MatchExt(path, r.Exts) {
				return true
			}
		}
	}
	return false
}

func isExcluded(path string, excludes []string) bool {
	for _, ex := range excludes {
		if ExcludeMatches(ex, path) {
			return true
		}
	}
	return false
}

// normExt 归一化扩展名(去点,小写)
func normExt(e string) string {
	return strings.ToLower(strings.TrimPrefix(e, "."))
}
