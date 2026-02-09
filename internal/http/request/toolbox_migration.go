package request

// ToolboxMigrationConnection 迁移连接信息
type ToolboxMigrationConnection struct {
	URL     string `json:"url" validate:"required"`
	TokenID uint   `json:"token_id" validate:"required"`
	Token   string `json:"token" validate:"required"`
}

// ToolboxMigrationItems 迁移选择项
type ToolboxMigrationItems struct {
	Websites  []ToolboxMigrationWebsite  `json:"websites"`
	Databases []ToolboxMigrationDatabase `json:"databases"`
	Projects  []ToolboxMigrationProject  `json:"projects"`
	StopOnMig bool                       `json:"stop_on_mig"` // 迁移中是否停止服务
}

// ToolboxMigrationWebsite 迁移网站项
type ToolboxMigrationWebsite struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"` // 网站目录
}

// ToolboxMigrationDatabase 迁移数据库项
type ToolboxMigrationDatabase struct {
	Type     string `json:"type"` // mysql / postgresql
	Name     string `json:"name"`
	ServerID uint   `json:"server_id"`
	Server   string `json:"server"` // 服务器名称
}

// ToolboxMigrationProject 迁移项目项
type ToolboxMigrationProject struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"` // 项目目录
}

// ToolboxMigrationExec 远程执行命令请求
type ToolboxMigrationExec struct {
	Command string `json:"command" validate:"required"`
}
