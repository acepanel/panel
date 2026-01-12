package biz

import (
	"time"
)

const (
	LogTypeApp  = "app"
	LogTypeDB   = "db"
	LogTypeHTTP = "http"
)

// 操作日志类型常量
const (
	OperationTypePanel         = "panel"
	OperationTypeWebsite       = "website"
	OperationTypeDatabase      = "database"
	OperationTypeDatabaseUser  = "database_user"
	OperationTypeProject       = "project"
	OperationTypeCert          = "cert"
	OperationTypeFile          = "file"
	OperationTypeApp           = "app"
	OperationTypeCron          = "cron"
	OperationTypeBackup        = "backup"
	OperationTypeContainer     = "container"
	OperationTypeFirewall      = "firewall"
	OperationTypeSafe          = "safe"
	OperationTypeSSH           = "ssh"
	OperationTypeSetting       = "setting"
	OperationTypeMonitor       = "monitor"
	OperationTypeDatabaseServer = "database_server"
)

// LogEntry 日志条目
type LogEntry struct {
	Time       time.Time         `json:"time"`
	Level      string            `json:"level"`
	Msg        string            `json:"msg"`
	Type       string            `json:"type,omitempty"`
	OperatorID uint              `json:"operator_id,omitempty"`
	Extra      map[string]any    `json:"extra,omitempty"`
}

// LogRepo 日志仓库接口
type LogRepo interface {
	// List 获取日志列表
	List(logType string, limit int) ([]LogEntry, error)
}
