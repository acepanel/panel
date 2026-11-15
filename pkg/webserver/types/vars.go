package types

import "strings"

// ApacheVars nginx 变量到 Apache 表达式的映射。表首命中即回读时的规范原像，所以 $host 要排在 $http_host 之前。
// OpenLiteSpeed 的 extraHeaders 用的也是 Apache 指令语法，两者共用
var ApacheVars = NewVarMap([][2]string{
	{"$proxy_add_x_forwarded_for", "%{HTTP:X-Forwarded-For}"},
	{"$remote_addr", "%{REMOTE_ADDR}"},
	{"$host", "%{HTTP_HOST}"},
	{"$http_host", "%{HTTP_HOST}"},
	{"$request_uri", "%{REQUEST_URI}"},
	{"$server_name", "%{SERVER_NAME}"},
	{"$server_port", "%{SERVER_PORT}"},
	{"$scheme", "%{REQUEST_SCHEME}"},
})

// VarMap nginx 变量与某个方言的占位符之间的双向映射。面板的头部值统一存 nginx 写法，
// 照搬到别的方言会被当字面量发给上游，所以写入时换成该方言的占位符、回读时换回来。
// 多个 nginx 变量映射到同一个占位符时，回读取表中首个，所以最常用的写法要排在前面
type VarMap struct {
	toNative *strings.Replacer
	toNginx  *strings.Replacer
}

func NewVarMap(pairs [][2]string) *VarMap {
	native := make([]string, 0, len(pairs)*2)
	nginx := make([]string, 0, len(pairs)*2)
	seen := make(map[string]bool, len(pairs))
	for _, p := range pairs {
		native = append(native, p[0], p[1])
		if !seen[p[1]] {
			seen[p[1]] = true
			nginx = append(nginx, p[1], p[0])
		}
	}

	return &VarMap{toNative: strings.NewReplacer(native...), toNginx: strings.NewReplacer(nginx...)}
}

func (m *VarMap) ToNative(value string) string {
	return m.toNative.Replace(value)
}

func (m *VarMap) ToNginx(value string) string {
	return m.toNginx.Replace(value)
}

// ApacheHeaderValue 含变量的头值要写成 expr 形式，Apache 与 OLS 的 extraHeaders 才会求值，照搬 nginx 变量会当字面量发出去
func ApacheHeaderValue(value string) string {
	native := ApacheVars.ToNative(value)
	if native == value {
		return value
	}
	return "expr=" + native
}

func ParseApacheHeaderValue(value string) string {
	expr, ok := strings.CutPrefix(value, "expr=")
	if !ok {
		return value
	}
	return ApacheVars.ToNginx(expr)
}
