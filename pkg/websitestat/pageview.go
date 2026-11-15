package websitestat

import (
	"path"
	"strings"
)

// 静态资源扩展名（非页面浏览）
var staticExts = map[string]struct{}{
	".js": {}, ".css": {}, ".png": {}, ".jpg": {}, ".jpeg": {},
	".gif": {}, ".svg": {}, ".ico": {}, ".woff": {}, ".woff2": {},
	".ttf": {}, ".eot": {}, ".otf": {}, ".map": {}, ".webp": {},
	".avif": {}, ".mp4": {}, ".mp3": {}, ".webm": {}, ".ogg": {},
	".pdf": {}, ".zip": {}, ".gz": {}, ".tar": {}, ".rar": {},
	".7z": {}, ".bz2": {}, ".xz": {}, ".zst": {}, ".swf": {}, ".flv": {},
}

// isStaticResource 判定 URI 是否为静态资源（js/css/图片/字体等）
func isStaticResource(uri string) bool {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	ext := strings.ToLower(path.Ext(uri))
	if ext == "" {
		return false
	}
	_, ok := staticExts[ext]
	return ok
}

// isPageView 判定请求是否为页面浏览（PV）
func isPageView(entry *LogEntry) bool {
	// 优先使用 Content-Type 判定
	if entry.ContentType != "" {
		return strings.Contains(entry.ContentType, "text/html")
	}

	// 回退到 URI 扩展名判定，先去掉 query string
	uri := entry.URI
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	ext := strings.ToLower(path.Ext(uri))
	// 无扩展名视为页面（如 /about, /api/xxx）
	if ext == "" {
		return true
	}
	// 检查是否为已知静态资源
	_, isStatic := staticExts[ext]
	return !isStatic
}
