package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"go.yaml.in/yaml/v4"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t        *gotext.Locale
	conf     *config.Config
	taskRepo biz.TaskRepo
}

func NewApp(t *gotext.Locale, conf *config.Config, taskRepo biz.TaskRepo) *App {
	return &App{t: t, conf: conf, taskRepo: taskRepo}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	// Exporters 管理
	r.Get("/exporters", s.ExporterList)
	r.Post("/exporters", s.InstallExporter)
	r.Delete("/exporters", s.UninstallExporter)
	r.Post("/exporters/{slug}/start", s.StartExporter)
	r.Post("/exporters/{slug}/stop", s.StopExporter)
	r.Post("/exporters/{slug}/restart", s.RestartExporter)
	r.Get("/exporters/{slug}/config", s.GetExporterConfig)
	r.Post("/exporters/{slug}/config", s.UpdateExporterConfig)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status("prometheus")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get prometheus status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	resp, err := client.R().Get("http://127.0.0.1:9090/api/v1/status/runtimeinfo")
	if err != nil || !resp.IsSuccess() {
		service.Success(w, []types.NV{})
		return
	}

	var result struct {
		Data struct {
			StartTime           string `json:"startTime"`
			GoroutineCount      int    `json:"goroutineCount"`
			GOMAXPROCS          int    `json:"GOMAXPROCS"`
			StorageRetention    string `json:"storageRetention"`
			ReloadConfigSuccess bool   `json:"reloadConfigSuccess"`
		} `json:"data"`
	}
	if err = json.Unmarshal(resp.Bytes(), &result); err != nil {
		service.Success(w, []types.NV{})
		return
	}

	data := []types.NV{
		{Name: s.t.Get("Start Time"), Value: result.Data.StartTime},
		{Name: s.t.Get("Storage Retention"), Value: result.Data.StorageRetention},
		{Name: s.t.Get("Goroutine Count"), Value: cast.ToString(result.Data.GoroutineCount)},
		{Name: "GOMAXPROCS", Value: cast.ToString(result.Data.GOMAXPROCS)},
		{Name: s.t.Get("Config Reload Success"), Value: cast.ToString(result.Data.ReloadConfigSuccess)},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/prometheus/prometheus.yml", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/prometheus/prometheus.yml", app.Root), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("prometheus"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetConfigTune 获取 Prometheus 全局配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/prometheus/prometheus.yml", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var cfg struct {
		Global struct {
			ScrapeInterval     string `yaml:"scrape_interval"`
			EvaluationInterval string `yaml:"evaluation_interval"`
			ScrapeTimeout      string `yaml:"scrape_timeout"`
		} `yaml:"global"`
	}
	if err = yaml.Unmarshal([]byte(config), &cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		ScrapeInterval:     cfg.Global.ScrapeInterval,
		EvaluationInterval: cfg.Global.EvaluationInterval,
		ScrapeTimeout:      cfg.Global.ScrapeTimeout,
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Prometheus 全局配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/prometheus/prometheus.yml", app.Root)
	raw, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var cfg map[string]any
	if err = yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	global, ok := cfg["global"].(map[string]any)
	if !ok {
		global = make(map[string]any)
	}

	if req.ScrapeInterval != "" {
		global["scrape_interval"] = req.ScrapeInterval
	}
	if req.EvaluationInterval != "" {
		global["evaluation_interval"] = req.EvaluationInterval
	}
	if req.ScrapeTimeout != "" {
		global["scrape_timeout"] = req.ScrapeTimeout
	}
	cfg["global"] = global

	data, err := yaml.Marshal(cfg)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(confPath, string(data), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("prometheus"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// getExporters 返回所有 exporter 定义
func (s *App) getExporters() []Exporter {
	return []Exporter{
		{Name: "Node Exporter", Slug: "node_exporter", Description: s.t.Get("Hardware and OS metrics")},
		{Name: "MySQL Exporter", Slug: "mysqld_exporter", Description: s.t.Get("MySQL database metrics")},
		{Name: "PostgreSQL Exporter", Slug: "postgres_exporter", Description: s.t.Get("PostgreSQL database metrics")},
		{Name: "Redis Exporter", Slug: "redis_exporter", Description: s.t.Get("Redis metrics")},
		{Name: "Memcached Exporter", Slug: "memcached_exporter", Description: s.t.Get("Memcached metrics")},
		{Name: "Nginx Exporter", Slug: "nginx-prometheus-exporter", Description: s.t.Get("Nginx metrics")},
	}
}

// checkExporter 检查 slug 是否有效
func (s *App) checkExporter(slug string) bool {
	for _, e := range s.getExporters() {
		if e.Slug == slug {
			return true
		}
	}
	return false
}

// ExporterList 获取 Exporter 列表及状态
func (s *App) ExporterList(w http.ResponseWriter, r *http.Request) {
	exporters := s.getExporters()
	for i := range exporters {
		exporters[i].Installed = io.Exists(fmt.Sprintf("%s/server/prometheus/exporters/%s", app.Root, exporters[i].Slug))
		if exporters[i].Installed {
			running, _ := systemctl.Status(exporters[i].Slug)
			exporters[i].Running = running
		}
	}

	service.Success(w, exporters)
}

// InstallExporter 安装 Exporter（异步任务）
func (s *App) InstallExporter(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ExporterSlug](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !s.checkExporter(req.Slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", req.Slug))
		return
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/prometheus/exporters/exporter.sh' | bash -s -- 'install' '%s'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug))

	task := new(biz.Task)
	task.Name = s.t.Get("Install Prometheus exporter %s", req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// UninstallExporter 卸载 Exporter（异步任务）
func (s *App) UninstallExporter(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ExporterSlug](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !s.checkExporter(req.Slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", req.Slug))
		return
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/prometheus/exporters/exporter.sh' | bash -s -- 'uninstall' '%s'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug))

	task := new(biz.Task)
	task.Name = s.t.Get("Uninstall Prometheus exporter %s", req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// StartExporter 启动 Exporter
func (s *App) StartExporter(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !s.checkExporter(slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", slug))
		return
	}

	if err := systemctl.Start(slug); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// StopExporter 停止 Exporter
func (s *App) StopExporter(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !s.checkExporter(slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", slug))
		return
	}

	if err := systemctl.Stop(slug); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// RestartExporter 重启 Exporter
func (s *App) RestartExporter(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !s.checkExporter(slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", slug))
		return
	}

	if err := systemctl.Restart(slug); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetExporterConfig 获取 Exporter 配置
func (s *App) GetExporterConfig(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !s.checkExporter(slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", slug))
		return
	}

	confPath := fmt.Sprintf("%s/server/prometheus/exporters/%s/%s.conf", app.Root, slug, slug)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

// UpdateExporterConfig 更新 Exporter 配置
func (s *App) UpdateExporterConfig(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if !s.checkExporter(slug) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("exporter %s does not exist", slug))
		return
	}

	req, err := service.Bind[ExporterConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/prometheus/exporters/%s/%s.conf", app.Root, slug, slug)
	if err = io.Write(confPath, req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(slug); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
