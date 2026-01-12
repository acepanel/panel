package request

// ToolboxLogClean 日志清理请求
type ToolboxLogClean struct {
	Type string `form:"type" json:"type" validate:"required|in:panel,website,mysql,docker,system"`
}
