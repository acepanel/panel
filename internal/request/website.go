package request

import (
	paneltypes "github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

type WebsiteDefaultConfig struct {
	Index       string   `json:"index" form:"index" validate:"required"`
	Stop        string   `json:"stop" form:"stop" validate:"required"`
	NotFound    string   `json:"not_found" form:"not_found"`
	TLSVersions []string `json:"tls_versions" form:"tls_versions" validate:"required && unique && dive && required"`
	ListenIPv6  bool     `json:"listen_ipv6" form:"listen_ipv6"`
}

type WebsiteList struct {
	Type      string `json:"type" form:"type" validate:"required && in:all,proxy,static,php"`
	Keyword   string `json:"keyword" form:"keyword" query:"keyword"`
	ProjectID uint   `json:"project_id" form:"project_id" query:"project_id"` // 只看绑定到该项目的网站，0 为不限
	Paginate
}

type WebsiteCreate struct {
	Type       string   `json:"type" form:"type" validate:"required && in:proxy,static,php"`
	Name       string   `form:"name" json:"name" validate:"required && not_exists:websites,name && not_in:phpmyadmin,default && regex:\"^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9._-]{0,126}[A-Za-z0-9_-])$\""`
	Listens    []string `form:"listens" json:"listens" validate:"required && unique && dive && required"`
	Domains    []string `form:"domains" json:"domains" validate:"required && unique && dive && required"`
	Path       string   `form:"path" json:"path"`
	DB         bool     `form:"db" json:"db"`
	DBType     string   `form:"db_type" json:"db_type" validate:"required_if:DB,true"`
	DBName     string   `form:"db_name" json:"db_name" validate:"required_if:DB,true && regex:\"^[A-Za-z0-9_.-]{1,64}$\""`
	DBUser     string   `form:"db_user" json:"db_user" validate:"required_if:DB,true && regex:\"^[A-Za-z0-9_.-]{1,63}$\""`
	DBPassword string   `form:"db_password" json:"db_password" validate:"required_if:DB,true"`
	Remark     string   `form:"remark" json:"remark"`

	PHP       uint   `form:"php" json:"php"`                                             // 仅 PHP 网站需要，0 为不使用 PHP
	Proxy     string `form:"proxy" json:"proxy" validate:"required_if:Type,proxy"`       // 仅反向代理网站需要
	ProjectID uint   `form:"project_id" json:"project_id" validate:"exists:projects,id"` // 绑定的项目，0 为不绑定
}

type WebsiteDelete struct {
	ID   uint `form:"id" json:"id" validate:"required && exists:websites,id"`
	Path bool `form:"path" json:"path"`
	DB   bool `form:"db" json:"db"`
}

type WebsiteUpdate struct {
	ID      uint           `form:"id" json:"id" validate:"required && exists:websites,id"`
	Listens []types.Listen `form:"listens" json:"listens" validate:"required && dive && required"`
	Domains []string       `form:"domains" json:"domains" validate:"required && unique && dive && required"`
	Path    string         `form:"path" json:"path" validate:"required && unix_path"` // 网站目录
	Root    string         `form:"root" json:"root" validate:"required && unix_path"` // 运行目录
	Index   []string       `form:"index" json:"index" validate:"required && unique && dive && required"`

	// SSL 相关
	SSL          bool     `form:"ssl" json:"ssl"`
	SSLCert      string   `json:"ssl_cert"`
	SSLKey       string   `json:"ssl_key"`
	HSTS         bool     `form:"hsts" json:"hsts"`
	OCSP         bool     `form:"ocsp" json:"ocsp"`
	HTTPRedirect bool     `form:"http_redirect" json:"http_redirect"`
	SSLProtocols []string `json:"ssl_protocols"`

	// PHP 相关
	PHP         uint    `form:"php" json:"php"`
	Rewrite     *string `form:"rewrite" json:"rewrite"` // 不传则不动现有规则
	OpenBasedir bool    `form:"open_basedir" json:"open_basedir"`

	// 反向代理
	Upstreams []types.Upstream `json:"upstreams"`
	Proxies   []types.Proxy    `json:"proxies"`

	// 重定向
	Redirects []types.Redirect `json:"redirects"`

	// 高级设置
	StatEnabled bool                          `json:"stat_enabled"` // 是否启用访问统计
	LSCache     bool                          `json:"lscache"`      // LiteSpeed 页面缓存
	AccessLog   string                        `json:"access_log"`   // 访问日志路径
	ErrorLog    string                        `json:"error_log"`    // 错误日志路径
	RateLimit   *types.RateLimit              `json:"rate_limit"`   // 限流限速配置
	RealIP      *types.RealIP                 `json:"real_ip"`      // 真实 IP 配置
	BasicAuth   []paneltypes.WebsiteBasicAuth `json:"basic_auth"`   // 基本认证规则

	// 自定义配置
	CustomConfigs []WebsiteCustomConfig `json:"custom_configs"`
}

type WebsiteSwitchType struct {
	ID    uint   `form:"id" json:"id" uri:"id" validate:"required && exists:websites,id"`
	Type  string `form:"type" json:"type" validate:"required && in:proxy,static,php"`
	PHP   uint   `form:"php" json:"php"` // 0 为不使用 PHP
	Proxy string `form:"proxy" json:"proxy" validate:"required_if:Type,proxy"`
}

// WebsiteCustomConfig 网站自定义配置请求
type WebsiteCustomConfig struct {
	Name    string `json:"name" validate:"required && regex:\"^[a-zA-Z0-9_-]+$\""` // 配置名称
	Scope   string `json:"scope" validate:"required && in:site,shared"`            // 作用域: site(此网站), shared(全局)
	Content string `json:"content"`                                                // 配置内容
}

type WebsiteUpdateRemark struct {
	ID     uint   `form:"id" json:"id" validate:"required && exists:websites,id"`
	Remark string `form:"remark" json:"remark"`
}

type WebsiteUpdateProject struct {
	ID        uint `form:"id" json:"id" validate:"required && exists:websites,id"`
	ProjectID uint `form:"project_id" json:"project_id" validate:"exists:projects,id"` // 0 为解除绑定
}

type WebsiteUpdateStatus struct {
	ID     uint `json:"id" form:"id" validate:"required && exists:websites,id"`
	Status bool `json:"status" form:"status"`
}

type WebsiteUpdateExpireAt struct {
	ID       uint   `json:"id" form:"id" validate:"required && exists:websites,id"`
	ExpireAt string `json:"expire_at" form:"expire_at"` // 为空表示清除到期时间（不限时）
}

type WebsiteUpdateCert struct {
	Name string `json:"name" validate:"required && exists:websites,name && regex:\"^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9._-]{0,126}[A-Za-z0-9_-])$\""`
	Cert string `json:"cert" validate:"required"`
	Key  string `json:"key" validate:"required"`
}

type WebsiteObtainCert struct {
	ID    uint `json:"id" form:"id" uri:"id" validate:"required && exists:websites,id"`
	DNSID uint `json:"dns_id" form:"dns_id"`
}
