package websitestat

import (
	"strconv"

	"github.com/valyala/fastjson"
)

var parserPool fastjson.ParserPool

// parseSyslog 从 syslog 消息中提取 tag 和 JSON 体
// 格式: <PRI>MMM DD HH:MM:SS tag: JSON（nohostname 模式下无主机名）；Caddy 直接发 JSON，没有 tag
func parseSyslog(msg []byte) (string, []byte) {
	n := len(msg)
	if n == 0 {
		return "", nil
	}
	if msg[0] == '{' {
		return "", msg
	}

	i := 0
	// 跳过 PRI: <xxx>
	if msg[0] == '<' {
		for i < n && msg[i] != '>' {
			i++
		}
		if i < n {
			i++ // 跳过 '>'
		}
	}

	// 找到 ": " 分隔符，tag 在其前面的最后一个空格之后
	colonIdx := -1
	for j := i; j < n-1; j++ {
		if msg[j] == ':' && msg[j+1] == ' ' {
			colonIdx = j
			break
		}
	}
	if colonIdx < 0 {
		return "", nil
	}

	// tag 是 ": " 前面最后一个空格之后的部分
	tagStart := i
	for j := colonIdx - 1; j >= i; j-- {
		if msg[j] == ' ' {
			tagStart = j + 1
			break
		}
	}

	tag := string(msg[tagStart:colonIdx])
	data := msg[colonIdx+2:] // 跳过 ": "

	return tag, data
}

// ParseLogEntry 将 JSON 数据解析为 LogEntry
func ParseLogEntry(tag string, data []byte) (*LogEntry, error) {
	p := parserPool.Get()
	defer parserPool.Put(p)

	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	status := getInt(v, "status")

	site := getString(v, "site")
	if site == "" {
		site = tag
	}

	// Caddy 的访问日志：请求信息在 request 对象里，站点名由 append 编码器追加，没有请求体
	if req := v.Get("request"); req != nil {
		return &LogEntry{
			Site:        site,
			URI:         getString(req, "uri"),
			Status:      int(status),
			Bytes:       getUint(v, "size"),
			UA:          firstHeader(req.Get("headers"), "User-Agent"),
			IP:          getString(req, "remote_ip"),
			Method:      getString(req, "method"),
			ContentType: firstHeader(v.Get("resp_headers"), "Content-Type"),
			ReqLength:   getUint(v, "bytes_read"),
			RequestTime: getFloat(v, "duration"),
		}, nil
	}

	entry := &LogEntry{
		Site:        site,
		URI:         getString(v, "uri"),
		Status:      int(status),
		Bytes:       getUint(v, "bytes"),
		UA:          getString(v, "ua"),
		IP:          getString(v, "ip"),
		Method:      getString(v, "method"),
		ContentType: getString(v, "content_type"),
		ReqLength:   getUint(v, "req_length"),
		RequestTime: getFloat(v, "rt"),
	}

	// 仅 4xx/5xx 时才提取 body
	if status >= 400 && status < 600 {
		entry.Body = getString(v, "body")
	}

	return entry, nil
}

// getString 从 JSON value 中提取字符串字段
func getString(v *fastjson.Value, key string) string {
	return string(v.GetStringBytes(key))
}

// getInt 从 JSON value 中提取数值字段
func getInt(v *fastjson.Value, key string) int64 {
	val := v.Get(key)
	if val == nil {
		return 0
	}
	switch val.Type() {
	case fastjson.TypeNumber:
		return val.GetInt64()
	case fastjson.TypeString:
		n, _ := strconv.ParseInt(string(val.GetStringBytes()), 10, 64)
		return n
	default:
		return 0
	}
}

// getUint 从 JSON value 中提取字节数字段
func getUint(v *fastjson.Value, key string) uint64 {
	n := getInt(v, key)
	if n < 0 {
		return 0
	}
	return uint64(n)
}

// getFloat 从 JSON value 中提取浮点数字段
func getFloat(v *fastjson.Value, key string) float64 {
	val := v.Get(key)
	if val == nil {
		return 0
	}
	switch val.Type() {
	case fastjson.TypeNumber:
		return val.GetFloat64()
	case fastjson.TypeString:
		f, _ := strconv.ParseFloat(string(val.GetStringBytes()), 64)
		return f
	default:
		return 0
	}
}

// firstHeader 取头部数组的首个值，Caddy 把每个头记为字符串数组
func firstHeader(headers *fastjson.Value, key string) string {
	if headers == nil {
		return ""
	}
	if values := headers.GetArray(key); len(values) > 0 {
		return string(values[0].GetStringBytes())
	}
	return ""
}
