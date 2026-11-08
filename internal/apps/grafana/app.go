package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
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
	t   *gotext.Locale
	log *slog.Logger
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		t:   t,
		log: log,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	// 数据源管理
	r.Get("/datasources", s.DataSourceList)
	r.Post("/datasources", s.CreateDataSource)
	r.Post("/datasources/{name}", s.UpdateDataSource)
	r.Delete("/datasources/{name}", s.DeleteDataSource)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "grafana")
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status(r.Context(), "grafana")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get grafana status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	// 从 defaults.ini 获取端口
	config, _ := io.Read(s.configPath())
	port := confval.SectionINI.GetIn(config, "server", "http_port")
	if port == "" {
		port = "3000"
	}

	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	resp, err := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/api/health", port))
	if err != nil || !resp.IsStatusSuccess() {
		service.Success(w, []types.NV{})
		return
	}

	var health struct {
		Commit   string `json:"commit"`
		Database string `json:"database"`
		Version  string `json:"version"`
	}
	if err = json.Unmarshal(resp.Bytes(), &health); err != nil {
		service.Success(w, []types.NV{})
		return
	}

	data := []types.NV{
		{Name: s.t.Get("Version"), Value: health.Version},
		{Name: "Commit", Value: health.Commit},
		{Name: s.t.Get("Database"), Value: health.Database},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "grafana"), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "grafana", s.configPath(), 0644, "grafana")
}

// GetConfigTune 获取 Grafana 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, _ := io.Read(s.configPath())

	get := func(section, key string) string {
		return confval.SectionINI.GetIn(config, section, key)
	}

	tune := ConfigTune{
		// [server]
		HTTPPort: get("server", "http_port"),
		Domain:   get("server", "domain"),
		RootURL:  get("server", "root_url"),
		Protocol: get("server", "protocol"),
		// [database]
		DBType:     get("database", "type"),
		DBHost:     get("database", "host"),
		DBName:     get("database", "name"),
		DBUser:     get("database", "user"),
		DBPassword: get("database", "password"),
		// [security]
		AdminUser:     get("security", "admin_user"),
		AdminPassword: get("security", "admin_password"),
		// [users]
		AllowSignUp:       get("users", "allow_sign_up"),
		AutoAssignOrgRole: get("users", "auto_assign_org_role"),
		// [smtp]
		SMTPEnabled:     get("smtp", "enabled"),
		SMTPHost:        get("smtp", "host"),
		SMTPUser:        get("smtp", "user"),
		SMTPPassword:    get("smtp", "password"),
		SMTPFromAddress: get("smtp", "from_address"),
		// [log]
		LogMode:  get("log", "mode"),
		LogLevel: get("log", "level"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "grafana"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Grafana 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, _ := io.Read(s.configPath())

	// [server]
	config = confval.SectionINI.SetIn(config, "server", "http_port", req.HTTPPort)
	config = confval.SectionINI.SetIn(config, "server", "domain", req.Domain)
	config = confval.SectionINI.SetIn(config, "server", "root_url", req.RootURL)
	config = confval.SectionINI.SetIn(config, "server", "protocol", req.Protocol)
	// [database]
	config = confval.SectionINI.SetIn(config, "database", "type", req.DBType)
	config = confval.SectionINI.SetIn(config, "database", "host", req.DBHost)
	config = confval.SectionINI.SetIn(config, "database", "name", req.DBName)
	config = confval.SectionINI.SetIn(config, "database", "user", req.DBUser)
	config = confval.SectionINI.SetIn(config, "database", "password", req.DBPassword)
	// [security]
	config = confval.SectionINI.SetIn(config, "security", "admin_user", req.AdminUser)
	config = confval.SectionINI.SetIn(config, "security", "admin_password", req.AdminPassword)
	// [users]
	config = confval.SectionINI.SetIn(config, "users", "allow_sign_up", req.AllowSignUp)
	config = confval.SectionINI.SetIn(config, "users", "auto_assign_org_role", req.AutoAssignOrgRole)
	// [smtp]
	config = confval.SectionINI.SetIn(config, "smtp", "enabled", req.SMTPEnabled)
	config = confval.SectionINI.SetIn(config, "smtp", "host", req.SMTPHost)
	config = confval.SectionINI.SetIn(config, "smtp", "user", req.SMTPUser)
	config = confval.SectionINI.SetIn(config, "smtp", "password", req.SMTPPassword)
	config = confval.SectionINI.SetIn(config, "smtp", "from_address", req.SMTPFromAddress)
	// [log]
	config = confval.SectionINI.SetIn(config, "log", "mode", req.LogMode)
	config = confval.SectionINI.SetIn(config, "log", "level", req.LogLevel)

	if err = io.Write(s.configPath(), config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "grafana"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", "grafana"),
		slog.String("path", s.configPath()),
		slog.String("http_port", req.HTTPPort),
		slog.String("domain", req.Domain),
		slog.String("protocol", req.Protocol),
		slog.String("db_type", req.DBType),
		slog.String("admin_user", req.AdminUser),
	)

	service.Success(w, nil)
}

// DataSourceList 获取数据源列表
func (s *App) DataSourceList(w http.ResponseWriter, r *http.Request) {
	list := s.getDatasourceList(s.readDatasources())

	s.log.Info(s.t.Get("grafana datasource list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "grafana"), slog.String("path", s.datasourcePath()))

	service.Success(w, list)
}

// CreateDataSource 创建数据源
func (s *App) CreateDataSource(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[DataSource](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cfg := s.readDatasources()

	if lo.ContainsBy(s.getDatasourceList(cfg), func(item any) bool {
		ds, ok := item.(map[string]any)
		return ok && ds["name"] == req.Name
	}) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("datasource %s already exists", req.Name))
		return
	}

	if req.IsDefault {
		s.clearDefault(cfg)
	}

	list := s.getDatasourceList(cfg)
	list = append(list, s.buildDatasourceMap(req))
	cfg["datasources"] = list

	if err = s.writeDatasources(r.Context(), cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("grafana datasource created"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", "grafana"),
		slog.String("name", req.Name),
		slog.String("datasource_type", req.Type),
		slog.String("url", req.URL),
		slog.Bool("is_default", req.IsDefault),
	)

	service.Success(w, nil)
}

// UpdateDataSource 更新数据源
func (s *App) UpdateDataSource(w http.ResponseWriter, r *http.Request) {
	oldName := chi.URLParam(r, "name")
	req, err := service.Bind[DataSource](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cfg := s.readDatasources()
	list := s.getDatasourceList(cfg)

	_, idx, found := lo.FindIndexOf(list, func(item any) bool {
		ds, ok := item.(map[string]any)
		return ok && ds["name"] == oldName
	})
	if !found {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("datasource %s not found", oldName))
		return
	}
	ds, _ := list[idx].(map[string]any)
	newDs := s.buildDatasourceMap(req)
	// 密码为空时保留原有 secureJsonData
	if req.Password == "" {
		if sec, exists := ds["secureJsonData"]; exists {
			newDs["secureJsonData"] = sec
		}
	}
	list[idx] = newDs

	if req.IsDefault {
		s.clearDefault(cfg)
		lo.ForEach(list, func(item any, _ int) {
			if ds, ok := item.(map[string]any); ok && ds["name"] == req.Name {
				ds["isDefault"] = true
			}
		})
	}

	if oldName != req.Name {
		s.addDeleteEntry(cfg, oldName)
	}

	cfg["datasources"] = list

	if err = s.writeDatasources(r.Context(), cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("grafana datasource updated"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", "grafana"),
		slog.String("old_name", oldName),
		slog.String("name", req.Name),
		slog.String("datasource_type", req.Type),
		slog.String("url", req.URL),
		slog.Bool("is_default", req.IsDefault),
	)

	service.Success(w, nil)
}

// DeleteDataSource 删除数据源
func (s *App) DeleteDataSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg := s.readDatasources()
	list := s.getDatasourceList(cfg)

	newList := lo.Filter(list, func(item any, _ int) bool {
		ds, ok := item.(map[string]any)
		return !ok || ds["name"] != name
	})
	found := len(newList) < len(list)
	if !found {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("datasource %s not found", name))
		return
	}

	cfg["datasources"] = newList
	s.addDeleteEntry(cfg, name)

	if err := s.writeDatasources(r.Context(), cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("grafana datasource deleted"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", "grafana"),
		slog.String("name", name),
	)

	service.Success(w, nil)
}

// configPath 返回 Grafana 主配置文件路径
func (s *App) configPath() string {
	return app.Root + "/server/grafana/conf/defaults.ini"
}

// datasourcePath 返回 provisioning 数据源文件路径
func (s *App) datasourcePath() string {
	return app.Root + "/server/grafana/conf/provisioning/datasources/panel.yml"
}

// readDatasources 读取 provisioning 文件
func (s *App) readDatasources() map[string]any {
	raw, _ := io.Read(s.datasourcePath())
	if raw == "" {
		return map[string]any{
			"apiVersion":  1,
			"datasources": []any{},
		}
	}
	var cfg map[string]any
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return map[string]any{
			"apiVersion":  1,
			"datasources": []any{},
		}
	}
	return cfg
}

// writeDatasources 写入 provisioning 文件并重启 Grafana
func (s *App) writeDatasources(ctx context.Context, cfg map[string]any) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err = io.Write(s.datasourcePath(), string(data), 0644); err != nil {
		return err
	}
	return systemctl.Restart(ctx, "grafana")
}

// getDatasourceList 从 cfg 中提取 datasources 切片
func (s *App) getDatasourceList(cfg map[string]any) []any {
	ds, ok := cfg["datasources"].([]any)
	if !ok {
		return []any{}
	}
	return ds
}

// buildDatasourceMap 从请求构建单条 datasource map
func (s *App) buildDatasourceMap(req *DataSource) map[string]any {
	ds := map[string]any{
		"name":      req.Name,
		"type":      req.Type,
		"access":    req.Access,
		"url":       req.URL,
		"isDefault": req.IsDefault,
		"editable":  true,
	}
	if req.Access == "" {
		ds["access"] = "proxy"
	}
	switch req.Type {
	case "mysql", "postgres", "influxdb", "mssql":
		if req.Database != "" {
			ds["database"] = req.Database
		}
		if req.User != "" {
			ds["user"] = req.User
		}
		if req.Password != "" {
			ds["secureJsonData"] = map[string]any{"password": req.Password}
		}
	}
	return ds
}

// clearDefault 清除所有数据源的默认标记
func (s *App) clearDefault(cfg map[string]any) {
	lo.ForEach(s.getDatasourceList(cfg), func(item any, _ int) {
		if ds, ok := item.(map[string]any); ok {
			ds["isDefault"] = false
		}
	})
}

// addDeleteEntry 向 deleteDatasources 添加条目
func (s *App) addDeleteEntry(cfg map[string]any, name string) {
	delList, _ := cfg["deleteDatasources"].([]any)
	delList = append(delList, map[string]any{"name": name, "orgId": 1})
	cfg["deleteDatasources"] = delList
}
