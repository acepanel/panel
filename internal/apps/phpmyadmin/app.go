package phpmyadmin

import (
	"context"
	"errors"
	"fmt"
	"html"
	stdio "io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/libtnb/utils/str"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

type App struct {
	t                  *gotext.Locale
	conf               *config.Config
	databaseServerRepo biz.DatabaseServerRepo
	settingRepo        biz.SettingRepo
}

func NewApp(conf *config.Config, t *gotext.Locale, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo) *App {
	return &App{
		t:                  t,
		conf:               conf,
		databaseServerRepo: databaseServerRepo,
		settingRepo:        settingRepo,
	}
}

// dialect 取当前 Web 服务器方言
func (s *App) dialect() (webserver.Dialect, error) {
	webServer, err := s.settingRepo.Get(biz.SettingKeyWebserver)
	if err != nil {
		return webserver.Dialect{}, err
	}

	return webserver.Get(webserver.Type(webServer))
}

func (s *App) Route(r chi.Router) {
	r.Get("/info", s.Info)
	r.Post("/port", s.UpdatePort)
	r.Post("/login", s.Login)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

// Status phpMyAdmin 由 Web 服务器站点承载，运行状态与 Web 服务器一致
func (s *App) Status(ctx context.Context) string {
	d, err := s.dialect()
	if err != nil {
		return types.AggregateAppStatus(false)
	}
	ok, _ := systemctl.Status(ctx, d.Service())
	return types.AggregateAppStatus(ok)
}

// info 获取 phpMyAdmin 的访问目录与端口
func (s *App) info() (string, int, error) {
	files, err := os.ReadDir(app.Root + "/server/phpmyadmin")
	if err != nil {
		return "", 0, errors.New(s.t.Get("phpMyAdmin directory not found"))
	}

	var phpmyadmin string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "phpmyadmin_") {
			phpmyadmin = f.Name()
		}
	}
	if len(phpmyadmin) == 0 {
		return "", 0, errors.New(s.t.Get("phpMyAdmin directory not found"))
	}

	d, err := s.dialect()
	if err != nil {
		return "", 0, err
	}
	vhost, err := d.NewPHPVhost(app.Root + "/sites/phpmyadmin/config")
	if err != nil {
		return "", 0, err
	}
	listens := vhost.Listen()
	if len(listens) == 0 {
		return "", 0, errors.New(s.t.Get("phpMyAdmin port not found"))
	}

	return phpmyadmin, cast.ToInt(listens[0].Address[strings.LastIndex(listens[0].Address, ":")+1:]), nil
}

func (s *App) Info(w http.ResponseWriter, r *http.Request) {
	path, port, err := s.info()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, chix.M{
		"path": path,
		"port": port,
	})
}

// ensureConfig 为存量安装补写 config.inc.php,允许登录到任意 MySQL 服务器
func (s *App) ensureConfig(path string) error {
	config := fmt.Sprintf("%s/server/phpmyadmin/%s/config.inc.php", app.Root, path)
	if io.Exists(config) {
		return nil
	}

	content := fmt.Sprintf(`<?php
declare(strict_types=1);
$cfg['blowfish_secret'] = '%s';
$cfg['AllowArbitraryServer'] = true;
`, str.Random(32))
	return io.Write(config, content, 0644)
}

// Login 代理登录 phpMyAdmin 并将会话 Cookie 转发给浏览器
// 面板与 phpMyAdmin 同主机不同端口,Cookie 按主机共享,浏览器凭转发的 Cookie 即为已登录态
func (s *App) Login(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Login](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	server, err := s.databaseServerRepo.Get(r.Context(), req.ServerID)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if server.Type != biz.DatabaseTypeMysql {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("server %s is not a MySQL server", server.Name))
		return
	}

	path, port, err := s.info()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.ensureConfig(path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	loginURL := fmt.Sprintf("http://127.0.0.1:%d/%s/index.php?route=/", port, path)
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 获取登录页以取得会话 Cookie 与 CSRF token
	// 转发浏览器语言,避免语言协商落空后回落英文
	pageReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, loginURL, nil)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	pageReq.Header.Set("Accept-Language", r.Header.Get("Accept-Language"))
	pageResp, err := client.Do(pageReq)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to request phpMyAdmin: %v", err))
		return
	}
	defer func() { _ = pageResp.Body.Close() }()
	page, err := stdio.ReadAll(stdio.LimitReader(pageResp.Body, 4<<20))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to request phpMyAdmin: %v", err))
		return
	}

	token := regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(string(page))
	session := regexp.MustCompile(`name="set_session" value="([^"]+)"`).FindStringSubmatch(string(page))
	if len(token) < 2 {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to parse phpMyAdmin login page"))
		return
	}

	form := url.Values{}
	form.Set("route", "/")
	// 语言跟随面板设置,登录后固化进 phpMyAdmin 会话
	if s.conf.App.Locale != "" {
		form.Set("lang", s.conf.App.Locale)
	}
	form.Set("token", html.UnescapeString(token[1]))
	if len(session) >= 2 {
		form.Set("set_session", html.UnescapeString(session[1]))
	}
	form.Set("pma_username", server.Username)
	form.Set("pma_password", server.Password)
	form.Set("server", "1")
	// 本地默认端口走 phpMyAdmin 默认配置(socket 连接),其余场景显式指定目标服务器
	// host 为 localhost 但端口非默认时须用 127.0.0.1 强制走 TCP,否则 mysqli 会忽略端口走 socket
	isLocal := server.Host == "localhost" || server.Host == "127.0.0.1"
	if !isLocal || server.Port != 3306 {
		host := server.Host
		if host == "localhost" {
			host = "127.0.0.1"
		}
		form.Set("pma_servername", fmt.Sprintf("%s %d", host, server.Port))
	}

	loginReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("Accept-Language", r.Header.Get("Accept-Language"))
	for _, cookie := range pageResp.Cookies() {
		loginReq.AddCookie(cookie)
	}

	loginResp, err := client.Do(loginReq)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to request phpMyAdmin: %v", err))
		return
	}
	defer func() { _ = loginResp.Body.Close() }()
	_, _ = stdio.Copy(stdio.Discard, stdio.LimitReader(loginResp.Body, 4<<20))

	// 登录成功时 phpMyAdmin 返回 302 并携带会话 Cookie
	if loginResp.StatusCode != http.StatusFound {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to login phpMyAdmin, please check the credentials and status of server %s", server.Name))
		return
	}

	// 改写 SameSite 以兼容面板 https 跳转 http 的场景
	for _, cookie := range append(pageResp.Cookies(), loginResp.Cookies()...) {
		cookie.SameSite = http.SameSiteLaxMode
		cookie.Secure = false
		http.SetCookie(w, cookie)
	}

	service.Success(w, chix.M{
		"path": path,
		"port": port,
	})
}

func (s *App) UpdatePort(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdatePort](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	d, err := s.dialect()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	vhost, err := d.NewPHPVhost(app.Root + "/sites/phpmyadmin/config")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = vhost.SetListen([]webservertypes.Listen{{Address: cast.ToString(req.Port), Args: []string{}}}); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = vhost.Save(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 站点配置已落盘，放行与重载不跟随请求取消，否则新端口不通
	ctx := context.WithoutCancel(r.Context())
	fw := firewall.NewFirewall()
	running, err := fw.Status(ctx)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if running {
		if err = fw.Port(ctx, firewall.FireInfo{
			Type:      firewall.TypeNormal,
			PortStart: req.Port,
			PortEnd:   req.Port,
			Strategy:  firewall.StrategyAccept,
			Direction: firewall.DirectionIn,
		}, firewall.OperationAdd); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = d.Reload(ctx); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	d, err := s.dialect()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	config, err := io.Read(filepath.Join(app.Root, "sites/phpmyadmin/config", d.ConfigFile()))
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

	d, err := s.dialect()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Write(filepath.Join(app.Root, "sites/phpmyadmin/config", d.ConfigFile()), req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = d.Reload(context.WithoutCancel(r.Context())); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
