package types

import (
	"cmp"
	"slices"
	"strings"
)

// LocationKind location 的匹配方式，面板统一用 nginx 的写法表达，各方言自行渲染
type LocationKind uint8

const (
	LocationPrefix   LocationKind = iota // 前缀匹配，含 ^~
	LocationExact                        // = 精确匹配
	LocationRegex                        // ~ 正则
	LocationRegexAny                     // ~* 正则，不区分大小写
)

// ParseLocation 拆出匹配方式与其后的路径或正则。nginx 允许操作符与后串之间没有空格
func ParseLocation(location string) (LocationKind, string) {
	location = strings.TrimSpace(location)
	for _, op := range []struct {
		prefix string
		kind   LocationKind
	}{
		{"~*", LocationRegexAny},
		{"~", LocationRegex},
		{"=", LocationExact},
		{"^~", LocationPrefix},
	} {
		if rest, ok := strings.CutPrefix(location, op.prefix); ok {
			return op.kind, strings.TrimSpace(rest)
		}
	}

	return LocationPrefix, location
}

func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

// ProxyOrder 代理规则按 nginx 的 location 优先级排出的下标：精确匹配、^~ 前缀（长者优先）、正则（书写顺序）、普通前缀（长者优先）。
// 按首个命中生效的方言照这个顺序写出规则，匹配结果就不依赖用户添加的先后
func ProxyOrder(proxies []Proxy) []int {
	rank := func(location string) (int, int) {
		switch kind, rest := ParseLocation(location); {
		case kind == LocationExact:
			return 0, 0
		case kind == LocationRegex, kind == LocationRegexAny:
			return 2, 0
		case strings.HasPrefix(strings.TrimSpace(location), "^~"):
			return 1, -len(rest)
		default:
			return 3, -len(rest)
		}
	}
	order := make([]int, len(proxies))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		ra, la := rank(proxies[a].Location)
		rb, lb := rank(proxies[b].Location)
		return cmp.Or(ra-rb, la-lb)
	})
	return order
}

// UnquoteMeta 还原 regexp.QuoteMeta 转义过的字面量
func UnquoteMeta(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
