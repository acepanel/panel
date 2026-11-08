package postgresql

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

var defaultVersionRegexp = regexp.MustCompile(`default_version\s*=\s*'([^']+)'`)

type App struct {
	t                  *gotext.Locale
	log                *slog.Logger
	conf               *config.Config
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
	taskRepo           biz.TaskRepo
}

func NewApp(t *gotext.Locale, log *slog.Logger, conf *config.Config, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo, taskRepo biz.TaskRepo) *App {
	return &App{
		t:                  t,
		log:                log,
		conf:               conf,
		settingRepo:        settingRepo,
		databaseServerRepo: databaseServerRepo,
		taskRepo:           taskRepo,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/user_config", s.GetUserConfig)
	r.Post("/user_config", s.UpdateUserConfig)
	r.Get("/load", s.Load)
	r.Get("/log", s.Log)
	r.Get("/postgres_password", s.GetPostgresPassword)
	r.Post("/postgres_password", s.SetPostgresPassword)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	// 扩展管理
	r.Get("/extensions", s.ExtensionList)
	r.Post("/extensions", s.InstallExtension)
	r.Delete("/extensions", s.UninstallExtension)
	r.Post("/extensions/enable", s.EnableExtension)
	// 性能
	r.Get("/sessions", s.SessionList)
	r.Post("/sessions/{pid}/terminate", s.TerminateSession)
	r.Get("/top_sql", s.TopSQL)
	r.Post("/top_sql/enable", s.EnableTopSQL)
	r.Post("/top_sql/reset", s.ResetTopSQL)
	// 维护
	r.Get("/databases", s.DatabaseList)
	r.Get("/bloat", s.BloatList)
	r.Post("/maintenance", s.RunMaintenance)
	r.Get("/wal", s.WalStatus)
	r.Delete("/replication_slots/{slot}", s.DropReplicationSlot)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "postgresql")
	return types.AggregateAppStatus(ok)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	// 获取配置
	config, err := io.Read(s.configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("path", s.configPath()))

	service.Success(w, config)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldPort := db.PostgresPort(app.Root)
	if err = io.Write(s.configPath(), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.applyConfig(r.Context(), req.Config, oldPort); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to apply PostgreSQL config: %v", err))
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("path", s.configPath()))

	service.Success(w, nil)
}

// GetUserConfig 获取用户配置
func (s *App) GetUserConfig(w http.ResponseWriter, r *http.Request) {
	// 获取配置
	config, err := io.Read(app.Root + "/server/postgresql/data/pg_hba.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("path", app.Root+"/server/postgresql/data/pg_hba.conf"))

	service.Success(w, config)
}

// UpdateUserConfig 保存用户配置
func (s *App) UpdateUserConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(app.Root+"/server/postgresql/data/pg_hba.conf", req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload(r.Context(), "postgresql"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload PostgreSQL: %v", err))
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("path", app.Root+"/server/postgresql/data/pg_hba.conf"))

	service.Success(w, nil)
}

// Load 获取负载
func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, _ := systemctl.Status(r.Context(), "postgresql")
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	postgresPassword, err := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load PostgreSQL postgres password: %v", err))
		return
	}

	env := []string{"PGPASSWORD=" + postgresPassword}
	port := db.PostgresPort(app.Root)
	start, err := shell.ExecfWithEnv(r.Context(), env, `psql -h 127.0.0.1 -p %d -U postgres -t -c "select pg_postmaster_start_time();" | head -1 | cut -d'.' -f1`, port)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL start time: %v", err))
		return
	}
	pid, err := shell.ExecfWithEnv(r.Context(), env, `psql -h 127.0.0.1 -p %d -U postgres -t -c "select pg_backend_pid();"`, port)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL backend pid: %v", err))
		return
	}
	process, err := shell.Execf(r.Context(), `ps aux | grep postgres | grep -v grep | wc -l`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL process: %v", err))
		return
	}
	connections, err := shell.ExecfWithEnv(r.Context(), env, `psql -h 127.0.0.1 -p %d -U postgres -t -c "SELECT count(*) FROM pg_stat_activity WHERE NOT pid=pg_backend_pid();"`, port)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL connections: %v", err))
		return
	}
	storage, err := shell.ExecfWithEnv(r.Context(), env, `psql -h 127.0.0.1 -p %d -U postgres -t -c "select pg_size_pretty(pg_database_size('postgres'));"`, port)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL database size: %v", err))
		return
	}

	data := []types.NV{
		{Name: s.t.Get("Start Time"), Value: start},
		{Name: s.t.Get("Process PID"), Value: pid},
		{Name: s.t.Get("Process Count"), Value: process},
		{Name: s.t.Get("Total Connections"), Value: connections},
		{Name: s.t.Get("Storage Usage"), Value: storage},
	}

	service.Success(w, data)
}

// Log 获取应用日志路径列表
func (s *App) Log(w http.ResponseWriter, r *http.Request) {
	paths, err := filepath.Glob(app.Root + "/server/postgresql/logs/postgresql-*.log")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, paths)
}

// GetPostgresPassword 获取 postgres 用户密码
func (s *App) GetPostgresPassword(w http.ResponseWriter, r *http.Request) {
	password, err := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get postgres password: %v", err))
		return
	}

	s.log.Info(s.t.Get("app password viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("username", "postgres"))

	service.Success(w, password)
}

// SetPostgresPassword 设置 postgres 用户密码
func (s *App) SetPostgresPassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SetPostgresPassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldPassword, _ := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	port := db.PostgresPort(app.Root)
	// 改完密码还要存进面板库，中途取消会让两边密码对不上
	ctx := context.WithoutCancel(r.Context())
	postgres, err := db.NewPostgres(r.Context(), "postgres", oldPassword, "127.0.0.1", port)
	if err != nil {
		// 回退到直接修改密码
		if _, err = shell.Execf(ctx, `su -s /bin/bash - postgres -c "psql -p %d -c \"ALTER USER postgres WITH PASSWORD '%s';\""`, port, req.Password); err != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set postgres password: %v", err))
			return
		}
	} else {
		defer postgres.Close()
		if err = postgres.UserPassword(ctx, "postgres", req.Password); err != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set postgres password: %v", err))
			return
		}
	}

	if err = s.settingRepo.Set(biz.SettingKeyPostgresPassword, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set postgres password: %v", err))
		return
	}

	_ = s.databaseServerRepo.UpdatePassword("local_postgresql", req.Password)

	s.log.Info(s.t.Get("app password updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("username", "postgres"))

	service.Success(w, nil)
}

// GetConfigTune 获取 PostgreSQL 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(s.configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		// 连接设置
		ListenAddresses:              confval.Postgres.Get(config, "listen_addresses"),
		Port:                         confval.Postgres.Get(config, "port"),
		MaxConnections:               confval.Postgres.Get(config, "max_connections"),
		SuperuserReservedConnections: confval.Postgres.Get(config, "superuser_reserved_connections"),
		// 内存设置
		SharedBuffers:      confval.Postgres.Get(config, "shared_buffers"),
		WorkMem:            confval.Postgres.Get(config, "work_mem"),
		MaintenanceWorkMem: confval.Postgres.Get(config, "maintenance_work_mem"),
		EffectiveCacheSize: confval.Postgres.Get(config, "effective_cache_size"),
		HugePages:          confval.Postgres.Get(config, "huge_pages"),
		// WAL 设置
		WalLevel:                   confval.Postgres.Get(config, "wal_level"),
		WalBuffers:                 confval.Postgres.Get(config, "wal_buffers"),
		MaxWalSize:                 confval.Postgres.Get(config, "max_wal_size"),
		MinWalSize:                 confval.Postgres.Get(config, "min_wal_size"),
		CheckpointCompletionTarget: confval.Postgres.Get(config, "checkpoint_completion_target"),
		// 查询优化
		DefaultStatisticsTarget: confval.Postgres.Get(config, "default_statistics_target"),
		RandomPageCost:          confval.Postgres.Get(config, "random_page_cost"),
		EffectiveIoConcurrency:  confval.Postgres.Get(config, "effective_io_concurrency"),
		// 日志设置
		LogDestination:          confval.Postgres.Get(config, "log_destination"),
		LogMinDurationStatement: confval.Postgres.Get(config, "log_min_duration_statement"),
		LogTimezone:             confval.Postgres.Get(config, "log_timezone"),
		// IO 设置
		IoMethod: confval.Postgres.Get(config, "io_method"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 PostgreSQL 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := s.configPath()
	oldPort := db.PostgresPort(app.Root)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新连接设置
	config = confval.Postgres.Set(config, "listen_addresses", req.ListenAddresses)
	config = confval.Postgres.Set(config, "port", req.Port)
	config = confval.Postgres.Set(config, "max_connections", req.MaxConnections)
	config = confval.Postgres.Set(config, "superuser_reserved_connections", req.SuperuserReservedConnections)
	// 更新内存设置
	config = confval.Postgres.Set(config, "shared_buffers", req.SharedBuffers)
	config = confval.Postgres.Set(config, "work_mem", req.WorkMem)
	config = confval.Postgres.Set(config, "maintenance_work_mem", req.MaintenanceWorkMem)
	config = confval.Postgres.Set(config, "effective_cache_size", req.EffectiveCacheSize)
	config = confval.Postgres.Set(config, "huge_pages", req.HugePages)
	// 更新 WAL 设置
	config = confval.Postgres.Set(config, "wal_level", req.WalLevel)
	config = confval.Postgres.Set(config, "wal_buffers", req.WalBuffers)
	config = confval.Postgres.Set(config, "max_wal_size", req.MaxWalSize)
	config = confval.Postgres.Set(config, "min_wal_size", req.MinWalSize)
	config = confval.Postgres.Set(config, "checkpoint_completion_target", req.CheckpointCompletionTarget)
	// 更新查询优化
	config = confval.Postgres.Set(config, "default_statistics_target", req.DefaultStatisticsTarget)
	config = confval.Postgres.Set(config, "random_page_cost", req.RandomPageCost)
	config = confval.Postgres.Set(config, "effective_io_concurrency", req.EffectiveIoConcurrency)
	// 更新日志设置
	config = confval.Postgres.Set(config, "log_destination", req.LogDestination)
	config = confval.Postgres.Set(config, "log_min_duration_statement", req.LogMinDurationStatement)
	config = confval.Postgres.Set(config, "log_timezone", req.LogTimezone)
	// 更新 IO 设置
	config = confval.Postgres.Set(config, "io_method", req.IoMethod)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.applyConfig(r.Context(), config, oldPort); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to apply PostgreSQL config: %v", err))
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("path", confPath), slog.String("port", req.Port), slog.String("listen_addresses", req.ListenAddresses), slog.String("max_connections", req.MaxConnections), slog.String("shared_buffers", req.SharedBuffers))

	service.Success(w, nil)
}

// ExtensionList 获取扩展列表及安装状态
func (s *App) ExtensionList(w http.ResponseWriter, r *http.Request) {
	extensions := s.getExtensions()
	for i := range extensions {
		controlPath := fmt.Sprintf("%s/server/postgresql/share/extension/%s.control", app.Root, extensions[i].ExtName)
		extensions[i].Installed = io.Exists(controlPath)
		if extensions[i].Installed {
			control, _ := io.Read(controlPath)
			if m := defaultVersionRegexp.FindStringSubmatch(control); len(m) > 1 {
				extensions[i].InstalledVersion = m[1]
			}
		}
	}

	s.log.Info(s.t.Get("postgresql extension list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"))

	service.Success(w, extensions)
}

// InstallExtension 安装扩展（异步任务）
func (s *App) InstallExtension(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ExtensionSlug](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !s.checkExtension(req.Slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("extension %s does not exist", req.Slug))
		return
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/postgresql/extensions/%s.sh' | bash -s -- 'install'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug))

	task := new(biz.Task)
	task.Key = "postgresql:extension:" + req.Slug
	task.Name = s.t.Get("Install PostgreSQL extension %s", req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("postgresql extension install queued"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("slug", req.Slug), slog.Uint64("task_id", uint64(task.ID)))

	service.Success(w, nil)
}

// UninstallExtension 卸载扩展（异步任务）
func (s *App) UninstallExtension(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ExtensionSlug](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !s.checkExtension(req.Slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("extension %s does not exist", req.Slug))
		return
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/postgresql/extensions/%s.sh' | bash -s -- 'uninstall'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug))

	task := new(biz.Task)
	task.Key = "postgresql:extension:" + req.Slug
	task.Name = s.t.Get("Uninstall PostgreSQL extension %s", req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("postgresql extension uninstall queued"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("slug", req.Slug), slog.Uint64("task_id", uint64(task.ID)))

	service.Success(w, nil)
}

// EnableExtension 在指定数据库启用扩展
func (s *App) EnableExtension(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ExtensionEnable](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	ext, ok := lo.Find(s.getExtensions(), func(e Extension) bool {
		return e.Slug == req.Slug
	})
	if !ok {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("extension %s does not exist", req.Slug))
		return
	}

	postgres, err := s.connect(r.Context(), req.Database)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	if _, err = postgres.Exec(r.Context(), fmt.Sprintf(`CREATE EXTENSION IF NOT EXISTS "%s"`, ext.ExtName)); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to enable extension: %v", err))
		return
	}

	s.log.Info(s.t.Get("postgresql extension enabled"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("slug", req.Slug), slog.String("ext_name", ext.ExtName), slog.String("database", req.Database))

	service.Success(w, nil)
}

// SessionList 获取会话列表
func (s *App) SessionList(w http.ResponseWriter, r *http.Request) {
	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	rows, err := postgres.Query(r.Context(), `
		SELECT a.pid, coalesce(a.datname,''), coalesce(a.usename,''), coalesce(a.client_addr::text,''),
		       coalesce(a.state,''), coalesce(a.wait_event_type,''), coalesce(a.wait_event,''),
		       coalesce(array_to_string(pg_blocking_pids(a.pid),','),''),
		       coalesce(extract(epoch from (now()-a.xact_start))::bigint,0),
		       coalesce(extract(epoch from (now()-a.query_start))::bigint,0),
		       coalesce(a.query,'')
		FROM pg_stat_activity a
		WHERE a.pid != pg_backend_pid() AND a.backend_type = 'client backend'
		ORDER BY a.backend_start`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	sessions := make([]Session, 0)
	for rows.Next() {
		var item Session
		if err = rows.Scan(&item.PID, &item.Database, &item.User, &item.ClientAddr, &item.State,
			&item.WaitEventType, &item.WaitEvent, &item.BlockedBy, &item.XactSeconds, &item.QuerySeconds, &item.Query); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		sessions = append(sessions, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("postgresql session list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"))

	service.Success(w, sessions)
}

// TerminateSession 终止会话
func (s *App) TerminateSession(w http.ResponseWriter, r *http.Request) {
	pid := cast.ToInt64(chi.URLParam(r, "pid"))
	if pid <= 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid pid"))
		return
	}

	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	if _, err = postgres.Exec(r.Context(), `SELECT pg_terminate_backend($1)`, pid); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("postgresql session terminated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.Int64("pid", pid))

	service.Success(w, nil)
}

// TopSQL 获取 SQL 性能统计
func (s *App) TopSQL(w http.ResponseWriter, r *http.Request) {
	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	rows, err := postgres.Query(r.Context(), `
		SELECT coalesce(d.datname,''), s.calls, round(s.total_exec_time)::bigint,
		       round(s.mean_exec_time::numeric,2)::float8, s.rows,
		       coalesce(round(100.0*s.shared_blks_hit/nullif(s.shared_blks_hit+s.shared_blks_read,0),1),0)::float8,
		       s.query
		FROM pg_stat_statements s LEFT JOIN pg_database d ON d.oid = s.dbid
		ORDER BY s.total_exec_time DESC LIMIT 50`)
	if err != nil {
		// pg_stat_statements 未启用时返回状态而非报错
		if strings.Contains(err.Error(), "shared_preload_libraries") {
			service.Success(w, TopSQL{Enabled: false, PendingRestart: true, Items: []TopSQLItem{}})
			return
		}
		if strings.Contains(err.Error(), "does not exist") {
			service.Success(w, TopSQL{Enabled: false, Items: []TopSQLItem{}})
			return
		}
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	items := make([]TopSQLItem, 0)
	for rows.Next() {
		var item TopSQLItem
		if err = rows.Scan(&item.Database, &item.Calls, &item.TotalMs, &item.MeanMs, &item.Rows, &item.HitRate, &item.Query); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app top sql viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"))

	service.Success(w, TopSQL{Enabled: true, Items: items})
}

// EnableTopSQL 启用 pg_stat_statements
func (s *App) EnableTopSQL(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(s.configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 将 pg_stat_statements 加入 shared_preload_libraries，重启后生效
	var libs []string
	for lib := range strings.SplitSeq(confval.Postgres.Get(config, "shared_preload_libraries"), ",") {
		if lib = strings.TrimSpace(lib); lib != "" {
			libs = append(libs, lib)
		}
	}
	configChanged := !slices.Contains(libs, "pg_stat_statements")
	if configChanged {
		libs = append(libs, "pg_stat_statements")
		config = confval.Postgres.Set(config, "shared_preload_libraries", strings.Join(libs, ","))
		if err = io.Write(s.configPath(), config, 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	if _, err = postgres.Exec(r.Context(), `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app top sql enabled"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("extension", "pg_stat_statements"), slog.String("path", s.configPath()), slog.Bool("config_changed", configChanged))

	service.Success(w, nil)
}

// ResetTopSQL 重置 SQL 性能统计
func (s *App) ResetTopSQL(w http.ResponseWriter, r *http.Request) {
	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	if _, err = postgres.Exec(r.Context(), `SELECT pg_stat_statements_reset()`); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app top sql reset"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("extension", "pg_stat_statements"))

	service.Success(w, nil)
}

// DatabaseList 获取可连接的数据库列表
func (s *App) DatabaseList(w http.ResponseWriter, r *http.Request) {
	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	rows, err := postgres.Query(r.Context(), `SELECT datname FROM pg_database WHERE datallowconn ORDER BY datname`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	databases := make([]string, 0)
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		databases = append(databases, name)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, databases)
}

// BloatList 获取指定数据库的表膨胀情况
func (s *App) BloatList(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[BloatQuery](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	postgres, err := s.connect(r.Context(), req.Database)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	rows, err := postgres.Query(r.Context(), `
		SELECT schemaname, relname, pg_size_pretty(pg_total_relation_size(relid)),
		       pg_total_relation_size(relid), n_live_tup, n_dead_tup,
		       coalesce(round(100.0*n_dead_tup/nullif(n_live_tup+n_dead_tup,0),1),0)::float8,
		       coalesce(to_char(last_vacuum,'YYYY-MM-DD HH24:MI'),''), coalesce(to_char(last_autovacuum,'YYYY-MM-DD HH24:MI'),''),
		       coalesce(to_char(last_analyze,'YYYY-MM-DD HH24:MI'),''), coalesce(to_char(last_autoanalyze,'YYYY-MM-DD HH24:MI'),'')
		FROM pg_stat_user_tables ORDER BY n_dead_tup DESC LIMIT 50`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	items := make([]BloatItem, 0)
	for rows.Next() {
		var item BloatItem
		if err = rows.Scan(&item.Schema, &item.Table, &item.Size, &item.SizeBytes, &item.LiveTuples, &item.DeadTuples,
			&item.DeadRate, &item.LastVacuum, &item.LastAutovacuum, &item.LastAnalyze, &item.LastAutoanalyz); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app table list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("database", req.Database))

	service.Success(w, Bloat{
		RepackInstalled: io.Exists(app.Root + "/server/postgresql/share/extension/pg_repack.control"),
		Items:           items,
	})
}

// RunMaintenance 对表执行维护操作（异步任务）
func (s *App) RunMaintenance(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[MaintenanceRun](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if len(req.Tables) == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("no tables selected"))
		return
	}
	if !slices.Contains([]string{"vacuum", "vacuum_full", "analyze", "repack"}, req.Operation) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid operation"))
		return
	}

	postgresPassword, err := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load PostgreSQL postgres password: %v", err))
		return
	}
	port := db.PostgresPort(app.Root)
	env := "PGPASSWORD=" + shell.Quote(postgresPassword)

	var cmd string
	if req.Operation == "repack" {
		if !io.Exists(app.Root + "/server/postgresql/share/extension/pg_repack.control") {
			service.Error(w, http.StatusUnprocessableEntity, s.t.Get("pg_repack is not installed, please install it in the extensions tab first"))
			return
		}
		var args strings.Builder
		for _, table := range req.Tables {
			fmt.Fprintf(&args, ` -t "%s.%s"`, table.Schema, table.Table)
		}
		cmd = fmt.Sprintf(`%s %s/server/postgresql/bin/pg_repack -h 127.0.0.1 -p %d -U postgres -d "%s"%s`, env, app.Root, port, req.Database, args.String())
	} else {
		statement := map[string]string{"vacuum": "VACUUM", "vacuum_full": "VACUUM FULL", "analyze": "ANALYZE"}[req.Operation]
		var args strings.Builder
		for _, table := range req.Tables {
			fmt.Fprintf(&args, ` -c "%s \"%s\".\"%s\""`, statement, table.Schema, table.Table)
		}
		cmd = fmt.Sprintf(`%s psql -h 127.0.0.1 -p %d -U postgres -d "%s"%s`, env, port, req.Database, args.String())
	}

	task := new(biz.Task)
	task.Key = "postgresql:maintenance:" + req.Database
	task.Name = s.t.Get("Run %[1]s on %[2]d tables in database %[3]s", req.Operation, len(req.Tables), req.Database)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tables := lo.Map(req.Tables, func(table MaintenanceTable, _ int) string {
		return table.Schema + "." + table.Table
	})
	s.log.Info(s.t.Get("app maintenance queued"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("database", req.Database), slog.String("operation", req.Operation), slog.Any("tables", tables), slog.Uint64("task_id", uint64(task.ID)))

	service.Success(w, nil)
}

// WalStatus 获取 WAL 及复制状态
func (s *App) WalStatus(w http.ResponseWriter, r *http.Request) {
	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	var wal Wal
	if err = postgres.QueryRow(r.Context(), `SELECT pg_size_pretty(coalesce(sum(size),0)) FROM pg_ls_waldir()`).Scan(&wal.WalSize); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = postgres.QueryRow(r.Context(), `SELECT archived_count, failed_count, coalesce(last_archived_wal,''), coalesce(last_failed_wal,'') FROM pg_stat_archiver`).Scan(
		&wal.Archiver.ArchivedCount, &wal.Archiver.FailedCount, &wal.Archiver.LastArchivedWal, &wal.Archiver.LastFailedWal); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	slotRows, err := postgres.Query(r.Context(), `SELECT slot_name, slot_type, active, coalesce(pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)),'') FROM pg_replication_slots`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = slotRows.Close() }()
	wal.Slots = make([]ReplicationSlot, 0)
	for slotRows.Next() {
		var slot ReplicationSlot
		if err = slotRows.Scan(&slot.Name, &slot.Type, &slot.Active, &slot.RetainedWal); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		wal.Slots = append(wal.Slots, slot)
	}
	if err = slotRows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	replRows, err := postgres.Query(r.Context(), `SELECT coalesce(client_addr::text,''), coalesce(state,''), coalesce(sync_state,''), coalesce(pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)),'') FROM pg_stat_replication`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = replRows.Close() }()
	wal.Replications = make([]Replication, 0)
	for replRows.Next() {
		var repl Replication
		if err = replRows.Scan(&repl.ClientAddr, &repl.State, &repl.SyncState, &repl.Lag); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		wal.Replications = append(wal.Replications, repl)
	}
	if err = replRows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, wal)
}

// DropReplicationSlot 删除复制槽
func (s *App) DropReplicationSlot(w http.ResponseWriter, r *http.Request) {
	slot := chi.URLParam(r, "slot")
	if slot == "" {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid slot name"))
		return
	}

	postgres, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer postgres.Close()

	if _, err = postgres.Exec(r.Context(), `SELECT pg_drop_replication_slot($1)`, slot); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("postgresql replication slot deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "postgresql"), slog.String("slot", slot))

	service.Success(w, nil)
}

// getExtensions 返回所有扩展定义
func (s *App) getExtensions() []Extension {
	return []Extension{
		{Name: "pgvector", Slug: "pgvector", ExtName: "vector", Description: s.t.Get("Vector similarity search")},
		{Name: "PostGIS", Slug: "postgis", ExtName: "postgis", Description: s.t.Get("Spatial and geographic objects support")},
		{Name: "TimescaleDB", Slug: "timescaledb", ExtName: "timescaledb", Description: s.t.Get("Time-series database engine, requires restarting PostgreSQL after installation")},
		{Name: "zhparser", Slug: "zhparser", ExtName: "zhparser", Description: s.t.Get("Chinese full-text search parser based on SCWS")},
		{Name: "pg_repack", Slug: "pg_repack", ExtName: "pg_repack", Description: s.t.Get("Reorganize tables online to remove bloat")},
		{Name: "pg_cron", Slug: "pg_cron", ExtName: "pg_cron", Description: s.t.Get("Run periodic jobs inside the database, requires restarting PostgreSQL after installation")},
		{Name: "pg_partman", Slug: "pg_partman", ExtName: "pg_partman", Description: s.t.Get("Automated partition management")},
		{Name: "pgaudit", Slug: "pgaudit", ExtName: "pgaudit", Description: s.t.Get("Session and object audit logging, requires restarting PostgreSQL after installation")},
		{Name: "pg_hint_plan", Slug: "pg_hint_plan", ExtName: "pg_hint_plan", Description: s.t.Get("Control execution plans with hints in SQL comments, requires restarting PostgreSQL after installation")},
		{Name: "pg_stat_monitor", Slug: "pg_stat_monitor", ExtName: "pg_stat_monitor", Description: s.t.Get("Advanced query performance monitoring, requires restarting PostgreSQL after installation")},
		{Name: "pg_ivm", Slug: "pg_ivm", ExtName: "pg_ivm", Description: s.t.Get("Incremental view maintenance for materialized views")},
		{Name: "hypopg", Slug: "hypopg", ExtName: "hypopg", Description: s.t.Get("Hypothetical indexes for query plan testing")},
		{Name: "pgmq", Slug: "pgmq", ExtName: "pgmq", Description: s.t.Get("Lightweight message queue")},
		{Name: "orafce", Slug: "orafce", ExtName: "orafce", Description: s.t.Get("Oracle compatibility functions")},
		{Name: "http", Slug: "http", ExtName: "http", Description: s.t.Get("HTTP client for SQL, send requests from the database")},
	}
}

// checkExtension 检查 slug 是否有效
func (s *App) checkExtension(slug string) bool {
	return lo.ContainsBy(s.getExtensions(), func(e Extension) bool {
		return e.Slug == slug
	})
}

// connect 以 postgres 超级用户连接指定数据库，默认 postgres 库
func (s *App) connect(ctx context.Context, database ...string) (db.Operator, error) {
	password, err := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		return nil, err
	}

	return db.NewPostgres(ctx, "postgres", password, "127.0.0.1", db.PostgresPort(app.Root), database...)
}

func (s *App) configPath() string {
	return app.Root + "/server/postgresql/data/postgresql.conf"
}

// parsePort 从 config 内容解析端口，未配置时返回默认值
func (s *App) parsePort(config string) uint {
	port := cast.ToUint(confval.Postgres.Get(config, "port"))
	if port == 0 {
		return 5432
	}
	return port
}

// applyConfig 让 PostgreSQL 配置生效
func (s *App) applyConfig(ctx context.Context, newConfig string, oldPort uint) error {
	// 配置已落盘，且改端口后还要回写面板记录，整段不跟随请求取消
	ctx = context.WithoutCancel(ctx)
	newPort := s.parsePort(newConfig)
	if oldPort == newPort {
		return systemctl.Reload(ctx, "postgresql")
	}
	if err := systemctl.Restart(ctx, "postgresql"); err != nil {
		return err
	}
	return s.databaseServerRepo.UpdatePort("local_postgresql", newPort)
}
