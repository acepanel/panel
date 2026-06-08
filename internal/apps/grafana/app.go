package grafana

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"go.yaml.in/yaml/v4"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{t: t}
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

func (s *App) Status() string {
	ok, _ := systemctl.Status("grafana")
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status("grafana")
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
	port := s.getINIValue(config, "server", "http_port")
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
	config, _ := io.Read(s.configPath())

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(s.configPath(), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("grafana"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetConfigTune 获取 Grafana 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, _ := io.Read(s.configPath())

	get := func(section, key string) string {
		return s.getINIValue(config, section, key)
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
	config = s.setINIValue(config, "server", "http_port", req.HTTPPort)
	config = s.setINIValue(config, "server", "domain", req.Domain)
	config = s.setINIValue(config, "server", "root_url", req.RootURL)
	config = s.setINIValue(config, "server", "protocol", req.Protocol)
	// [database]
	config = s.setINIValue(config, "database", "type", req.DBType)
	config = s.setINIValue(config, "database", "host", req.DBHost)
	config = s.setINIValue(config, "database", "name", req.DBName)
	config = s.setINIValue(config, "database", "user", req.DBUser)
	config = s.setINIValue(config, "database", "password", req.DBPassword)
	// [security]
	config = s.setINIValue(config, "security", "admin_user", req.AdminUser)
	config = s.setINIValue(config, "security", "admin_password", req.AdminPassword)
	// [users]
	config = s.setINIValue(config, "users", "allow_sign_up", req.AllowSignUp)
	config = s.setINIValue(config, "users", "auto_assign_org_role", req.AutoAssignOrgRole)
	// [smtp]
	config = s.setINIValue(config, "smtp", "enabled", req.SMTPEnabled)
	config = s.setINIValue(config, "smtp", "host", req.SMTPHost)
	config = s.setINIValue(config, "smtp", "user", req.SMTPUser)
	config = s.setINIValue(config, "smtp", "password", req.SMTPPassword)
	config = s.setINIValue(config, "smtp", "from_address", req.SMTPFromAddress)
	// [log]
	config = s.setINIValue(config, "log", "mode", req.LogMode)
	config = s.setINIValue(config, "log", "level", req.LogLevel)

	if err = io.Write(s.configPath(), config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("grafana"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// DataSourceList 获取数据源列表
func (s *App) DataSourceList(w http.ResponseWriter, r *http.Request) {
	service.Success(w, s.getDatasourceList(s.readDatasources()))
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

	if err = s.writeDatasources(cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	if err = s.writeDatasources(cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	if err := s.writeDatasources(cfg); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// configPath 返回 Grafana 主配置文件路径
func (s *App) configPath() string {
	return fmt.Sprintf("%s/server/grafana/conf/defaults.ini", app.Root)
}

// getINIValue 从 INI 配置中获取指定 section 下的 key 值
func (s *App) getINIValue(content string, section string, key string) string {
	currentSection := ""
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			continue
		}
		if currentSection != section {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// setINIValue 在 INI 配置中设置指定 section 下的 key 值
func (s *App) setINIValue(content string, section string, key string, value string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	currentSection := ""
	found := false
	lastSectionLine := -1 // 目标 section 的最后一行索引

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检测 section 头
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			// 如果离开目标 section 且未找到 key，在 section 末尾插入
			if currentSection == section && !found && lastSectionLine >= 0 {
				found = true
				if value != "" {
					// 在 section 末尾插入新行
					insertIdx := lastSectionLine + 1
					newLine := key + " = " + value
					result = append(result[:insertIdx+1], append([]string{newLine}, result[insertIdx+1:]...)...)
				}
			}
			currentSection = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}

		if currentSection == section {
			lastSectionLine = len(result)
		}

		// 在目标 section 内匹配 key
		if currentSection == section && !found {
			checkLine := trimmed
			commented := false
			if strings.HasPrefix(checkLine, ";") {
				checkLine = strings.TrimSpace(checkLine[1:])
				commented = true
			} else if strings.HasPrefix(checkLine, "#") {
				checkLine = strings.TrimSpace(checkLine[1:])
				commented = true
			}
			parts := strings.SplitN(checkLine, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
				found = true
				if value == "" {
					// 值为空时注释掉
					if !commented {
						result = append(result, ";"+line)
					} else {
						result = append(result, line)
					}
				} else {
					result = append(result, key+" = "+value)
				}
				_ = i
				continue
			}
		}

		result = append(result, line)
	}

	// 如果在最后一个 section 中未找到 key
	if currentSection == section && !found {
		found = true
		if value != "" {
			result = append(result, key+" = "+value)
		}
	}

	// section 不存在，在文件末尾追加
	if !found && value != "" {
		result = append(result, "")
		result = append(result, "["+section+"]")
		result = append(result, key+" = "+value)
	}

	return strings.Join(result, "\n")
}

// datasourcePath 返回 provisioning 数据源文件路径
func (s *App) datasourcePath() string {
	return fmt.Sprintf("%s/server/grafana/conf/provisioning/datasources/panel.yml", app.Root)
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
func (s *App) writeDatasources(cfg map[string]any) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err = io.Write(s.datasourcePath(), string(data), 0644); err != nil {
		return err
	}
	return systemctl.Restart("grafana")
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
