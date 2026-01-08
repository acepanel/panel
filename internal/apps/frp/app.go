package frp

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/systemctl"
)

type App struct{}

func NewApp() *App {
	return &App{}
}

func (s *App) Route(r chi.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/user", s.GetUser)
	r.Post("/user", s.UpdateUser)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Name](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := io.Read(fmt.Sprintf("%s/server/frp/%s.toml", app.Root, req.Name))
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

	if err = io.Write(fmt.Sprintf("%s/server/frp/%s.toml", app.Root, req.Name), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(req.Name); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// UserInfo 运行用户信息
type UserInfo struct {
	User  string `json:"user"`
	Group string `json:"group"`
}

// GetUser 获取服务的运行用户
func (s *App) GetUser(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Name](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", req.Name)
	content, err := io.Read(servicePath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	userInfo := UserInfo{
		User:  "",
		Group: "",
	}

	// 解析 User 和 Group
	userRegex := regexp.MustCompile(`(?m)^User=(.*)$`)
	groupRegex := regexp.MustCompile(`(?m)^Group=(.*)$`)

	if matches := userRegex.FindStringSubmatch(content); len(matches) > 1 {
		userInfo.User = matches[1]
	}
	if matches := groupRegex.FindStringSubmatch(content); len(matches) > 1 {
		userInfo.Group = matches[1]
	}

	service.Success(w, userInfo)
}

// UpdateUser 更新服务的运行用户
func (s *App) UpdateUser(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateUser](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", req.Name)
	content, err := io.Read(servicePath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 替换或添加 User 和 Group 配置
	userRegex := regexp.MustCompile(`(?m)^User=.*$`)
	groupRegex := regexp.MustCompile(`(?m)^Group=.*$`)

	if userRegex.MatchString(content) {
		content = userRegex.ReplaceAllString(content, fmt.Sprintf("User=%s", req.User))
	} else {
		// 在 [Service] 后添加 User
		serviceRegex := regexp.MustCompile(`(?m)^\[Service\]$`)
		content = serviceRegex.ReplaceAllString(content, fmt.Sprintf("[Service]\nUser=%s", req.User))
	}

	if groupRegex.MatchString(content) {
		content = groupRegex.ReplaceAllString(content, fmt.Sprintf("Group=%s", req.Group))
	} else {
		// 在 User 后添加 Group
		userLineRegex := regexp.MustCompile(`(?m)^User=.*$`)
		content = userLineRegex.ReplaceAllString(content, fmt.Sprintf("User=%s\nGroup=%s", req.User, req.Group))
	}

	if err = io.Write(servicePath, content, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 重载 systemd 配置
	if err = systemctl.DaemonReload(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 重启服务以应用更改
	if err = systemctl.Restart(req.Name); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
