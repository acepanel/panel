package postgresql

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/systemctl"
	"github.com/tnborg/panel/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/user_config", s.GetUserConfig)
	r.Post("/user_config", s.UpdateUserConfig)
	r.Get("/load", s.Load)
	r.Get("/log", s.Log)
	r.Post("/clear_log", s.ClearLog)
}

// GetConfig 获取配置
func (s *App) GetConfig(c fiber.Ctx) error {
	// 获取配置
	config, err := io.Read(fmt.Sprintf("%s/server/postgresql/data/postgresql.conf", app.Root))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(fmt.Sprintf("%s/server/postgresql/data/postgresql.conf", app.Root), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Reload("postgresql"); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to reload PostgreSQL: %v", err))
	}

	return service.Success(c, nil)
}

// GetUserConfig 获取用户配置
func (s *App) GetUserConfig(c fiber.Ctx) error {
	// 获取配置
	config, err := io.Read(fmt.Sprintf("%s/server/postgresql/data/pg_hba.conf", app.Root))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

// UpdateUserConfig 保存用户配置
func (s *App) UpdateUserConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(fmt.Sprintf("%s/server/postgresql/data/pg_hba.conf", app.Root), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Reload("postgresql"); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to reload PostgreSQL: %v", err))
	}

	return service.Success(c, nil)
}

// Load 获取负载
func (s *App) Load(c fiber.Ctx) error {
	status, _ := systemctl.Status("postgresql")
	if !status {
		return service.Success(c, []types.NV{})
	}

	start, err := shell.Execf(`echo "select pg_postmaster_start_time();" | su - postgres -c "psql" | sed -n 3p | cut -d'.' -f1`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL start time: %v", err))
	}
	pid, err := shell.Execf(`echo "select pg_backend_pid();" | su - postgres -c "psql" | sed -n 3p`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL backend pid: %v", err))
	}
	process, err := shell.Execf(`ps aux | grep postgres | grep -v grep | wc -l`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL process: %v", err))
	}
	connections, err := shell.Execf(`echo "SELECT count(*) FROM pg_stat_activity WHERE NOT pid=pg_backend_pid();" | su - postgres -c "psql" | sed -n 3p`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL connections: %v", err))
	}
	storage, err := shell.Execf(`echo "select pg_size_pretty(pg_database_size('postgres'));" | su - postgres -c "psql" | sed -n 3p`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get PostgreSQL database size: %v", err))
	}

	data := []types.NV{
		{Name: s.t.Get("Start Time"), Value: start},
		{Name: s.t.Get("Process PID"), Value: pid},
		{Name: s.t.Get("Process Count"), Value: process},
		{Name: s.t.Get("Total Connections"), Value: connections},
		{Name: s.t.Get("Storage Usage"), Value: storage},
	}

	return service.Success(c, data)
}

// Log 获取日志
func (s *App) Log(c fiber.Ctx) error {
	return service.Success(c, fmt.Sprintf("%s/server/postgresql/logs/postgresql-%s.log", app.Root, time.Now().Format(time.DateOnly)))
}

// ClearLog 清空日志
func (s *App) ClearLog(c fiber.Ctx) error {
	if _, err := shell.Execf("rm -rf %s/server/postgresql/logs/postgresql-*.log", app.Root); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}
