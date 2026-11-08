package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

// MigrationSourceRepo 读取来源面板（宝塔 / 1Panel）的资源并生成备份
type MigrationSourceRepo interface {
	// Probe 探测来源面板类型与版本，同时校验凭据
	Probe(ctx context.Context, conn *request.ToolboxMigrationConnection) (*types.MigrationSource, error)
	// Items 列出来源面板上所有可迁移资源
	Items(ctx context.Context, conn *request.ToolboxMigrationConnection) ([]types.MigrationItem, error)
	// Detail 迁移前重新读取资源详情
	Detail(ctx context.Context, conn *request.ToolboxMigrationConnection, item types.MigrationItem) (*types.MigrationDetail, error)
	// SetRunning 启停来源资源
	SetRunning(ctx context.Context, conn *request.ToolboxMigrationConnection, item types.MigrationItem, running bool) error
	// Backup 在来源侧生成备份，返回来源上的文件路径
	Backup(ctx context.Context, conn *request.ToolboxMigrationConnection, detail *types.MigrationDetail) (string, error)
	// Download 下载来源备份到本地，progress 可为 nil
	Download(ctx context.Context, conn *request.ToolboxMigrationConnection, remote, local string, progress types.MigrationProgress) error
}

// MigrationRemoteRepo 访问目标 AcePanel 面板（AcePanel 之间迁移使用）
type MigrationRemoteRepo interface {
	// Request 调用目标面板 API
	Request(ctx context.Context, conn *request.ToolboxMigrationConnection, method, path string, body any) ([]byte, error)
	// Upload 分片上传文件到目标面板，progress 可为 nil
	Upload(ctx context.Context, conn *request.ToolboxMigrationConnection, local, remote string, progress types.MigrationProgress) error
	// Exec 在目标面板执行命令
	Exec(ctx context.Context, conn *request.ToolboxMigrationConnection, command string) error
}

// MigrationArchiveRepo 本地归档与目录操作
type MigrationArchiveRepo interface {
	// TempDir 创建迁移临时目录
	TempDir() (string, error)
	// Extract 解包归档到目标目录，返回真实内容根目录
	Extract(ctx context.Context, archive, target string) (string, error)
	// Compress 打包目录为 tar.gz
	Compress(ctx context.Context, source, archive string) error
	// CopyTree 复制目录内容（保留属性）
	CopyTree(ctx context.Context, source, target string) error
	// IsEmpty 判断目录为空或不存在
	IsEmpty(path string) bool
}

// migrationState 迁移全局状态
type migrationState struct {
	mu         sync.RWMutex
	step       types.MigrationStep
	connection *request.ToolboxMigrationConnection
	results    []types.MigrationResult
	logs       []string
	startedAt  *time.Time
	endedAt    *time.Time
}

// ToolboxMigrationUsecase 迁移编排，同一时间只允许一个迁移任务
type ToolboxMigrationUsecase struct {
	log     *slog.Logger
	t       *gotext.Locale
	source  MigrationSourceRepo
	remote  MigrationRemoteRepo
	archive MigrationArchiveRepo

	setting        *SettingUsecase
	website        *WebsiteUsecase
	database       *DatabaseUsecase
	databaseServer *DatabaseServerUsecase
	databaseUser   *DatabaseUserUsecase
	backup         *BackupUsecase
	project        *ProjectUsecase
	app            *AppUsecase
	environment    *EnvironmentUsecase

	// 目标资源名限制：网站与项目沿用容器命名规范，数据库按各自产品限制
	resourceName *regexp.Regexp
	mysqlName    *regexp.Regexp
	postgresName *regexp.Regexp
	version      *regexp.Regexp

	state migrationState
}

func NewToolboxMigrationUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	source MigrationSourceRepo,
	remote MigrationRemoteRepo,
	archive MigrationArchiveRepo,
	setting *SettingUsecase,
	website *WebsiteUsecase,
	database *DatabaseUsecase,
	databaseServer *DatabaseServerUsecase,
	databaseUser *DatabaseUserUsecase,
	backup *BackupUsecase,
	project *ProjectUsecase,
	app *AppUsecase,
	environment *EnvironmentUsecase,
) *ToolboxMigrationUsecase {
	return &ToolboxMigrationUsecase{
		log: log, t: t, source: source, remote: remote, archive: archive,
		setting: setting, website: website, database: database, databaseServer: databaseServer,
		databaseUser: databaseUser, backup: backup, project: project, app: app, environment: environment,
		resourceName: regexp.MustCompile(`^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9._-]{0,126}[A-Za-z0-9_-])$`),
		mysqlName:    regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`),
		postgresName: regexp.MustCompile(`^[A-Za-z0-9_.-]{1,63}$`),
		version:      regexp.MustCompile(`\d+`),
		state:        migrationState{step: types.MigrationStepIdle},
	}
}

// Connect 校验来源连接并保存，返回来源面板信息
func (uc *ToolboxMigrationUsecase) Connect(ctx context.Context, conn *request.ToolboxMigrationConnection) (*types.MigrationSource, error) {
	if conn.SourcePanel == "" {
		conn.SourcePanel = "acepanel"
	}
	if conn.SourcePanel == "acepanel" {
		if conn.TokenID == 0 || conn.Token == "" {
			return nil, errors.New(uc.t.Get("please fill in the AcePanel API credentials"))
		}
	} else if conn.APIKey == "" {
		return nil, errors.New(uc.t.Get("please fill in the source panel API key"))
	}

	uc.state.mu.RLock()
	running := uc.state.step == types.MigrationStepRunning
	uc.state.mu.RUnlock()
	if running {
		return nil, errors.New(uc.t.Get("migration is already running"))
	}

	var source *types.MigrationSource
	var err error
	if conn.SourcePanel == "acepanel" {
		source, err = uc.probeRemote(ctx, conn)
	} else {
		source, err = uc.source.Probe(ctx, conn)
	}
	if err != nil {
		return nil, err
	}

	uc.state.mu.Lock()
	uc.state.connection = conn
	uc.state.step = types.MigrationStepPreCheck
	uc.state.mu.Unlock()
	return source, nil
}

// Items 列出可迁移资源，并标注目标侧冲突
func (uc *ToolboxMigrationUsecase) Items(ctx context.Context) ([]types.MigrationItem, error) {
	conn, err := uc.connection()
	if err != nil {
		return nil, err
	}

	var items []types.MigrationItem
	if conn.SourcePanel == "acepanel" {
		items, err = uc.localItems(ctx)
	} else {
		items, err = uc.source.Items(ctx, conn)
		if err == nil {
			uc.checkConflicts(ctx, items)
		}
	}
	if err != nil {
		return nil, err
	}

	uc.state.mu.Lock()
	if uc.state.step == types.MigrationStepPreCheck {
		uc.state.step = types.MigrationStepSelect
	}
	uc.state.mu.Unlock()

	uc.log.Info(uc.t.Get("migration item list viewed"), slog.String("type", OperationTypeToolbox), slog.Uint64("operator_id", OperatorID(ctx)),
		slog.String("source_panel", conn.SourcePanel), slog.String("url", conn.URL))
	return items, nil
}

// Start 异步开始迁移
func (uc *ToolboxMigrationUsecase) Start(ctx context.Context, req *request.ToolboxMigrationStart) error {
	conn, err := uc.connection()
	if err != nil {
		return err
	}
	if len(req.Items) == 0 {
		return errors.New(uc.t.Get("please select at least one migration resource"))
	}

	uc.state.mu.Lock()
	if uc.state.step == types.MigrationStepRunning {
		uc.state.mu.Unlock()
		return errors.New(uc.t.Get("migration is already running"))
	}
	uc.state.step = types.MigrationStepRunning
	uc.state.results = nil
	uc.state.logs = nil
	uc.state.startedAt = new(time.Now())
	uc.state.endedAt = nil
	uc.state.mu.Unlock()

	connection := *conn
	go uc.run(context.WithoutCancel(ctx), &connection, req)

	uc.log.Info(uc.t.Get("migration started"), slog.String("type", OperationTypeToolbox), slog.Uint64("operator_id", OperatorID(ctx)),
		slog.String("source_panel", conn.SourcePanel), slog.String("url", conn.URL),
		slog.Any("items", lo.Map(req.Items, func(item request.ToolboxMigrationItem, _ int) string { return item.Key })),
		slog.Bool("skip_blocked", req.SkipBlocked), slog.Bool("stop_source", req.StopSource))
	return nil
}

// Status 返回当前迁移状态，since 为已获取的日志条数
func (uc *ToolboxMigrationUsecase) Status(since int) (types.MigrationStep, []types.MigrationResult, []string, *time.Time, *time.Time) {
	uc.state.mu.RLock()
	defer uc.state.mu.RUnlock()
	logs := uc.state.logs
	if since >= len(logs) {
		logs = nil
	} else if since > 0 {
		logs = logs[since:]
	}
	return uc.state.step, slices.Clone(uc.state.results), slices.Clone(logs), uc.state.startedAt, uc.state.endedAt
}

// Logs 返回完整迁移日志
func (uc *ToolboxMigrationUsecase) Logs() []string {
	uc.state.mu.RLock()
	defer uc.state.mu.RUnlock()
	return slices.Clone(uc.state.logs)
}

// Reset 重置迁移状态
func (uc *ToolboxMigrationUsecase) Reset(ctx context.Context) error {
	uc.state.mu.Lock()
	defer uc.state.mu.Unlock()
	if uc.state.step == types.MigrationStepRunning {
		return errors.New(uc.t.Get("migration is running, cannot reset"))
	}
	// 逐字段重置，整体赋值会把持有的锁一并换成零值
	uc.state.step = types.MigrationStepIdle
	uc.state.connection = nil
	uc.state.results = nil
	uc.state.logs = nil
	uc.state.startedAt = nil
	uc.state.endedAt = nil
	uc.log.Info(uc.t.Get("migration reset"), slog.String("type", OperationTypeToolbox), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}

// run 执行迁移，按依赖顺序逐项处理
func (uc *ToolboxMigrationUsecase) run(ctx context.Context, conn *request.ToolboxMigrationConnection, req *request.ToolboxMigrationStart) {
	// 迁移脱离请求生命周期，大文件传输给足超时
	ctx, cancel := context.WithTimeout(ctx, 24*time.Hour)
	defer cancel()
	uc.addLog("===== " + uc.t.Get("Migration started") + " =====")
	defer func() {
		uc.state.mu.Lock()
		uc.state.step = types.MigrationStepDone
		uc.state.endedAt = new(time.Now())
		uc.state.mu.Unlock()
		uc.addLog("===== " + uc.t.Get("Migration completed") + " =====")
	}()

	items, err := uc.selected(ctx, conn, req)
	if err != nil {
		uc.addLog(uc.t.Get("failed to refresh source resource list: %v", err))
		return
	}

	// 用户最先建，PG 的 dump 带 OWNER TO，角色不存在会让整个导入失败
	priority := map[string]int{"database_user": 0, "database": 1, "project": 2, "website": 3}
	slices.SortStableFunc(items, func(a, b types.MigrationItem) int {
		return priority[a.Type] - priority[b.Type]
	})
	for _, item := range items {
		uc.addResult(types.MigrationResult{
			Key: item.Key, Type: item.Type, Name: item.Name,
			Status: types.MigrationPending, Warnings: slices.Clone(item.Warnings),
		})
	}

	selected := lo.SliceToMap(items, func(item types.MigrationItem) (string, struct{}) { return item.Key, struct{}{} })
	for _, item := range items {
		if reason := uc.blocked(item, selected, req.SkipBlocked); reason != "" {
			continue
		}
		uc.migrate(ctx, conn, item, req)
	}
}

// blocked 检查该项是否应跳过，返回非空原因表示已记录结果
func (uc *ToolboxMigrationUsecase) blocked(item types.MigrationItem, selected map[string]struct{}, skip bool) string {
	reason := strings.Join(item.Blockers, "; ")
	if reason == "" {
		for _, dependency := range item.DependsOn {
			if _, ok := selected[dependency]; !ok {
				reason = uc.t.Get("a required dependency was not selected")
				break
			}
			// 依赖按类型优先级排在前面，此时已有终态
			result, ok := uc.result(dependency)
			if ok && result.Stage == types.MigrationStageDone &&
				result.Status != types.MigrationSuccess && result.Status != types.MigrationPartial {
				reason = uc.t.Get("a required dependency did not migrate successfully")
				break
			}
		}
	}
	if reason == "" {
		return ""
	}
	if skip {
		uc.finish(item.Key, types.MigrationSkipped, reason, nil)
	} else {
		uc.finish(item.Key, types.MigrationFailed, reason, nil)
	}
	return reason
}

// migrate 迁移单个资源
func (uc *ToolboxMigrationUsecase) migrate(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
	req *request.ToolboxMigrationStart,
) {
	uc.setStage(item.Key, types.MigrationStageBackup)
	uc.addLog(uc.t.Get("[%s] start migrating", item.Name))

	var warnings []string
	var err error
	if conn.SourcePanel == "acepanel" {
		warnings, err = uc.push(ctx, conn, item, req)
	} else {
		warnings, err = uc.pull(ctx, conn, item, req)
	}

	switch {
	case err != nil:
		uc.finish(item.Key, types.MigrationFailed, err.Error(), warnings)
		uc.addLog(uc.t.Get("[%s] migration failed: %v", item.Name, err))
	case len(warnings) > 0:
		uc.finish(item.Key, types.MigrationPartial, "", warnings)
		uc.addLog(uc.t.Get("[%s] migration completed with warnings", item.Name))
	default:
		uc.finish(item.Key, types.MigrationSuccess, "", nil)
		uc.addLog(uc.t.Get("[%s] migration completed", item.Name))
	}
}

// pull 从来源面板拉取并导入到本地
func (uc *ToolboxMigrationUsecase) pull(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	item types.MigrationItem,
	req *request.ToolboxMigrationStart,
) ([]string, error) {
	detail, err := uc.source.Detail(ctx, conn, item)
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to read source resource detail: %v", err))
	}
	detail.Item = item

	// 用户没有可备份的内容，直接建到目标
	if item.Type == "database_user" {
		uc.setStage(item.Key, types.MigrationStageImport)
		return uc.importDatabaseUser(ctx, detail)
	}

	// 备份期间停止来源避免文件不一致，备份落盘后立即恢复；数据库为在线导出无需停止
	stopped := req.StopSource && item.Status == "running" && item.Type != "database"
	if stopped {
		_ = uc.source.SetRunning(ctx, conn, item, false)
	}
	remote, err := uc.source.Backup(ctx, conn, detail)
	if stopped {
		_ = uc.source.SetRunning(ctx, conn, item, true)
	}
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to create source backup: %v", err))
	}

	tmpDir, err := uc.archive.TempDir()
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	uc.setStage(item.Key, types.MigrationStageTransfer)
	local := filepath.Join(tmpDir, filepath.Base(remote))
	if err = uc.source.Download(ctx, conn, remote, local, uc.transferProgress(item.Name, uc.t.Get("downloading"))); err != nil {
		return nil, errors.New(uc.t.Get("failed to download source backup: %v", err))
	}

	uc.setStage(item.Key, types.MigrationStageImport)
	switch item.Type {
	case "website":
		return uc.importWebsite(ctx, detail, local)
	case "database":
		return uc.importDatabase(ctx, detail, local)
	case "project":
		return uc.importProject(ctx, detail, local)
	default:
		return nil, errors.New(uc.t.Get("unsupported migration resource type: %s", item.Type))
	}
}

// selected 按前端选择解析出待迁移资源，并应用目标覆盖项
func (uc *ToolboxMigrationUsecase) selected(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	req *request.ToolboxMigrationStart,
) ([]types.MigrationItem, error) {
	var catalog []types.MigrationItem
	var err error
	if conn.SourcePanel == "acepanel" {
		catalog, err = uc.localItems(ctx)
	} else {
		catalog, err = uc.source.Items(ctx, conn)
		if err == nil {
			uc.checkConflicts(ctx, catalog)
		}
	}
	if err != nil {
		return nil, err
	}

	byKey := lo.SliceToMap(catalog, func(item types.MigrationItem) (string, types.MigrationItem) { return item.Key, item })
	items := make([]types.MigrationItem, 0, len(req.Items))
	for _, choice := range req.Items {
		item, ok := byKey[choice.Key]
		if !ok {
			uc.addResult(types.MigrationResult{
				Key: choice.Key, Type: "unknown", Name: choice.Key, Status: types.MigrationFailed,
				Stage: types.MigrationStageDone, Error: uc.t.Get("resource no longer exists on the source server"),
			})
			continue
		}
		if choice.TargetPath != "" {
			item.TargetPath = choice.TargetPath
		}
		item.TargetUser = choice.TargetUser
		items = append(items, item)
	}
	return items, nil
}

func (uc *ToolboxMigrationUsecase) connection() (*request.ToolboxMigrationConnection, error) {
	uc.state.mu.RLock()
	defer uc.state.mu.RUnlock()
	if uc.state.connection == nil {
		return nil, errors.New(uc.t.Get("please connect to the source server first"))
	}
	return uc.state.connection, nil
}

// transferProgress 生成传输进度回调
func (uc *ToolboxMigrationUsecase) transferProgress(name, action string) types.MigrationProgress {
	start := time.Now()
	last, logged := start, false
	return func(transferred, total int64) {
		now := time.Now()
		done := logged && total > 0 && transferred >= total
		if now.Sub(last) < time.Second && !done {
			return
		}
		last, logged = now, true

		speed := float64(transferred) / now.Sub(start).Seconds()
		progress := tools.FormatBytes(float64(transferred))
		if total > 0 {
			progress = fmt.Sprintf("%s/%s %.1f%%", progress, tools.FormatBytes(float64(total)), float64(transferred)/float64(total)*100)
		}
		uc.addLog(fmt.Sprintf("[%s] %s %s %s/s", name, action, progress, tools.FormatBytes(speed)))
	}
}

func (uc *ToolboxMigrationUsecase) addLog(message string) {
	uc.state.mu.Lock()
	defer uc.state.mu.Unlock()
	uc.state.logs = append(uc.state.logs, time.Now().Format("2006-01-02 15:04:05")+" "+message)
}

func (uc *ToolboxMigrationUsecase) addResult(result types.MigrationResult) {
	uc.state.mu.Lock()
	defer uc.state.mu.Unlock()
	uc.state.results = append(uc.state.results, result)
}

func (uc *ToolboxMigrationUsecase) result(key string) (types.MigrationResult, bool) {
	uc.state.mu.RLock()
	defer uc.state.mu.RUnlock()
	index := slices.IndexFunc(uc.state.results, func(result types.MigrationResult) bool { return result.Key == key })
	if index < 0 {
		return types.MigrationResult{}, false
	}
	return uc.state.results[index], true
}

func (uc *ToolboxMigrationUsecase) setStage(key string, stage types.MigrationStage) {
	uc.state.mu.Lock()
	defer uc.state.mu.Unlock()
	index := slices.IndexFunc(uc.state.results, func(result types.MigrationResult) bool { return result.Key == key })
	if index < 0 {
		return
	}
	result := &uc.state.results[index]
	result.Status = types.MigrationRunning
	result.Stage = stage
	if result.StartedAt == nil {
		result.StartedAt = new(time.Now())
	}
}

// finish 记录迁移项终态
func (uc *ToolboxMigrationUsecase) finish(key string, status types.MigrationStatus, message string, warnings []string) {
	uc.state.mu.Lock()
	defer uc.state.mu.Unlock()
	index := slices.IndexFunc(uc.state.results, func(result types.MigrationResult) bool { return result.Key == key })
	if index < 0 {
		return
	}
	now := time.Now()
	result := &uc.state.results[index]
	result.Status = status
	result.Stage = types.MigrationStageDone
	result.Error = message
	result.Warnings = append(result.Warnings, warnings...)
	result.EndedAt = &now
	if result.StartedAt != nil {
		result.Duration = now.Sub(*result.StartedAt).Seconds()
	}
}
