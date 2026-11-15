package openlitespeed

// SetPHP 切换 PHP 版本运行协议
type SetPHP struct {
	Version uint `form:"version" json:"version" validate:"required"`
	LSAPI   bool `form:"lsapi" json:"lsapi"`
}

// SetRealIP 服务器级真实 IP 配置
type SetRealIP struct {
	Enabled bool     `form:"enabled" json:"enabled"`
	Trusted []string `form:"trusted" json:"trusted" validate:"unique && dive && ipcidr"`
}

// PHPProtocol PHP 版本运行协议信息
type PHPProtocol struct {
	Version uint `json:"version"`
	LSPHP   bool `json:"lsphp"` // lsphp 二进制是否存在
	LSAPI   bool `json:"lsapi"` // 是否以 LSAPI 协议运行
}
