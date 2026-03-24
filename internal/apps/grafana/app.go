package grafana

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
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

	// 从配置中获取端口
	confPath := fmt.Sprintf("%s/server/grafana/conf/grafana.ini", app.Root)
	config, _ := io.Read(confPath)
	port := s.getINIValue(config, "server", "http_port")
	if port == "" {
		port = "3000"
	}

	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	resp, err := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/api/health", port))
	if err != nil || !resp.IsSuccess() {
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
	confPath := fmt.Sprintf("%s/server/grafana/conf/grafana.ini", app.Root)
	config, _ := io.Read(confPath)

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/grafana/conf/grafana.ini", app.Root), req.Config, 0644); err != nil {
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
	config, _ := io.Read(fmt.Sprintf("%s/server/grafana/conf/grafana.ini", app.Root))

	tune := ConfigTune{
		// [server]
		HTTPPort: s.getINIValue(config, "server", "http_port"),
		Domain:   s.getINIValue(config, "server", "domain"),
		RootURL:  s.getINIValue(config, "server", "root_url"),
		Protocol: s.getINIValue(config, "server", "protocol"),
		// [database]
		DBType:     s.getINIValue(config, "database", "type"),
		DBHost:     s.getINIValue(config, "database", "host"),
		DBName:     s.getINIValue(config, "database", "name"),
		DBUser:     s.getINIValue(config, "database", "user"),
		DBPassword: s.getINIValue(config, "database", "password"),
		// [security]
		AdminUser:     s.getINIValue(config, "security", "admin_user"),
		AdminPassword: s.getINIValue(config, "security", "admin_password"),
		// [users]
		AllowSignUp:       s.getINIValue(config, "users", "allow_sign_up"),
		AutoAssignOrgRole: s.getINIValue(config, "users", "auto_assign_org_role"),
		// [smtp]
		SMTPEnabled:     s.getINIValue(config, "smtp", "enabled"),
		SMTPHost:        s.getINIValue(config, "smtp", "host"),
		SMTPUser:        s.getINIValue(config, "smtp", "user"),
		SMTPPassword:    s.getINIValue(config, "smtp", "password"),
		SMTPFromAddress: s.getINIValue(config, "smtp", "from_address"),
		// [log]
		LogMode:  s.getINIValue(config, "log", "mode"),
		LogLevel: s.getINIValue(config, "log", "level"),
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

	confPath := fmt.Sprintf("%s/server/grafana/conf/grafana.ini", app.Root)
	config, _ := io.Read(confPath)

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

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("grafana"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
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
