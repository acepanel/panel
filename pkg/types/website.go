package types

import "github.com/acepanel/panel/pkg/webserver/types"

// WebsiteListen 网站监听配置
type WebsiteListen struct {
	Address string `form:"address" json:"address" validate:"required"` // 监听地址 e.g. 80 0.0.0.0:80 [::]:80
	HTTPS   bool   `form:"https" json:"https"`                         // 是否启用HTTPS
	QUIC    bool   `form:"quic" json:"quic"`                           // 是否启用QUIC
}

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
	SSLCiphers    string   `json:"ssl_ciphers"`
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
	RateLimit *types.RateLimit  `json:"rate_limit"` // 限流限速配置
	BasicAuth map[string]string `json:"basic_auth"` // 基本认证配置

	// 自定义配置
	CustomConfigs []WebsiteCustomConfig `json:"custom_configs"`
}

// WebsiteCustomConfig 网站自定义配置
type WebsiteCustomConfig struct {
	Name    string `json:"name"`    // 配置名称
	Scope   string `json:"scope"`   // 作用域: site(此网站), shared(全局)
	Content string `json:"content"` // 配置内容
}
