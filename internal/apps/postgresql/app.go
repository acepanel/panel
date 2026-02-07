package postgresql

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/acepanel/panel/pkg/db"
	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/types"
)

type App struct {
	t                  *gotext.Locale
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
}

func NewApp(t *gotext.Locale, setting biz.SettingRepo, databaseServer biz.DatabaseServerRepo) *App {
	return &App{
		t:                  t,
		settingRepo:        setting,
		databaseServerRepo: databaseServer,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/user_config", s.GetUserConfig)
	r.Post("/user_config", s.UpdateUserConfig)
	r.Get("/load", s.Load)
	r.Get("/log", s.Log)
	r.Post("/clear_log", s.ClearLog)
	r.Get("/postgres_password", s.GetPostgresPassword)
	r.Post("/postgres_password", s.SetPostgresPassword)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	// 获取配置
	config, err := io.Read(fmt.Sprintf("%s/server/postgresql/data/postgresql.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/postgresql/data/postgresql.conf", app.Root), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("postgresql"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload PostgreSQL: %v", err))
		return
	}

	service.Success(w, nil)
}

// GetUserConfig 获取用户配置
func (s *App) GetUserConfig(w http.ResponseWriter, r *http.Request) {
	// 获取配置
	config, err := io.Read(fmt.Sprintf("%s/server/postgresql/data/pg_hba.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

// UpdateUserConfig 保存用户配置
func (s *App) UpdateUserConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/postgresql/data/pg_hba.conf", app.Root), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("postgresql"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload PostgreSQL: %v", err))
		return
	}

	service.Success(w, nil)
}

// Load 获取负载
func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, _ := systemctl.Status("postgresql")
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	postgresPassword, err := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load PostgreSQL postgres password: %v", err))
		return
	}

	if err = os.Setenv("PGPASSWORD", postgresPassword); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set PGPASSWORD env: %v", err))
		return
	}

	start, err := shell.Execf(`psql -h 127.0.0.1 -U postgres -t -c "select pg_postmaster_start_time();" | head -1 | cut -d'.' -f1`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL start time: %v", err))
		return
	}
	pid, err := shell.Execf(`psql -h 127.0.0.1 -U postgres -t -c "select pg_backend_pid();"`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL backend pid: %v", err))
		return
	}
	process, err := shell.Execf(`ps aux | grep postgres | grep -v grep | wc -l`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL process: %v", err))
		return
	}
	connections, err := shell.Execf(`psql -h 127.0.0.1 -U postgres -t -c "SELECT count(*) FROM pg_stat_activity WHERE NOT pid=pg_backend_pid();"`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL connections: %v", err))
		return
	}
	storage, err := shell.Execf(`psql -h 127.0.0.1 -U postgres -t -c "select pg_size_pretty(pg_database_size('postgres'));"`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL database size: %v", err))
		return
	}

	if err = os.Unsetenv("PGPASSWORD"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to unset PGPASSWORD env: %v", err))
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

// Log 获取日志
func (s *App) Log(w http.ResponseWriter, r *http.Request) {
	service.Success(w, fmt.Sprintf("%s/server/postgresql/logs/postgresql-%s.log", app.Root, time.Now().Format(time.DateOnly)))
}

// ClearLog 清空日志
func (s *App) ClearLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf("rm -rf %s/server/postgresql/logs/postgresql-*.log", app.Root); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetPostgresPassword 获取 postgres 用户密码
func (s *App) GetPostgresPassword(w http.ResponseWriter, r *http.Request) {
	password, err := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get postgres password: %v", err))
		return
	}

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
	postgres, err := db.NewPostgres("postgres", oldPassword, "127.0.0.1", 5432)
	if err != nil {
		// 直接修改密码
		if _, err = shell.Execf(`su - postgres -c "psql -c \"ALTER USER postgres WITH PASSWORD '%s';\""`, req.Password); err != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set postgres password: %v", err))
			return
		}
	} else {
		defer postgres.Close()
		if err = postgres.UserPassword("postgres", req.Password); err != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set postgres password: %v", err))
			return
		}
	}

	if err = s.settingRepo.Set(biz.SettingKeyPostgresPassword, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set postgres password: %v", err))
		return
	}

	_ = s.databaseServerRepo.UpdatePassword("local_postgresql", req.Password)

	service.Success(w, nil)
}

// GetConfigTune 获取 PostgreSQL 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/postgresql/data/postgresql.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		// 连接设置
		ListenAddresses:              s.getPGValue(config, "listen_addresses"),
		Port:                         s.getPGValue(config, "port"),
		MaxConnections:               s.getPGValue(config, "max_connections"),
		SuperuserReservedConnections: s.getPGValue(config, "superuser_reserved_connections"),
		// 内存设置
		SharedBuffers:      s.getPGValue(config, "shared_buffers"),
		WorkMem:            s.getPGValue(config, "work_mem"),
		MaintenanceWorkMem: s.getPGValue(config, "maintenance_work_mem"),
		EffectiveCacheSize: s.getPGValue(config, "effective_cache_size"),
		HugePages:          s.getPGValue(config, "huge_pages"),
		// WAL 设置
		WalLevel:                   s.getPGValue(config, "wal_level"),
		WalBuffers:                 s.getPGValue(config, "wal_buffers"),
		MaxWalSize:                 s.getPGValue(config, "max_wal_size"),
		MinWalSize:                 s.getPGValue(config, "min_wal_size"),
		CheckpointCompletionTarget: s.getPGValue(config, "checkpoint_completion_target"),
		// 查询优化
		DefaultStatisticsTarget: s.getPGValue(config, "default_statistics_target"),
		RandomPageCost:          s.getPGValue(config, "random_page_cost"),
		EffectiveIoConcurrency:  s.getPGValue(config, "effective_io_concurrency"),
		// 日志设置
		LogDestination:          s.getPGValue(config, "log_destination"),
		LogMinDurationStatement: s.getPGValue(config, "log_min_duration_statement"),
		LogTimezone:             s.getPGValue(config, "log_timezone"),
		// IO 设置
		IoMethod: s.getPGValue(config, "io_method"),
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 PostgreSQL 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/postgresql/data/postgresql.conf", app.Root)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新连接设置
	config = s.setPGValue(config, "listen_addresses", req.ListenAddresses)
	config = s.setPGValue(config, "port", req.Port)
	config = s.setPGValue(config, "max_connections", req.MaxConnections)
	config = s.setPGValue(config, "superuser_reserved_connections", req.SuperuserReservedConnections)
	// 更新内存设置
	config = s.setPGValue(config, "shared_buffers", req.SharedBuffers)
	config = s.setPGValue(config, "work_mem", req.WorkMem)
	config = s.setPGValue(config, "maintenance_work_mem", req.MaintenanceWorkMem)
	config = s.setPGValue(config, "effective_cache_size", req.EffectiveCacheSize)
	config = s.setPGValue(config, "huge_pages", req.HugePages)
	// 更新 WAL 设置
	config = s.setPGValue(config, "wal_level", req.WalLevel)
	config = s.setPGValue(config, "wal_buffers", req.WalBuffers)
	config = s.setPGValue(config, "max_wal_size", req.MaxWalSize)
	config = s.setPGValue(config, "min_wal_size", req.MinWalSize)
	config = s.setPGValue(config, "checkpoint_completion_target", req.CheckpointCompletionTarget)
	// 更新查询优化
	config = s.setPGValue(config, "default_statistics_target", req.DefaultStatisticsTarget)
	config = s.setPGValue(config, "random_page_cost", req.RandomPageCost)
	config = s.setPGValue(config, "effective_io_concurrency", req.EffectiveIoConcurrency)
	// 更新日志设置
	config = s.setPGValue(config, "log_destination", req.LogDestination)
	config = s.setPGValue(config, "log_min_duration_statement", req.LogMinDurationStatement)
	config = s.setPGValue(config, "log_timezone", req.LogTimezone)
	// 更新 IO 设置
	config = s.setPGValue(config, "io_method", req.IoMethod)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("postgresql"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload PostgreSQL: %v", err))
		return
	}

	service.Success(w, nil)
}

// getPGValue 从 PostgreSQL 配置内容中获取指定键的值
func (s *App) getPGValue(content string, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k == key {
			v := strings.TrimSpace(parts[1])
			// 去除行尾注释
			if idx := strings.Index(v, "#"); idx >= 0 {
				v = strings.TrimSpace(v[:idx])
			}
			// 去除引号
			v = strings.Trim(v, "'\"")
			return v
		}
	}
	return ""
}

// setPGValue 在 PostgreSQL 配置内容中设置指定键的值
func (s *App) setPGValue(content string, key string, value string) string {
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	lines := strings.Split(content, "\n")
	found := false
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}
		checkLine := trimmed
		if strings.HasPrefix(checkLine, "#") {
			checkLine = strings.TrimSpace(checkLine[1:])
		}
		parts := strings.SplitN(checkLine, "=", 2)
		if len(parts) != 2 {
			result = append(result, line)
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k == key {
			if found {
				continue
			}
			found = true
			// 值为空时注释掉该配置项
			if value == "" {
				if !strings.HasPrefix(trimmed, "#") {
					result = append(result, "#"+line)
				} else {
					result = append(result, line)
				}
				continue
			}
			result = append(result, key+" = "+value)
		} else {
			result = append(result, line)
		}
	}
	if !found && value != "" {
		result = append(result, key+" = "+value)
	}
	return strings.Join(result, "\n")
}
