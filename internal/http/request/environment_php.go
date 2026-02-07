package request

type EnvironmentPHPVersion struct {
	Version uint `json:"version"`
}

type EnvironmentPHPModule struct {
	Version uint   `json:"version"`
	Slug    string `form:"slug" json:"slug" validate:"required"`
}

type EnvironmentPHPUpdateConfig struct {
	Version uint   `json:"version"`
	Config  string `form:"config" json:"config" validate:"required"`
}

// EnvironmentPHPConfigTune PHP 配置调整
type EnvironmentPHPConfigTune struct {
	Version uint `json:"version"`
	// php.ini 常规设置
	ShortOpenTag   string `form:"short_open_tag" json:"short_open_tag"`
	DateTimezone   string `form:"date_timezone" json:"date_timezone"`
	DisplayErrors  string `form:"display_errors" json:"display_errors"`
	ErrorReporting string `form:"error_reporting" json:"error_reporting"`
	// php.ini 禁用函数
	DisableFunctions string `form:"disable_functions" json:"disable_functions"`
	// php.ini 上传限制
	UploadMaxFilesize string `form:"upload_max_filesize" json:"upload_max_filesize"`
	PostMaxSize       string `form:"post_max_size" json:"post_max_size"`
	MaxFileUploads    string `form:"max_file_uploads" json:"max_file_uploads"`
	MemoryLimit       string `form:"memory_limit" json:"memory_limit"`
	// php.ini 超时限制
	MaxExecutionTime string `form:"max_execution_time" json:"max_execution_time"`
	MaxInputTime     string `form:"max_input_time" json:"max_input_time"`
	MaxInputVars     string `form:"max_input_vars" json:"max_input_vars"`
	// php.ini Session 相关
	SessionSaveHandler    string `form:"session_save_handler" json:"session_save_handler"`
	SessionSavePath       string `form:"session_save_path" json:"session_save_path"`
	SessionGcMaxlifetime  string `form:"session_gc_maxlifetime" json:"session_gc_maxlifetime"`
	SessionCookieLifetime string `form:"session_cookie_lifetime" json:"session_cookie_lifetime"`
	// php-fpm.conf 相关
	Pm                string `form:"pm" json:"pm"`
	PmMaxChildren     string `form:"pm_max_children" json:"pm_max_children"`
	PmStartServers    string `form:"pm_start_servers" json:"pm_start_servers"`
	PmMinSpareServers string `form:"pm_min_spare_servers" json:"pm_min_spare_servers"`
	PmMaxSpareServers string `form:"pm_max_spare_servers" json:"pm_max_spare_servers"`
}
