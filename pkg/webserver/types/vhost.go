package types

type ConfigScope string

const (
	ScopeSite   ConfigScope = "site"   // 站点级片段（site/ 目录）
	ScopeShared ConfigScope = "shared" // 共享级片段（shared/ 目录）
)

type Vhost interface {
	Enable() bool
	SetEnable(enable bool) error

	// Default 是否为默认站点，承接未匹配任何域名的请求
	Default() bool
	SetDefault(enable bool) error

	Listen() []Listen
	SetListen(listen []Listen) error

	// ServerName 取服务器名称，如: ["example.com", "www.example.com"]
	ServerName() []string
	SetServerName(serverName []string) error

	// Index 取默认首页，如: ["index.php", "index.html"]
	Index() []string
	SetIndex(index []string) error

	// Root 取网站根目录，如: "/opt/ace/sites/example/public"
	Root() string
	SetRoot(root string) error

	Includes() []IncludeFile
	SetIncludes(includes []IncludeFile) error

	// AccessLog 取访问日志路径，如: "/opt/ace/sites/example/log/access.log"
	AccessLog() string
	SetAccessLog(accessLog string) error

	// ErrorLog 取错误日志路径，如: "/opt/ace/sites/example/log/error.log"
	ErrorLog() string
	SetErrorLog(errorLog string) error

	Save() error
	Reset() error

	SSL() bool
	SSLConfig() *SSLConfig
	SetSSLConfig(cfg *SSLConfig) error
	ClearSSL() error

	RateLimit() *RateLimit
	SetRateLimit(limit *RateLimit) error
	ClearRateLimit() error

	BasicAuth() []BasicAuth
	SetBasicAuth(auths []BasicAuth) error
	ClearBasicAuth() error

	RealIP() *RealIP
	SetRealIP(realIP *RealIP) error
	ClearRealIP() error

	Config(name string, scope ConfigScope) string
	// SetConfig 设置指定名称的配置内容，自动添加生成标记注释
	SetConfig(name string, scope ConfigScope, content string) error
	// SetRawConfig 设置配置内容但不添加生成标记注释（用于用户可编辑的片段）
	SetRawConfig(name string, scope ConfigScope, content string) error
	RemoveConfig(name string, scope ConfigScope) error
}

type StaticVhost interface {
	Vhost
	VhostRedirect
	VhostProxy
}

type PHPVhost interface {
	Vhost
	VhostPHP
	VhostRedirect
	VhostProxy
}

type ProxyVhost interface {
	Vhost
	VhostRedirect
	VhostProxy
}

type VhostPHP interface {
	// PHP 取 PHP 版本，如: 84, 81, 80, 0 表示未启用 PHP
	PHP() uint
	SetPHP(version uint) error
}

type VhostRedirect interface {
	Redirects() []Redirect
	SetRedirects(redirects []Redirect) error
}

// VhostProxy 反向代理规则与上游，静态与 PHP 站点也能用它把子路径（如 /api）交给后端
type VhostProxy interface {
	Proxies() []Proxy
	SetProxies(proxies []Proxy) error
	ClearProxies() error

	Upstreams() []Upstream
	SetUpstreams(upstreams []Upstream) error
	ClearUpstreams() error
}

type Listen struct {
	Address string   `form:"address" json:"address"` // 监听地址，如: "80", "0.0.0.0:80", "[::]:443"
	Args    []string `form:"args" json:"args"`       // 其他参数，如: ["default_server", "ssl", "quic"]
}

type SSLConfig struct {
	Cert      string   `json:"cert"`      // 证书路径
	Key       string   `json:"key"`       // 私钥路径
	Protocols []string `json:"protocols"` // 支持的协议，如: ["TLSv1.2", "TLSv1.3"]

	// 高级选项
	HSTS         bool   `json:"hsts"`          // HTTP 严格传输安全
	OCSP         bool   `json:"ocsp"`          // OCSP Stapling
	HTTPRedirect bool   `json:"http_redirect"` // HTTP 强制跳转 HTTPS
	AltSvc       string `json:"alt_svc"`       // Alt-Svc 配置，如: 'h3=":443"; ma=86400'
}

type RateLimit struct {
	PerServer int `json:"per_server"` // 站点最大并发数 (limit_conn perserver X)
	PerIP     int `json:"per_ip"`     // 单 IP 最大并发数 (limit_conn perip X)
	Rate      int `json:"rate"`       // 流量限制，单位 KB (limit_rate Xk)
}

type BasicAuth struct {
	Path     string `json:"path"`      // 生效路径前缀，"/" 表示整站
	UserFile string `json:"user_file"` // htpasswd 文件路径
}

type RealIP struct {
	From      []string `json:"from"`      // 可信 IP 来源列表 (set_real_ip_from)
	Header    string   `json:"header"`    // 真实 IP 头 (real_ip_header)，如: X-Real-IP, X-Forwarded-For
	Recursive bool     `json:"recursive"` // 递归搜索 (real_ip_recursive)
}

type IncludeFile struct {
	Path    string   `json:"path"`    // 文件路径
	Comment []string `json:"comment"` // 注释说明
}
