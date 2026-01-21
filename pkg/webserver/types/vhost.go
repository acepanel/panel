package types

// Vhost 虚拟主机通用接口
type Vhost interface {
	// ========== 核心方法 ==========

	// Enable 取启用状态
	Enable() bool
	// SetEnable 设置启用状态
	SetEnable(enable bool) error

	// Listen 取监听配置
	Listen() []Listen
	// SetListen 设置监听配置
	SetListen(listen []Listen) error

	// ServerName 取服务器名称，如: ["example.com", "www.example.com"]
	ServerName() []string
	// SetServerName 设置服务器名称
	SetServerName(serverName []string) error

	// Index 取默认首页，如: ["index.php", "index.html"]
	Index() []string
	// SetIndex 设置默认首页
	SetIndex(index []string) error

	// Root 取网站根目录，如: "/opt/ace/sites/example/public"
	Root() string
	// SetRoot 设置网站根目录
	SetRoot(root string) error

	// Includes 取包含的文件配置
	Includes() []IncludeFile
	// SetIncludes 设置包含的文件配置
	SetIncludes(includes []IncludeFile) error

	// AccessLog 取访问日志路径，如: "/opt/ace/sites/example/log/access.log"
	AccessLog() string
	// SetAccessLog 设置访问日志路径
	SetAccessLog(accessLog string) error

	// ErrorLog 取错误日志路径，如: "/opt/ace/sites/example/log/error.log"
	ErrorLog() string
	// SetErrorLog 设置错误日志路径
	SetErrorLog(errorLog string) error

	// Save 保存配置到文件
	Save() error
	// Reset 重置配置为默认值
	Reset() error

	// ========== SSL/TLS 方法 ==========

	// SSL 取 SSL 启用状态
	SSL() bool
	// SSLConfig 取 SSL 配置
	SSLConfig() *SSLConfig
	// SetSSLConfig 设置 SSL 配置（自动启用 HTTPS）
	SetSSLConfig(cfg *SSLConfig) error
	// ClearSSL 清除 SSL 配置
	ClearSSL() error

	// ========== 高级功能方法 ==========

	// RateLimit 取限流限速配置
	RateLimit() *RateLimit
	// SetRateLimit 设置限流限速配置
	SetRateLimit(limit *RateLimit) error
	// ClearRateLimit 清除限流限速配置
	ClearRateLimit() error

	// BasicAuth 取基本认证配置
	BasicAuth() map[string]string
	// SetBasicAuth 设置基本认证
	SetBasicAuth(auth map[string]string) error
	// ClearBasicAuth 清除基本认证
	ClearBasicAuth() error

	// Config 取指定名称的配置内容
	// type 可选值: "site", "shared"
	Config(name string, typ string) string
	// SetConfig 设置指定名称的配置内容
	// type 可选值: "site", "shared"
	SetConfig(name string, typ string, content string) error
	// RemoveConfig 清除指定名称的配置内容
	// type 可选值: "site", "shared"
	RemoveConfig(name string, typ string) error
}

// StaticVhost 纯静态虚拟主机接口
type StaticVhost interface {
	Vhost
	VhostRedirect
}

// PHPVhost PHP 虚拟主机接口
type PHPVhost interface {
	Vhost
	VhostPHP
	VhostRedirect
}

// ProxyVhost 反向代理虚拟主机接口
type ProxyVhost interface {
	Vhost
	VhostRedirect
	VhostProxy
}

// VhostPHP PHP 相关接口
type VhostPHP interface {
	// PHP 取 PHP 版本，如: 84, 81, 80, 0 表示未启用 PHP
	PHP() uint
	// SetPHP 设置 PHP 版本
	SetPHP(version uint) error
}

// VhostRedirect 重定向相关接口
type VhostRedirect interface {
	// Redirects 取所有重定向配置
	Redirects() []Redirect
	// SetRedirects 设置重定向
	SetRedirects(redirects []Redirect) error
}

// VhostProxy 反向代理相关接口
type VhostProxy interface {
	// Proxies 取所有反向代理配置
	Proxies() []Proxy
	// SetProxies 设置反向代理配置
	SetProxies(proxies []Proxy) error
	// ClearProxies 清除所有反向代理配置
	ClearProxies() error

	// Upstreams 取上游服务器配置
	Upstreams() []Upstream
	// SetUpstreams 设置上游服务器配置
	SetUpstreams(upstreams []Upstream) error
	// ClearUpstreams 清除所有上游服务器配置
	ClearUpstreams() error
}

// Listen 监听配置
type Listen struct {
	Address string   `form:"address" json:"address"` // 监听地址，如: "80", "0.0.0.0:80", "[::]:443"
	Args    []string `form:"args" json:"args"`       // 其他参数，如: ["default_server", "ssl", "quic"]
}

// SSLConfig SSL/TLS 配置
type SSLConfig struct {
	Cert      string   `json:"cert"`      // 证书路径
	Key       string   `json:"key"`       // 私钥路径
	Protocols []string `json:"protocols"` // 支持的协议，如: ["TLSv1.2", "TLSv1.3"]
	Ciphers   string   `json:"ciphers"`   // 加密套件

	// 高级选项
	HSTS         bool   `json:"hsts"`          // HTTP 严格传输安全
	OCSP         bool   `json:"ocsp"`          // OCSP Stapling
	HTTPRedirect bool   `json:"http_redirect"` // HTTP 强制跳转 HTTPS
	AltSvc       string `json:"alt_svc"`       // Alt-Svc 配置，如: 'h3=":443"; ma=86400'
}

// RateLimit 限流限速配置
type RateLimit struct {
	PerServer int `json:"per_server"` // 站点最大并发数 (limit_conn perserver X)
	PerIP     int `json:"per_ip"`     // 单 IP 最大并发数 (limit_conn perip X)
	Rate      int `json:"rate"`       // 流量限制，单位 KB (limit_rate Xk)
}

// IncludeFile 包含文件配置
type IncludeFile struct {
	Path    string   `json:"path"`    // 文件路径
	Comment []string `json:"comment"` // 注释说明
}
