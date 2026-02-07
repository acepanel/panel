package pureftpd

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/firewall"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/users", s.List)
	r.Post("/users", s.Create)
	r.Delete("/users/{username}", s.Delete)
	r.Post("/users/{username}/password", s.ChangePassword)
	r.Get("/port", s.GetPort)
	r.Post("/port", s.UpdatePort)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

// List 获取用户列表
func (s *App) List(w http.ResponseWriter, r *http.Request) {
	listRaw, err := shell.Execf("pure-pw list")
	if err != nil {
		service.Success(w, chix.M{
			"total": 0,
			"items": []User{},
		})
	}

	listArr := strings.Split(listRaw, "\n")
	var users []User
	for _, v := range listArr {
		if len(v) == 0 {
			continue
		}

		match := regexp.MustCompile(`(\S+)\s+(\S+)`).FindStringSubmatch(v)
		users = append(users, User{
			Username: match[1],
			Path:     strings.Replace(match[2], "/./", "/", 1),
		})
	}

	paged, total := service.Paginate(r, users)

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// Create 创建用户
func (s *App) Create(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Create](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + req.Path
	}
	if !io.Exists(req.Path) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("directory %s does not exist", req.Path))
		return
	}

	if _, err = shell.Execf(`yes '%s' | pure-pw useradd '%s' -u www -g www -d '%s'`, req.Password, req.Username, req.Path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf("pure-pw mkdb"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// Delete 删除用户
func (s *App) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Delete](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf("pure-pw userdel '%s' -m", req.Username); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf("pure-pw mkdb"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// ChangePassword 修改密码
func (s *App) ChangePassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ChangePassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = shell.Execf(`yes '%s' | pure-pw passwd '%s' -m`, req.Password, req.Username); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf("pure-pw mkdb"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetPort 获取端口
func (s *App) GetPort(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/pure-ftpd/etc/pure-ftpd.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get port: %v", err))
		return
	}

	bind := strings.Trim(s.getFTPValue(config, "Bind"), `"'`)
	port := 21 // 默认端口
	if parts := strings.SplitN(bind, ",", 2); len(parts) == 2 {
		port = cast.ToInt(strings.TrimSpace(parts[1]))
	}

	service.Success(w, port)
}

// UpdatePort 设置端口
func (s *App) UpdatePort(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdatePort](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/pure-ftpd/etc/pure-ftpd.conf", app.Root)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	config = s.setFTPValue(config, "Bind", fmt.Sprintf(`"0.0.0.0,%d"`, req.Port))
	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	fw := firewall.NewFirewall()
	err = fw.Port(firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: req.Port,
		PortEnd:   req.Port,
		Direction: firewall.DirectionIn,
		Strategy:  firewall.StrategyAccept,
	}, firewall.OperationAdd)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("pure-ftpd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetConfigTune 获取 Pure-FTPd 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/pure-ftpd/etc/pure-ftpd.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		MaxClientsNumber: s.getFTPValue(config, "MaxClientsNumber"),
		MaxClientsPerIP:  s.getFTPValue(config, "MaxClientsPerIP"),
		MaxIdleTime:      s.getFTPValue(config, "MaxIdleTime"),
		MaxLoad:          s.getFTPValue(config, "MaxLoad"),
		PassivePortRange: s.getFTPValue(config, "PassivePortRange"),
		AnonymousOnly:    s.getFTPValue(config, "AnonymousOnly"),
		NoAnonymous:      s.getFTPValue(config, "NoAnonymous"),
		MaxDiskUsage:     s.getFTPValue(config, "MaxDiskUsage"),
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Pure-FTPd 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/pure-ftpd/etc/pure-ftpd.conf", app.Root)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = s.setFTPValue(config, "MaxClientsNumber", req.MaxClientsNumber)
	config = s.setFTPValue(config, "MaxClientsPerIP", req.MaxClientsPerIP)
	config = s.setFTPValue(config, "MaxIdleTime", req.MaxIdleTime)
	config = s.setFTPValue(config, "MaxLoad", req.MaxLoad)
	config = s.setFTPValue(config, "PassivePortRange", req.PassivePortRange)
	config = s.setFTPValue(config, "AnonymousOnly", req.AnonymousOnly)
	config = s.setFTPValue(config, "NoAnonymous", req.NoAnonymous)
	config = s.setFTPValue(config, "MaxDiskUsage", req.MaxDiskUsage)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("pure-ftpd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// getFTPValue 从 Pure-FTPd 配置内容中获取指定键的值
func (s *App) getFTPValue(content string, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) >= 2 && parts[0] == key {
			return strings.Join(parts[1:], " ")
		}
	}
	return ""
}

// setFTPValue 在 Pure-FTPd 配置内容中设置指定键的值
func (s *App) setFTPValue(content string, key string, value string) string {
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
		parts := strings.Fields(checkLine)
		if len(parts) >= 2 && parts[0] == key {
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
			// 保留原行格式
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			result = append(result, indent+key+" "+value)
		} else {
			result = append(result, line)
		}
	}
	if !found && value != "" {
		result = append(result, key+" "+value)
	}
	return strings.Join(result, "\n")
}
