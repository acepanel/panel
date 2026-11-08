package frp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/apps/common"
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
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/user", s.GetUser)
	r.Post("/user", s.UpdateUser)
	r.Get("/server", s.GetServer)
	r.Post("/server", s.UpdateServer)
	r.Get("/client", s.GetClient)
	r.Post("/client", s.UpdateClient)
	r.Get("/proxies", s.Proxies)
	r.Post("/proxies", s.CreateProxy)
	r.Get("/proxies/{name}", s.GetProxy)
	r.Post("/proxies/{name}", s.UpdateProxy)
	r.Delete("/proxies/{name}", s.DeleteProxy)
	r.Get("/visitors", s.Visitors)
	r.Post("/visitors", s.CreateVisitor)
	r.Get("/visitors/{name}", s.GetVisitor)
	r.Post("/visitors/{name}", s.UpdateVisitor)
	r.Delete("/visitors/{name}", s.DeleteVisitor)
}

func (s *App) Status(ctx context.Context) string {
	frps, _ := systemctl.Status(ctx, "frps")
	frpc, _ := systemctl.Status(ctx, "frpc")
	return types.AggregateAppStatus(frps, frpc)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Name](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := confPath(req.Name)
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", req.Name), slog.String("path", path))

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	common.WriteConfig(w, r, s.log, s.t, "frp", confPath(req.Name), req.Config, 0644, req.Name)
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

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", req.Name), slog.String("path", servicePath))

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
	switch {
	case hasUser && hasGroup:
		// 两者都存在，分别替换
		content = userRegex.ReplaceAllString(content, "User="+req.User)
		content = groupRegex.ReplaceAllString(content, "Group="+req.Group)
	case hasUser:
		// 只有 User，替换 User 并添加 Group
		content = userRegex.ReplaceAllString(content, fmt.Sprintf("User=%s\nGroup=%s", req.User, req.Group))
	case hasGroup:
		// 只有 Group，添加 User 并替换 Group
		content = serviceRegex.ReplaceAllString(content, "[Service]\nUser="+req.User)
		content = groupRegex.ReplaceAllString(content, "Group="+req.Group)
	default:
		// 两者都不存在，在 [Service] 后添加两者
		content = serviceRegex.ReplaceAllString(content, fmt.Sprintf("[Service]\nUser=%s\nGroup=%s", req.User, req.Group))
	}

	if err = io.Write(servicePath, content, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.DaemonReload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = systemctl.Restart(r.Context(), req.Name); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("frp service user updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", req.Name), slog.String("user", req.User), slog.String("group", req.Group))

	service.Success(w, nil)
}

// GetServer 获取 frps 可视化参数
func (s *App) GetServer(w http.ResponseWriter, r *http.Request) {
	path := confPath("frps")
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := new(ServerTune)
	readTune(config, serverFields(tune))

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", "frps"), slog.String("path", path))

	service.Success(w, tune)
}

// UpdateServer 更新 frps 可视化参数
func (s *App) UpdateServer(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ServerTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := confPath("frps")
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(path, writeTune(config, serverFields(req)), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "frps"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", "frps"), slog.String("path", path), slog.String("bind_port", req.BindPort), slog.String("vhost_http_port", req.VhostHTTPPort), slog.String("vhost_https_port", req.VhostHTTPSPort))

	service.Success(w, nil)
}

// GetClient 获取 frpc 可视化公共参数
func (s *App) GetClient(w http.ResponseWriter, r *http.Request) {
	path := confPath("frpc")
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := new(ClientTune)
	readTune(config, clientFields(tune))

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", "frpc"), slog.String("path", path))

	service.Success(w, tune)
}

// UpdateClient 更新 frpc 可视化公共参数
func (s *App) UpdateClient(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ClientTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := confPath("frpc")
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(path, writeTune(config, clientFields(req)), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "frpc"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("service", "frpc"), slog.String("path", path), slog.String("server_addr", req.ServerAddr), slog.String("server_port", req.ServerPort))

	service.Success(w, nil)
}

// Proxies 代理列表
func (s *App) Proxies(w http.ResponseWriter, r *http.Request) {
	confD, err := listConfD()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := service.Paginate(r, confD.Proxies)

	s.log.Info(s.t.Get("frp proxy list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"))

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// CreateProxy 新增代理
func (s *App) CreateProxy(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Proxy](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if io.Exists(itemPath(proxyPrefix, req.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("proxy %s already exists", req.Name))
		return
	}

	if err = s.save(r.Context(), proxyPrefix, req.Name, ConfD{Proxies: []Proxy{*req}}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("frp proxy created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("proxy_type", req.Type), slog.String("local_ip", req.LocalIP), slog.Int("local_port", req.LocalPort), slog.Int("remote_port", req.RemotePort), slog.Any("custom_domains", req.CustomDomains), slog.String("subdomain", req.Subdomain))

	service.Success(w, nil)
}

// GetProxy 获取单个代理
func (s *App) GetProxy(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ItemName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := itemPath(proxyPrefix, req.Name)
	confD, err := readConfD(path)
	if err != nil || len(confD.Proxies) == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("proxy %s does not exist", req.Name))
		return
	}

	s.log.Info(s.t.Get("frp proxy viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("path", path))

	service.Success(w, confD.Proxies[0])
}

// UpdateProxy 更新代理
func (s *App) UpdateProxy(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Proxy](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !io.Exists(itemPath(proxyPrefix, req.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("proxy %s does not exist", req.Name))
		return
	}

	if err = s.save(r.Context(), proxyPrefix, req.Name, ConfD{Proxies: []Proxy{*req}}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("frp proxy updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("proxy_type", req.Type), slog.String("local_ip", req.LocalIP), slog.Int("local_port", req.LocalPort), slog.Int("remote_port", req.RemotePort), slog.Any("custom_domains", req.CustomDomains), slog.Bool("enabled", lo.FromPtrOr(req.Enabled, true)))

	service.Success(w, nil)
}

// DeleteProxy 删除代理
func (s *App) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ItemName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 配置已删除，重启不跟随请求取消，否则代理仍在运行的 frpc 里生效
	ctx := context.WithoutCancel(r.Context())
	path := itemPath(proxyPrefix, req.Name)
	if err = io.Remove(path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(ctx, "frpc"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("frp proxy deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("path", path))

	service.Success(w, nil)
}

// Visitors 访问者列表
func (s *App) Visitors(w http.ResponseWriter, r *http.Request) {
	confD, err := listConfD()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := service.Paginate(r, confD.Visitors)

	s.log.Info(s.t.Get("frp visitor list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"))

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// CreateVisitor 新增访问者
func (s *App) CreateVisitor(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Visitor](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if io.Exists(itemPath(visitorPrefix, req.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("visitor %s already exists", req.Name))
		return
	}

	if err = s.save(r.Context(), visitorPrefix, req.Name, ConfD{Visitors: []Visitor{*req}}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("frp visitor created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("visitor_type", req.Type), slog.String("server_name", req.ServerName), slog.String("bind_addr", req.BindAddr), slog.Int("bind_port", req.BindPort))

	service.Success(w, nil)
}

// GetVisitor 获取单个访问者
func (s *App) GetVisitor(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ItemName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := itemPath(visitorPrefix, req.Name)
	confD, err := readConfD(path)
	if err != nil || len(confD.Visitors) == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("visitor %s does not exist", req.Name))
		return
	}

	s.log.Info(s.t.Get("frp visitor viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("path", path))

	service.Success(w, confD.Visitors[0])
}

// UpdateVisitor 更新访问者
func (s *App) UpdateVisitor(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Visitor](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if !io.Exists(itemPath(visitorPrefix, req.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("visitor %s does not exist", req.Name))
		return
	}

	if err = s.save(r.Context(), visitorPrefix, req.Name, ConfD{Visitors: []Visitor{*req}}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("frp visitor updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("visitor_type", req.Type), slog.String("server_name", req.ServerName), slog.String("bind_addr", req.BindAddr), slog.Int("bind_port", req.BindPort), slog.Bool("enabled", lo.FromPtrOr(req.Enabled, true)))

	service.Success(w, nil)
}

// DeleteVisitor 删除访问者
func (s *App) DeleteVisitor(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ItemName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	ctx := context.WithoutCancel(r.Context())
	path := itemPath(visitorPrefix, req.Name)
	if err = io.Remove(path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(ctx, "frpc"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("frp visitor deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "frp"), slog.String("name", req.Name), slog.String("path", path))

	service.Success(w, nil)
}

func (s *App) save(ctx context.Context, prefix, name string, confD ConfD) error {
	if err := writeConfD(prefix, name, confD); err != nil {
		return err
	}

	return systemctl.Restart(ctx, "frpc")
}
