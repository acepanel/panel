package frp

import (
	"fmt"
	"net/http"

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

	userInfo := UserInfo{}

	// 解析 User 和 Group
	if matches := userCaptureRegex.FindStringSubmatch(content); len(matches) > 1 {
		userInfo.User = matches[1]
	}
	if matches := groupCaptureRegex.FindStringSubmatch(content); len(matches) > 1 {
		userInfo.Group = matches[1]
	}

	service.Success(w, userInfo)
}

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

	// 检查 User 和 Group 是否存在
	hasUser := userRegex.MatchString(content)
	hasGroup := groupRegex.MatchString(content)

	// 替换或添加 User 和 Group 配置
	if hasUser && hasGroup {
		// 两者都存在，分别替换
		content = userRegex.ReplaceAllString(content, fmt.Sprintf("User=%s", req.User))
		content = groupRegex.ReplaceAllString(content, fmt.Sprintf("Group=%s", req.Group))
	} else if hasUser && !hasGroup {
		// 只有 User，替换 User 并添加 Group
		content = userRegex.ReplaceAllString(content, fmt.Sprintf("User=%s\nGroup=%s", req.User, req.Group))
	} else if !hasUser && hasGroup {
		// 只有 Group，添加 User 并替换 Group
		content = serviceRegex.ReplaceAllString(content, fmt.Sprintf("[Service]\nUser=%s", req.User))
		content = groupRegex.ReplaceAllString(content, fmt.Sprintf("Group=%s", req.Group))
	} else {
		// 两者都不存在，在 [Service] 后添加两者
		content = serviceRegex.ReplaceAllString(content, fmt.Sprintf("[Service]\nUser=%s\nGroup=%s", req.User, req.Group))
	}

	if err = io.Write(servicePath, content, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.DaemonReload(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = systemctl.Restart(req.Name); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
