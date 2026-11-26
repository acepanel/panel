package request

type SettingPanel struct {
	Name           string   `json:"name" validate:"required"`
	Channel        string   `json:"channel" validate:"required && in:stable,beta"`
	Locale         string   `json:"locale" validate:"required"`
	Entrance       string   `json:"entrance" validate:"required && entrance"`
	EntranceError  string   `json:"entrance_error" validate:"in:418,nginx,close"` // 安全入口错误页伪装类型
	LoginCaptcha   bool     `json:"login_captcha"`                                // 登录验证码
	OfflineMode    bool     `json:"offline_mode"`
	AutoUpdate     bool     `json:"auto_update"`
	Lifetime       uint     `json:"lifetime" validate:"required && min:10 && max:43200"` // 登录超时，单位：分
	IPHeader       string   `json:"ip_header"`
	TrustedProxies []string `json:"trusted_proxies" validate:"unique && dive && ipcidr"` // 可以设置 IPHeader 的代理
	BindDomain     []string `json:"bind_domain"`
	BindIP         []string `json:"bind_ip" validate:"unique && dive && ipcidr"`
	BindUA         []string `json:"bind_ua"`
	WebsitePath    string   `json:"website_path" validate:"required"`
	BackupPath     string   `json:"backup_path" validate:"required"`
	BackupFormat   string   `json:"backup_format" validate:"in:tar.xz,tar.gz,tar.zst,zip,7z"` // 备份压缩格式
	ProjectPath    string   `json:"project_path" validate:"required"`
	ContainerSock  string   `json:"container_sock"`
	HiddenMenu     []string `json:"hidden_menu"`                                    // 隐藏的菜单项
	CustomLogo     string   `json:"custom_logo" validate:"url"`                     // 自定义 Logo URL
	IPDBType       string   `json:"ipdb_type" validate:"in:custom,subscribe"`       // IPDB 来源类型: "" / "custom" / "subscribe"
	IPDBURL        string   `json:"ipdb_url"`                                       // IPDB 订阅链接
	IPDBPath       string   `json:"ipdb_path"`                                      // IPDB 地理位置库路径
	Port           uint     `json:"port" validate:"required && min:1 && max:65535"` // 面板端口
	TLS            string   `json:"tls" validate:"in:off,acme,self-signed,custom"`  // 面板 TLS: off, acme, self-signed, custom
	PublicIP       []string `json:"public_ip"`
	Cert           string   `json:"cert"`
	Key            string   `json:"key"`
}

type SettingCert struct {
	Cert string `json:"cert" validate:"required"`
	Key  string `json:"key" validate:"required"`
}

type SettingMemo struct {
	Content string `json:"content"`
}
