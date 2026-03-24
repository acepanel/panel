package grafana

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

// ConfigTune Grafana 配置调整
type ConfigTune struct {
	// [server]
	HTTPPort string `form:"http_port" json:"http_port"`
	Domain   string `form:"domain" json:"domain"`
	RootURL  string `form:"root_url" json:"root_url"`
	Protocol string `form:"protocol" json:"protocol"`
	// [database]
	DBType     string `form:"db_type" json:"db_type"`
	DBHost     string `form:"db_host" json:"db_host"`
	DBName     string `form:"db_name" json:"db_name"`
	DBUser     string `form:"db_user" json:"db_user"`
	DBPassword string `form:"db_password" json:"db_password"`
	// [security]
	AdminUser     string `form:"admin_user" json:"admin_user"`
	AdminPassword string `form:"admin_password" json:"admin_password"`
	// [users]
	AllowSignUp       string `form:"allow_sign_up" json:"allow_sign_up"`
	AutoAssignOrgRole string `form:"auto_assign_org_role" json:"auto_assign_org_role"`
	// [smtp]
	SMTPEnabled     string `form:"smtp_enabled" json:"smtp_enabled"`
	SMTPHost        string `form:"smtp_host" json:"smtp_host"`
	SMTPUser        string `form:"smtp_user" json:"smtp_user"`
	SMTPPassword    string `form:"smtp_password" json:"smtp_password"`
	SMTPFromAddress string `form:"smtp_from_address" json:"smtp_from_address"`
	// [log]
	LogMode  string `form:"log_mode" json:"log_mode"`
	LogLevel string `form:"log_level" json:"log_level"`
}
