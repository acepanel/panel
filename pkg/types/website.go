package types

import "github.com/acepanel/panel/v3/pkg/webserver/types"

// WebsiteSetting 网站设置
type WebsiteSetting struct {
	ID      uint           `json:"id"`
	Name    string         `json:"name"`
	Type    string         `json:"type"`
	Listens []types.Listen `form:"listens" json:"listens" validate:"required"`
	Domains []string       `json:"domains"`
	Path    string         `json:"path"` // 网站目录
	Root    string         `json:"root"` // 运行目录
	Index   []string       `json:"index"`

	// SSL 相关
	SSL           bool     `json:"ssl"`
	SSLCert       string   `json:"ssl_cert"`
	SSLKey        string   `json:"ssl_key"`
	HSTS          bool     `json:"hsts"`
	OCSP          bool     `json:"ocsp"`
	HTTPRedirect  bool     `json:"http_redirect"`
	SSLProtocols  []string `json:"ssl_protocols"`
	SSLNotBefore  string   `json:"ssl_not_before"`
	SSLNotAfter   string   `json:"ssl_not_after"`
	SSLDNSNames   []string `json:"ssl_dns_names"`
	SSLIssuer     string   `json:"ssl_issuer"`
	SSLOCSPServer []string `json:"ssl_ocsp_server"`

	AccessLog string `json:"access_log"`
	ErrorLog  string `json:"error_log"`

	// PHP 相关
	PHP         uint   `json:"php"`
	Rewrite     string `json:"rewrite"`
	OpenBasedir bool   `json:"open_basedir"`

	// 反向代理
	Upstreams []types.Upstream `json:"upstreams"`
	Proxies   []types.Proxy    `json:"proxies"`

	// 重定向
	Redirects []types.Redirect `json:"redirects"`

	// 高级设置
	StatEnabled bool               `json:"stat_enabled"` // 是否启用访问统计
	LSCache     bool               `json:"lscache"`      // LiteSpeed 页面缓存
	RateLimit   *types.RateLimit   `json:"rate_limit"`   // 限流限速配置
	RealIP      *types.RealIP      `json:"real_ip"`      // 真实 IP 配置
	BasicAuth   []WebsiteBasicAuth `json:"basic_auth"`   // 基本认证规则

	// 自定义配置
	CustomConfigs []WebsiteCustomConfig `json:"custom_configs"`
}

// WebsiteBasicAuth 网站基本认证规则
type WebsiteBasicAuth struct {
	Path  string            `json:"path"`  // 生效路径，空或 "/" 表示整站
	Users map[string]string `json:"users"` // 用户名 -> 密码
}

// WebsiteCustomConfig 网站自定义配置
type WebsiteCustomConfig struct {
	Name    string `json:"name"`    // 配置名称
	Scope   string `json:"scope"`   // 作用域: site(此网站), shared(全局)
	Content string `json:"content"` // 配置内容
}
