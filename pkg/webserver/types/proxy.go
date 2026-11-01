package types

import (
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
)

type CacheConfig struct {
	// 缓存时长，状态码 -> 时长，如: {"200 302": "10m", "404": "1m", "any": "5m"}
	Valid map[string]string `form:"valid" json:"valid"`

	// 不缓存条件 (proxy_cache_bypass + proxy_no_cache)
	// 常用值: "$cookie_nocache", "$arg_nocache", "$http_pragma", "$http_authorization"
	NoCacheConditions []string `form:"no_cache_conditions" json:"no_cache_conditions"`

	// 过期缓存使用策略 (proxy_cache_use_stale)
	// 可选值: "error", "timeout", "updating", "http_500", "http_502", "http_503", "http_504"
	UseStale []string `form:"use_stale" json:"use_stale"`

	// 后台更新 (proxy_cache_background_update)
	BackgroundUpdate bool `form:"background_update" json:"background_update"`

	// 缓存锁 (proxy_cache_lock)，防止缓存击穿
	Lock bool `form:"lock" json:"lock"`

	// 最小请求次数 (proxy_cache_min_uses)，请求 N 次后才缓存
	MinUses int `form:"min_uses" json:"min_uses"`

	// 缓存方法 (proxy_cache_methods)，默认 GET HEAD
	Methods []string `form:"methods" json:"methods"`

	// 自定义缓存键 (proxy_cache_key)
	Key string `form:"key" json:"key"`
}

type TimeoutConfig struct {
	Connect time.Duration `form:"connect" json:"connect"` // proxy_connect_timeout，默认 60s
	Read    time.Duration `form:"read" json:"read"`       // proxy_read_timeout，默认 60s
	Send    time.Duration `form:"send" json:"send"`       // proxy_send_timeout，默认 60s
}

type RetryConfig struct {
	// 触发重试的条件 (proxy_next_upstream)
	// 可选值: "error", "timeout", "invalid_header", "http_500", "http_502", "http_503", "http_504", "http_429", "non_idempotent", "off"
	Conditions []string `form:"conditions" json:"conditions"`

	// 最大重试次数 (proxy_next_upstream_tries)，0 表示不限制
	Tries int `form:"tries" json:"tries"`

	// 重试超时时间 (proxy_next_upstream_timeout)，0 表示不限制
	Timeout time.Duration `form:"timeout" json:"timeout"`
}

type SSLBackendConfig struct {
	Verify             bool   `form:"verify" json:"verify"`                           // proxy_ssl_verify on/off
	TrustedCertificate string `form:"trusted_certificate" json:"trusted_certificate"` // proxy_ssl_trusted_certificate 路径
	VerifyDepth        int    `form:"verify_depth" json:"verify_depth"`               // proxy_ssl_verify_depth，默认 1
}

type ResponseHeaderConfig struct {
	// 隐藏的响应头 (proxy_hide_header)
	Hide []string `form:"hide" json:"hide"`

	// 添加的响应头 (add_header)，key -> value
	// 值可以包含变量，如 $upstream_cache_status
	Add map[string]string `form:"add" json:"add"`
}

type AccessControlConfig struct {
	// 允许的 IP/CIDR 列表 (allow)
	Allow []string `form:"allow" json:"allow"`

	// 拒绝的 IP/CIDR 列表 (deny)
	Deny []string `form:"deny" json:"deny"`
}

type Proxy struct {
	Location          string                `form:"location" json:"location" validate:"required"`             // 匹配路径，如: "/", "/api", "~ ^/api/v[0-9]+/"
	Pass              string                `form:"pass" json:"pass" validate:"required"`                     // 代理地址，如: "http://example.com", "http://backend"
	Host              string                `form:"host" json:"host"`                                         // 代理 Host，如: "example.com"
	SNI               string                `form:"sni" json:"sni"`                                           // 代理 SNI，如: "example.com"
	Cache             *CacheConfig          `form:"cache" json:"cache"`                                       // 缓存配置，nil 表示禁用缓存
	Buffering         bool                  `form:"buffering" json:"buffering"`                               // 是否启用缓冲
	Resolver          []string              `form:"resolver" json:"resolver"`                                 // 自定义 DNS 解析器配置，如: ["8.8.8.8", "ipv6=off"]
	ResolverTimeout   time.Duration         `form:"resolver_timeout" json:"resolver_timeout"`                 // DNS 解析超时时间，如: 5 * time.Second
	Headers           map[string]string     `form:"headers" json:"headers"`                                   // 自定义请求头，如: map["X-Custom-Header"] = "value"
	Replaces          map[string]string     `form:"replaces" json:"replaces"`                                 // 响应内容替换，如: map["/old"] = "/new"
	HTTPVersion       string                `form:"http_version" json:"http_version" validate:"in:1.0,1.1,2"` // HTTP 协议版本 (proxy_http_version)，可选: "1.0", "1.1", "2"
	Timeout           *TimeoutConfig        `form:"timeout" json:"timeout"`                                   // 超时配置
	Retry             *RetryConfig          `form:"retry" json:"retry"`                                       // 重试配置
	ClientMaxBodySize int64                 `form:"client_max_body_size" json:"client_max_body_size"`         // 请求体大小限制 (client_max_body_size)，单位字节，0 表示使用全局配置
	SSLBackend        *SSLBackendConfig     `form:"ssl_backend" json:"ssl_backend"`                           // SSL 后端验证配置
	ResponseHeaders   *ResponseHeaderConfig `form:"response_headers" json:"response_headers"`                 // 响应头修改配置
	AccessControl     *AccessControlConfig  `form:"access_control" json:"access_control"`                     // IP 访问控制配置
	Custom            string                `form:"custom" json:"custom"`                                     // 自定义指令，当前 Web 服务器的原生语法，追加到该规则作用域的末尾，切换服务器时不迁移
}

// DefaultProxyHost 按代理目标给出默认的 Host。本机、IP、unix 套接字与上游都是站点自己的应用，
// 要发访客的域名，否则应用生成的链接、CSRF 与 WebSocket 的 Origin 校验都会出错；
// 外部域名靠 Host 选虚拟主机，必须发目标自己的域名
func DefaultProxyHost(pass string, upstreams []Upstream) string {
	u, err := url.Parse(pass)
	if err != nil {
		return "$host"
	}
	host := u.Hostname()
	upstream := slices.ContainsFunc(upstreams, func(up Upstream) bool { return strings.EqualFold(up.Name, host) })
	if upstream || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return "$host"
	}
	return "$proxy_host"
}

// 负载均衡算法的规范值，取 nginx 写法。各方言写入时映射到自己的语法，
// 没有对应实现的降级为轮询并把原值留在注释里，换服务器时不丢
const (
	AlgoRoundRobin = ""           // 轮询
	AlgoLeastConn  = "least_conn" // 最少连接
	AlgoIPHash     = "ip_hash"    // 按客户端 IP
	AlgoRandom     = "random"     // 随机
)

// NormalizeAlgo 收敛到规范算法名，无法识别的按轮询处理
func NormalizeAlgo(algo string) string {
	switch algo {
	case AlgoLeastConn, AlgoIPHash, AlgoRandom:
		return algo
	default:
		return AlgoRoundRobin
	}
}

type Upstream struct {
	Name            string            `form:"name" json:"name" validate:"required"`       // 上游名称，如: "backend"
	Servers         map[string]string `form:"servers" json:"servers" validate:"required"` // 上游服务器及配置，如: map["server1"] = "weight=5 resolve"
	Algo            string            `form:"algo" json:"algo"`                           // 负载均衡算法，如: "least_conn", "ip_hash"
	Keepalive       int               `form:"keepalive" json:"keepalive"`                 // 保持连接数，如: 32
	Resolver        []string          `form:"resolver" json:"resolver"`                   // 自定义 DNS 解析器配置，如: ["8.8.8.8", "ipv6=off"]
	ResolverTimeout time.Duration     `form:"resolver_timeout" json:"resolver_timeout"`   // DNS 解析超时时间，如: 5 * time.Second
}
