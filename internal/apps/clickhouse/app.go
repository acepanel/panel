package clickhouse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"go.yaml.in/yaml/v4"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t                  *gotext.Locale
	log                *slog.Logger
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo) *App {
	return &App{
		t:                  t,
		log:                log,
		settingRepo:        settingRepo,
		databaseServerRepo: databaseServerRepo,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	r.Get("/default_password", s.GetDefaultPassword)
	r.Post("/default_password", s.SetDefaultPassword)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "clickhouse-server")
	return types.AggregateAppStatus(ok)
}

// Load 获取 ClickHouse 运行状态
func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, _ := systemctl.Status(r.Context(), "clickhouse-server")
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	password, _ := s.settingRepo.Get(biz.SettingKeyClickHouseDefaultPassword)
	port := s.getPort()

	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)

	// 获取版本
	versionResp, err := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/?query=SELECT+version()&user=default&password=%s", port, password))
	if err != nil || !versionResp.IsStatusSuccess() {
		service.Success(w, []types.NV{})
		return
	}
	version := strings.TrimSpace(string(versionResp.Bytes()))

	// 获取运行时间
	uptimeResp, _ := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/?query=SELECT+uptime()&user=default&password=%s", port, password))
	uptime := strings.TrimSpace(string(uptimeResp.Bytes()))

	// 获取当前查询数
	queriesResp, _ := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/?query=SELECT+value+FROM+system.metrics+WHERE+metric='Query'&user=default&password=%s", port, password))
	queries := strings.TrimSpace(string(queriesResp.Bytes()))

	// 获取内存使用
	memResp, _ := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/?query=SELECT+value+FROM+system.metrics+WHERE+metric='MemoryTracking'&user=default&password=%s", port, password))
	memUsage := strings.TrimSpace(string(memResp.Bytes()))

	// 获取数据库数量
	dbCountResp, _ := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/?query=SELECT+count()+FROM+system.databases&user=default&password=%s", port, password))
	dbCount := strings.TrimSpace(string(dbCountResp.Bytes()))

	// 获取表数量
	tableCountResp, _ := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/?query=SELECT+count()+FROM+system.tables&user=default&password=%s", port, password))
	tableCount := strings.TrimSpace(string(tableCountResp.Bytes()))

	data := []types.NV{
		{Name: s.t.Get("Version"), Value: version},
		{Name: s.t.Get("Uptime (seconds)"), Value: uptime},
		{Name: s.t.Get("Active Queries"), Value: queries},
		{Name: s.t.Get("Memory Usage (bytes)"), Value: memUsage},
		{Name: s.t.Get("Databases"), Value: dbCount},
		{Name: s.t.Get("Tables"), Value: tableCount},
	}

	service.Success(w, data)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "clickhouse"), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

// UpdateConfig 更新配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "clickhouse", s.configPath(), 0644, "clickhouse-server")
}

// GetConfigTune 获取配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	_ = yaml.Unmarshal([]byte(raw), &cfg)
	if cfg == nil {
		cfg = make(map[string]any)
	}

	tune := ConfigTune{
		ListenHost:     confval.GetYAML(cfg, "listen_host"),
		HTTPPort:       confval.GetYAML(cfg, "http_port"),
		TCPPort:        confval.GetYAML(cfg, "tcp_port"),
		MaxMemoryUsage: confval.GetYAML(cfg, "max_memory_usage"),
		MaxThreads:     confval.GetYAML(cfg, "max_threads"),
		Path:           confval.GetYAML(cfg, "path"),
		TmpPath:        confval.GetYAML(cfg, "tmp_path"),
		LogLevel:       confval.GetYAML(cfg, "logger.level"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "clickhouse"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	if err = yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		cfg = make(map[string]any)
	}

	// ClickHouse 顶层键直接用平铺方式
	s.setYAMLValue(cfg, "listen_host", req.ListenHost)
	s.setYAMLValue(cfg, "http_port", req.HTTPPort)
	s.setYAMLValue(cfg, "tcp_port", req.TCPPort)
	s.setYAMLValue(cfg, "max_memory_usage", req.MaxMemoryUsage)
	s.setYAMLValue(cfg, "max_threads", req.MaxThreads)
	s.setYAMLValue(cfg, "path", req.Path)
	s.setYAMLValue(cfg, "tmp_path", req.TmpPath)
	// logger.level 是嵌套的
	s.setNestedYAMLValue(cfg, "logger.level", req.LogLevel)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Write(s.configPath(), string(data), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "clickhouse-server"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "clickhouse"), slog.String("listen_host", req.ListenHost), slog.String("http_port", req.HTTPPort), slog.String("tcp_port", req.TCPPort), slog.String("max_memory_usage", req.MaxMemoryUsage), slog.String("max_threads", req.MaxThreads), slog.String("data_path", req.Path), slog.String("tmp_path", req.TmpPath), slog.String("log_level", req.LogLevel))

	service.Success(w, nil)
}

// GetDefaultPassword 获取 default 用户密码
func (s *App) GetDefaultPassword(w http.ResponseWriter, r *http.Request) {
	password, err := s.settingRepo.Get(biz.SettingKeyClickHouseDefaultPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get ClickHouse default password: %v", err))
		return
	}

	s.log.Info(s.t.Get("app password viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "clickhouse"), slog.String("username", "default"))

	service.Success(w, password)
}

// SetDefaultPassword 设置 default 用户密码
func (s *App) SetDefaultPassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SetDefaultPassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256([]byte(req.Password))
	hexHash := hex.EncodeToString(hash[:])

	// 读取 users.d/default.yaml 并更新密码
	raw, _ := io.Read(s.usersConfigPath())
	var cfg map[string]any
	if err = yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		cfg = make(map[string]any)
	}
	users, _ := cfg["users"].(map[string]any)
	if users == nil {
		users = make(map[string]any)
		cfg["users"] = users
	}
	def, _ := users["default"].(map[string]any)
	if def == nil {
		def = make(map[string]any)
		users["default"] = def
	}
	def["password_sha256_hex"] = hexHash
	out, _ := yaml.Marshal(cfg)
	if err = io.Write(s.usersConfigPath(), string(out), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write ClickHouse user config: %v", err))
		return
	}

	if err = systemctl.Restart(r.Context(), "clickhouse-server"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 保存明文到面板数据库
	if err = s.settingRepo.Set(biz.SettingKeyClickHouseDefaultPassword, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to save ClickHouse default password: %v", err))
		return
	}

	_ = s.databaseServerRepo.UpdatePassword("local_clickhouse", req.Password)

	s.log.Info(s.t.Get("app password updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "clickhouse"), slog.String("username", "default"))

	service.Success(w, nil)
}

// configPath 返回主配置文件路径
func (s *App) configPath() string {
	return app.Root + "/server/clickhouse/config/config.yaml"
}

// usersConfigPath 返回用户密码配置文件路径（users.d/ 由 ConfigProcessor 自动合并到 users.yaml）
func (s *App) usersConfigPath() string {
	return app.Root + "/server/clickhouse/config/users.d/default.yaml"
}

// getPort 从配置中获取 HTTP 端口
func (s *App) getPort() string {
	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	_ = yaml.Unmarshal([]byte(raw), &cfg)
	if cfg != nil {
		if v := confval.GetYAML(cfg, "http_port"); v != "" {
			return v
		}
	}
	return "8123"
}

// setYAMLValue 设置平铺 YAML 值
func (s *App) setYAMLValue(cfg map[string]any, key string, value string) {
	if value == "" {
		return
	}
	cfg[key] = value
}

// setNestedYAMLValue 设置嵌套 YAML 值
func (s *App) setNestedYAMLValue(cfg map[string]any, key string, value string) {
	if value == "" {
		return
	}
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		cfg[parts[0]] = value
		return
	}
	nested, ok := cfg[parts[0]].(map[string]any)
	if !ok {
		nested = make(map[string]any)
		cfg[parts[0]] = nested
	}
	s.setNestedYAMLValue(nested, parts[1], value)
}
