package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

// rotatedLog 文件名带日期的是已轮转的旧日志
var rotatedLog = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

type ToolboxLogService struct {
	t                  *gotext.Locale
	log                *slog.Logger
	db                 *gorm.DB
	containerImageRepo *biz.ContainerImageUsecase
	settingRepo        *biz.SettingUsecase
}

func NewToolboxLogService(containerImageUsecase *biz.ContainerImageUsecase, settingUsecase *biz.SettingUsecase, db *gorm.DB, t *gotext.Locale, log *slog.Logger) *ToolboxLogService {
	return &ToolboxLogService{
		t:                  t,
		log:                log,
		db:                 db,
		containerImageRepo: containerImageUsecase,
		settingRepo:        settingUsecase,
	}
}

// LogItem 日志项信息
type LogItem struct {
	Name string `json:"name"` // 日志名称
	Path string `json:"path"` // 日志路径，也是勾选清理时的标识
	Size string `json:"size"` // 日志大小

	bytes int64
	clean func(ctx context.Context) error
}

// Scan 扫描日志
func (s *ToolboxLogService) Scan(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxLogScan](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items := s.scan(r.Context(), req.Type)
	s.log.Info(s.t.Get("log list viewed"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("log_type", req.Type))

	Success(w, items)
}

// Clean 清理勾选的日志
func (s *ToolboxLogService) Clean(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxLogClean](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 以重新扫描的结果为准，只清理其中被勾选的项
	var cleaned int64
	var errs []error
	var names []string
	for _, item := range s.scan(r.Context(), req.Type) {
		if !slices.Contains(req.Paths, item.Path) {
			continue
		}
		if err = item.clean(r.Context()); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", item.Name, err))
			continue
		}
		cleaned += item.bytes
		names = append(names, item.Name)
	}
	// 部分失败时已清理的项也要留痕
	if len(names) > 0 {
		s.log.Warn(s.t.Get("logs cleaned"), slog.String("type", biz.OperationTypeToolbox), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("log_type", req.Type), slog.Any("items", names), slog.String("cleaned", tools.FormatBytes(float64(cleaned))), slog.Int("failed", len(errs)))
	}
	if len(errs) > 0 {
		Error(w, http.StatusInternalServerError, "%v", errors.Join(errs...))
		return
	}

	Success(w, chix.M{
		"cleaned": tools.FormatBytes(float64(cleaned)),
	})
}

func (s *ToolboxLogService) scan(ctx context.Context, typ string) []LogItem {
	switch typ {
	case "panel":
		return fileItems("", filepath.Join(app.Root, "panel/storage/logs/*.log"))
	case "cron":
		return s.scanCronLogs()
	case "website":
		return s.scanWebsiteLogs()
	case "mysql":
		return s.scanMySQLLogs()
	case "docker":
		return s.scanDockerLogs(ctx)
	case "system":
		return s.scanSystemLogs(ctx)
	}
	return nil
}

// scanCronLogs 扫描计划任务日志
func (s *ToolboxLogService) scanCronLogs() []LogItem {
	var crons []*biz.Cron
	_ = s.db.Find(&crons).Error

	logs := lo.KeyBy(fileItems("", filepath.Join(app.Root, "server/cron/logs/*.log")), func(item LogItem) string {
		return item.Path
	})
	items := make([]LogItem, 0, len(logs))
	for _, cron := range crons {
		if item, ok := logs[cron.Log]; ok {
			item.Name = cron.Name
			items = append(items, item)
			delete(logs, cron.Log)
		}
	}

	// 剩下的属于已删除的任务，没有进程再写，直接删掉
	if len(logs) > 0 {
		deleted := make([]LogItem, 0, len(logs))
		for path, item := range logs {
			item.clean = func(context.Context) error { return os.Remove(path) }
			deleted = append(deleted, item)
		}
		items = append(items, mergeItems(s.t.Get("Deleted task logs: %d files", len(deleted)), "cron:deleted", deleted))
	}

	return items
}

// scanWebsiteLogs 扫描网站日志
func (s *ToolboxLogService) scanWebsiteLogs() []LogItem {
	var websites []*biz.Website
	_ = s.db.Find(&websites).Error

	items := make([]LogItem, 0)
	for _, website := range websites {
		items = append(items, fileItems(website.Name+" - ", filepath.Join(app.Root, "sites", website.Name, "log", "*"))...)
	}

	return items
}

// scanMySQLLogs 扫描 MySQL 日志
func (s *ToolboxLogService) scanMySQLLogs() []LogItem {
	mysqlPath := filepath.Join(app.Root, "server/mysql")
	items := fileItems("", filepath.Join(mysqlPath, "mysql-slow.log"))

	// binlog 只能按顺序清理且最后一个正在写入，合并为一项交给 MySQL 清理
	if binlogs := fileItems("", filepath.Join(mysqlPath, "data/mysql-bin.[0-9]*")); len(binlogs) > 1 {
		item := mergeItems(s.t.Get("Binary logs: %d files", len(binlogs)-1), "mysql:binlog", binlogs[:len(binlogs)-1])
		item.clean = s.purgeBinlogs
		items = append(items, item)
	}

	return items
}

func (s *ToolboxLogService) purgeBinlogs(ctx context.Context) error {
	password, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return err
	}
	mysql, err := db.NewMySQL(ctx, "root", password, db.MySQLSocket(app.Root), "unix")
	if err != nil {
		return err
	}
	defer mysql.Close()

	_, err = mysql.Exec(ctx, "PURGE BINARY LOGS BEFORE NOW()")
	return err
}

// scanDockerLogs 扫描 Docker/Podman 相关内容
func (s *ToolboxLogService) scanDockerLogs(ctx context.Context) []LogItem {
	items := make([]LogItem, 0)

	images, _ := s.containerImageRepo.List(ctx)
	if unused := lo.CountBy(images, func(img types.ContainerImage) bool { return img.Containers == 0 }); unused > 0 {
		items = append(items, LogItem{
			Name:  s.t.Get("Unused container images: %d", unused),
			Path:  "docker:images",
			Size:  s.t.Get("%d images", unused),
			clean: s.containerImageRepo.Prune,
		})
	}

	if logs := fileItems("", "/var/lib/docker/containers/*/*.log"); len(logs) > 0 {
		items = append(items, mergeItems(s.t.Get("Docker container logs: %d files", len(logs)), "docker:logs", logs))
	}
	if logs := fileItems("",
		"/var/lib/containers/storage/overlay-containers/*/userdata/*.log",
		"/run/containers/storage/overlay-containers/*/userdata/*.log",
	); len(logs) > 0 {
		items = append(items, mergeItems(s.t.Get("Podman container logs: %d files", len(logs)), "podman:logs", logs))
	}

	return items
}

// scanSystemLogs 扫描系统日志
func (s *ToolboxLogService) scanSystemLogs(ctx context.Context) []LogItem {
	items := fileItems("",
		"/var/log/syslog", "/var/log/messages", "/var/log/auth.log", "/var/log/secure",
		"/var/log/kern.log", "/var/log/dmesg", "/var/log/btmp", "/var/log/wtmp", "/var/log/*.log",
	)

	usage, _ := shell.Exec(ctx, `journalctl --disk-usage 2>/dev/null | grep -oP '\d+\.?\d*[KMGT]?' || echo '0'`)
	if usage != "" && usage != "0" {
		items = append(items, LogItem{
			Name: s.t.Get("Journal logs"),
			Path: "system:journal",
			Size: usage,
			clean: func(ctx context.Context) error {
				_, err := shell.Exec(ctx, "journalctl --vacuum-time=1d")
				return err
			},
		})
	}

	return items
}

// fileItems 列出匹配的日志文件，未轮转的可能仍被进程写着，只能清空不能删
func fileItems(prefix string, patterns ...string) []LogItem {
	var paths []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		paths = append(paths, matches...)
	}

	items := make([]LogItem, 0, len(paths))
	for _, path := range lo.Uniq(paths) {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		clean := func(context.Context) error { return os.Truncate(path, 0) }
		if rotatedLog.MatchString(info.Name()) {
			clean = func(context.Context) error { return os.Remove(path) }
		}
		items = append(items, LogItem{
			Name:  prefix + info.Name(),
			Path:  path,
			Size:  tools.FormatBytes(float64(info.Size())),
			bytes: info.Size(),
			clean: clean,
		})
	}

	return items
}

// mergeItems 把多个日志项合并为一项
func mergeItems(name, path string, items []LogItem) LogItem {
	size := lo.SumBy(items, func(item LogItem) int64 { return item.bytes })
	return LogItem{
		Name:  name,
		Path:  path,
		Size:  tools.FormatBytes(float64(size)),
		bytes: size,
		clean: func(ctx context.Context) error {
			errs := make([]error, 0, len(items))
			for _, item := range items {
				errs = append(errs, item.clean(ctx))
			}
			return errors.Join(errs...)
		},
	}
}
