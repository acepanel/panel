package biz

import (
	"context"
	"log/slog"
	"time"
)

const (
	LogTypeApp  = "app"
	LogTypeDB   = "db"
	LogTypeHTTP = "http"
)

// 操作日志类型常量
const (
	OperationTypePanel          = "panel"
	OperationTypeWebsite        = "website"
	OperationTypeDatabase       = "database"
	OperationTypeDatabaseUser   = "database_user"
	OperationTypeDatabaseServer = "database_server"
	OperationTypeProject        = "project"
	OperationTypeCert           = "cert"
	OperationTypeFile           = "file"
	OperationTypeApp            = "app"
	OperationTypeCron           = "cron"
	OperationTypeBackup         = "backup"
	OperationTypeContainer      = "container"
	OperationTypeFirewall       = "firewall"
	OperationTypeSafe           = "safe"
	OperationTypeSSH            = "ssh"
	OperationTypeSetting        = "setting"
	OperationTypeMonitor        = "monitor"
	OperationTypeWebhook        = "webhook"
	OperationTypeUser           = "user"
)

// LogEntry 日志条目
type LogEntry struct {
	Time         time.Time      `json:"time"`
	Level        string         `json:"level"`
	Msg          string         `json:"msg"`
	Type         string         `json:"type,omitempty"`
	OperatorID   uint           `json:"operator_id,omitempty"`
	OperatorName string         `json:"operator_name,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// LogRepo 日志仓库接口
type LogRepo interface {
	// List 获取日志列表
	// date 格式为 YYYY-MM-DD，空字符串表示当天日志
	// level 为日志级别（DEBUG/INFO/WARN/ERROR），空字符串表示不过滤
	List(logType string, limit int, date, level string) ([]LogEntry, error)
	// ListDates 获取可用的日志日期列表
	ListDates(logType string) ([]string, error)
	// Clean 清理指定日期及之前的日志
	Clean(logType string, date string) error
}

type LogUsecase struct {
	repo LogRepo
	log  *slog.Logger
}

func NewLogUsecase(repo LogRepo, log *slog.Logger) *LogUsecase {
	return &LogUsecase{repo: repo, log: log}
}

func (uc *LogUsecase) List(logType string, limit int, date, level string) ([]LogEntry, error) {
	return uc.repo.List(logType, limit, date, level)
}

func (uc *LogUsecase) ListDates(logType string) ([]string, error) {
	return uc.repo.ListDates(logType)
}

func (uc *LogUsecase) Clean(ctx context.Context, logType, date string) error {
	if err := uc.repo.Clean(logType, date); err != nil {
		return err
	}

	// 记录日志，操作日志被清空后留下这一条作为痕迹
	uc.log.Info("logs cleaned", slog.String("type", OperationTypePanel), slog.Uint64("operator_id", operatorID(ctx)), slog.String("log_type", logType), slog.String("date", date))

	return nil
}
