package request

// ToolboxSSHPort SSH 端口设置
type ToolboxSSHPort struct {
	Port uint `form:"port" json:"port" validate:"required|min:1|max:65535"`
}

// ToolboxSSHPasswordAuth SSH 密码认证设置
type ToolboxSSHPasswordAuth struct {
	Enabled bool `form:"enabled" json:"enabled"`
}

// ToolboxSSHPubKeyAuth SSH 密钥认证设置
type ToolboxSSHPubKeyAuth struct {
	Enabled bool `form:"enabled" json:"enabled"`
}

// ToolboxSSHRootLogin Root 登录设置
type ToolboxSSHRootLogin struct {
	Mode string `form:"mode" json:"mode" validate:"required|in:yes,no,without-password,prohibit-password"`
}

// ToolboxSSHRootPassword Root 密码设置
type ToolboxSSHRootPassword struct {
	Password string `form:"password" json:"password" validate:"required|password"`
}
