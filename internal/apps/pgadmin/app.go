package pgadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	stdio "io"
	"net"
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
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t                  *gotext.Locale
	conf               *config.Config
	databaseServerRepo biz.DatabaseServerRepo
}

func NewApp(conf *config.Config, t *gotext.Locale, databaseServerRepo biz.DatabaseServerRepo) *App {
	return &App{
		t:                  t,
		conf:               conf,
		databaseServerRepo: databaseServerRepo,
	}
}

// pgadminLanguage 面板语言映射为 pgAdmin 语言代码
func (s *App) pgadminLanguage() string {
	switch s.conf.App.Locale {
	case "":
		return ""
	case "zh_CN":
		return "zh_Hans_CN"
	case "zh_TW":
		return "zh_Hant_TW"
	default:
		return s.conf.App.Locale
	}
}

// clientIP 获取请求来源 IP,面板位于反代之后时优先取配置的 IP 头
func (s *App) clientIP(r *http.Request) string {
	ip := r.RemoteAddr
	if header := s.conf.HTTP.IPHeader; header != "" && r.Header.Get(header) != "" {
		ip = strings.TrimSpace(strings.Split(r.Header.Get(header), ",")[0])
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return ip
}

func (s *App) Route(r chi.Router) {
	r.Get("/info", s.Info)
	r.Post("/port", s.UpdatePort)
	r.Post("/login", s.Login)
	r.Post("/update_username", s.UpdateUsername)
	r.Post("/reset_password", s.ResetPassword)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "pgadmin")
	return types.AggregateAppStatus(ok)
}

func (s *App) path() string {
	return app.Root + "/server/pgadmin"
}

// port 从 systemd 环境文件中解析监听端口
func (s *App) port() (uint, error) {
	conf, err := io.Read(s.path() + "/pgadmin.conf")
	if err != nil {
		return 0, err
	}
	match := regexp.MustCompile(`PGADMIN_LISTEN=.+:(\d+)`).FindStringSubmatch(conf)
	if len(match) < 2 {
		return 0, errors.New(s.t.Get("pgAdmin port not found"))
	}

	return cast.ToUint(match[1]), nil
}

// credential 读取安装时生成的初始凭据(邮箱与密码)
func (s *App) credential() (string, string) {
	raw, err := io.Read(s.path() + "/credential")
	if err != nil {
		return "", ""
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) < 2 {
		return strings.TrimSpace(lines[0]), ""
	}

	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
}

func (s *App) Info(w http.ResponseWriter, r *http.Request) {
	port, err := s.port()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	email, password := s.credential()

	service.Success(w, chix.M{
		"port":     port,
		"email":    email,
		"password": password,
	})
}

func (s *App) UpdatePort(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdatePort](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	conf := s.path() + "/pgadmin.conf"
	content, err := io.Read(conf)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	content = regexp.MustCompile(`PGADMIN_LISTEN=(.+):\d+`).ReplaceAllString(content, "PGADMIN_LISTEN=${1}:"+cast.ToString(req.Port))
	if err = io.Write(conf, content, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 端口已改写入盘，放行与重启不跟随请求取消，否则新端口不通
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

	if err = systemctl.Restart(ctx, "pgadmin"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to restart pgAdmin: %v", err))
		return
	}

	service.Success(w, nil)
}

// serversFile pgAdmin dump-servers/load-servers 的 JSON 结构
type serversFile struct {
	Servers map[string]serverEntry `json:"Servers"`
}

type serverEntry struct {
	Name          string `json:"Name"`
	Group         string `json:"Group"`
	Host          string `json:"Host"`
	Port          int    `json:"Port"`
	MaintenanceDB string `json:"MaintenanceDB"`
	Username      string `json:"Username"`
	SSLMode       string `json:"SSLMode,omitempty"`
	PassFile      string `json:"PassFile,omitempty"`
}

// escapePgpass 转义 pgpass 字段中的反斜杠与冒号
func escapePgpass(s string) string {
	return strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(s)
}

// existingServers 只读 pgAdmin 配置库查询 Servers 组内已注册的服务器
func (s *App) existingServers(ctx context.Context, email string) (map[string]struct{}, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s/data/pgadmin.db?mode=ro", s.path()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	rows, err := db.QueryContext(ctx, `SELECT s.host, s.port, s.username FROM server s
		JOIN servergroup g ON s.servergroup_id = g.id
		JOIN "user" u ON s.user_id = u.id
		WHERE u.email = ? AND g.name = 'Servers'`, email)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	existing := make(map[string]struct{})
	for rows.Next() {
		var host, username sql.NullString
		var port sql.NullInt64
		if err = rows.Scan(&host, &port, &username); err != nil {
			return nil, err
		}
		existing[fmt.Sprintf("%s:%d:%s", host.String, port.Int64, username.String)] = struct{}{}
	}
	return existing, rows.Err()
}

// dumpExistingServers 通过 CLI 导出查询已注册服务器,直读配置库失败时的回退路径
func (s *App) dumpExistingServers(ctx context.Context, email string) map[string]struct{} {
	dump := filepath.Join(os.TempDir(), "pgadmin-servers.json")
	defer func() { _ = io.Remove(dump) }()
	_, _ = shell.Execf(ctx, "%s/cli dump-servers '%s' --user '%s'", s.path(), dump, email)

	existing := make(map[string]struct{})
	if raw, err := io.Read(dump); err == nil {
		var dumped serversFile
		if err = json.Unmarshal([]byte(raw), &dumped); err == nil {
			for _, item := range dumped.Servers {
				// 只认默认 Servers 组内的条目,历史版本同步进其他分组的会在此组补齐
				if item.Group != "Servers" {
					continue
				}
				existing[fmt.Sprintf("%s:%d:%s", item.Host, item.Port, item.Username)] = struct{}{}
			}
		}
	}
	return existing
}

// syncServers 将面板中全部 PostgreSQL 服务器合并注册到 pgAdmin,凭据写入 pgpass 实现免密
// 仅追加 pgAdmin 中缺失的服务器,不影响用户在 pgAdmin 中手动添加的内容
func (s *App) syncServers(ctx context.Context, email string) error {
	servers, _, err := s.databaseServerRepo.List(ctx, 1, 10000, string(biz.DatabaseTypePostgresql))
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}

	// pgAdmin server 模式下 PassFile 以用户 storage 目录为根,目录名为邮箱 @ 转 _
	storageDir := fmt.Sprintf("%s/data/storage/%s", s.path(), strings.ReplaceAll(email, "@", "_"))
	pgpass := filepath.Join(storageDir, "pgpass")

	// 重写 pgpass 中面板服务器的凭据行,保留其他行
	prefixes := make([]string, 0, len(servers))
	entries := make([]string, 0, len(servers))
	for _, server := range servers {
		prefix := fmt.Sprintf("%s:%d:*:%s:", escapePgpass(server.Host), server.Port, escapePgpass(server.Username))
		prefixes = append(prefixes, prefix)
		entries = append(entries, prefix+escapePgpass(server.Password))
	}
	var lines []string
	if raw, err := io.Read(pgpass); err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(raw), "\n") {
			if line == "" {
				continue
			}
			panelOwned := false
			for _, prefix := range prefixes {
				if strings.HasPrefix(line, prefix) {
					panelOwned = true
					break
				}
			}
			if !panelOwned {
				lines = append(lines, line)
			}
		}
	}
	lines = append(lines, entries...)
	if err = os.MkdirAll(storageDir, 0700); err != nil {
		return err
	}
	if err = io.Write(pgpass, strings.Join(lines, "\n")+"\n", 0600); err != nil {
		return err
	}

	// 查询 pgAdmin 已有服务器用于查缺,直读配置库,异常时回退 CLI 导出
	existing, err := s.existingServers(ctx, email)
	if err != nil {
		existing = s.dumpExistingServers(ctx, email)
	}

	// 一次性合并导入缺失的服务器
	missing := make(map[string]serverEntry)
	for i, server := range servers {
		if _, ok := existing[fmt.Sprintf("%s:%d:%s", server.Host, server.Port, server.Username)]; ok {
			continue
		}
		missing[cast.ToString(i+1)] = serverEntry{
			Name:          server.Name,
			Group:         "Servers",
			Host:          server.Host,
			Port:          int(server.Port),
			MaintenanceDB: "postgres",
			Username:      server.Username,
			SSLMode:       "prefer",
			PassFile:      "/pgpass",
		}
	}
	if len(missing) > 0 {
		load := filepath.Join(os.TempDir(), "pgadmin-servers-add.json")
		defer func() { _ = io.Remove(load) }()
		payload, err := json.Marshal(serversFile{Servers: missing})
		if err != nil {
			return err
		}
		if err = io.Write(load, string(payload), 0600); err != nil {
			return err
		}
		// 导入与随后的 chown 必须成对完成，否则数据目录属主错误导致服务写不进去
		loadCtx := context.WithoutCancel(ctx)
		if out, err := shell.Execf(loadCtx, "%s/cli load-servers '%s' --user '%s'", s.path(), load, email); err != nil {
			return errors.Join(err, errors.New(out))
		}
		// CLI 以 root 运行,修正数据目录属主避免服务写入失败
		if _, err = shell.Execf(loadCtx, "chown -R www:www %s/data", s.path()); err != nil {
			return err
		}
		return nil
	}

	return io.ChownR(ctx, storageDir, "www", "www")
}

// Login 同步面板全部 PostgreSQL 服务器后代理登录 pgAdmin 并将会话 Cookie 转发给浏览器
// 面板与 pgAdmin 同主机不同端口,Cookie 按主机共享,浏览器凭转发的 Cookie 即为已登录态
func (s *App) Login(w http.ResponseWriter, r *http.Request) {
	port, err := s.port()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	email, password := s.credential()
	if email == "" || password == "" {
		service.Error(w, http.StatusInternalServerError, s.t.Get("pgAdmin credential file not found"))
		return
	}

	if err = s.syncServers(r.Context(), email); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to sync servers to pgAdmin: %v", err))
		return
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 透传浏览器 IP 与 UA,pgAdmin 增强 Cookie 保护将会话绑定到 sha256(IP|UA),
	// 伪装成浏览器身份登录后浏览器直连即可通过校验,无需关闭该保护
	clientIP := s.clientIP(r)
	clientUA := r.UserAgent()

	// 获取登录页以取得会话 Cookie 与 CSRF token
	loginURL := fmt.Sprintf("http://127.0.0.1:%d/login", port)
	pageReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, loginURL, nil)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	pageReq.Header.Set("User-Agent", clientUA)
	pageReq.Header.Set("X-Forwarded-For", clientIP)
	pageResp, err := client.Do(pageReq)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to request pgAdmin: %v", err))
		return
	}
	defer func() { _ = pageResp.Body.Close() }()
	page, err := stdio.ReadAll(stdio.LimitReader(pageResp.Body, 4<<20))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to request pgAdmin: %v", err))
		return
	}

	// 登录页为 React 渲染,CSRF token 在内嵌 JSON 中,保留 input 形态兼容旧版本
	csrf := regexp.MustCompile(`"csrfToken":\s*"([^"]+)"`).FindStringSubmatch(string(page))
	if len(csrf) < 2 {
		csrf = regexp.MustCompile(`name="csrf_token"[^>]*value="([^"]+)"`).FindStringSubmatch(string(page))
	}
	if len(csrf) < 2 {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to parse pgAdmin login page"))
		return
	}

	form := url.Values{}
	form.Set("csrf_token", csrf[1])
	form.Set("email", email)
	form.Set("password", password)
	// 语言跟随面板设置,pgAdmin 会将 language 字段固化进会话
	if lang := s.pgadminLanguage(); lang != "" {
		form.Set("language", lang)
	}

	loginReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, fmt.Sprintf("http://127.0.0.1:%d/authenticate/login", port), strings.NewReader(form.Encode()))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("User-Agent", clientUA)
	loginReq.Header.Set("X-Forwarded-For", clientIP)
	for _, cookie := range pageResp.Cookies() {
		loginReq.AddCookie(cookie)
	}

	loginResp, err := client.Do(loginReq)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to request pgAdmin: %v", err))
		return
	}
	defer func() { _ = loginResp.Body.Close() }()
	_, _ = stdio.Copy(stdio.Discard, stdio.LimitReader(loginResp.Body, 4<<20))

	// 登录成功时 pgAdmin 返回 302 且跳转目标不是登录页
	location := loginResp.Header.Get("Location")
	if loginResp.StatusCode != http.StatusFound || strings.Contains(location, "/login") {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to login pgAdmin, please check the credential of pgAdmin"))
		return
	}

	// 改写 SameSite 以兼容面板 https 跳转 http 的场景
	for _, cookie := range append(pageResp.Cookies(), loginResp.Cookies()...) {
		cookie.SameSite = http.SameSiteLaxMode
		cookie.Secure = false
		http.SetCookie(w, cookie)
	}

	service.Success(w, chix.M{
		"port": port,
	})
}

// ResetPassword 通过 CLI 重置管理员密码并同步凭据文件
// UpdateUsername 修改管理员账号,迁移服务器配置与用户存储目录
func (s *App) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateUsername](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldEmail, password := s.credential()
	if oldEmail == "" || password == "" {
		service.Error(w, http.StatusInternalServerError, s.t.Get("pgAdmin credential file not found"))
		return
	}
	if req.Username == oldEmail {
		service.Success(w, nil)
		return
	}

	// 账号迁移中途取消会留下两个账号或凭据文件指向已删账号，整段不跟随请求取消
	ctx := context.WithoutCancel(r.Context())

	// 以当前密码创建新管理员账号
	if out, err := shell.Execf(ctx, "%s/cli add-user '%s' '%s' --admin", s.path(), req.Username, password); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to create new account: %v", errors.Join(err, errors.New(out))))
		return
	}

	// 迁移服务器连接配置到新账号
	dump := filepath.Join(os.TempDir(), "pgadmin-servers-migrate.json")
	defer func() { _ = io.Remove(dump) }()
	_, _ = shell.Execf(ctx, "%s/cli dump-servers '%s' --user '%s'", s.path(), dump, oldEmail)
	if io.Exists(dump) {
		_, _ = shell.Execf(ctx, "%s/cli load-servers '%s' --user '%s'", s.path(), dump, req.Username)
	}

	// 删除旧账号
	if out, err := shell.Execf(ctx, "%s/cli delete-user '%s' --yes", s.path(), oldEmail); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to delete old account: %v", errors.Join(err, errors.New(out))))
		return
	}

	// 迁移用户存储目录(pgpass 等),目录名为邮箱 @ 转 _
	oldDir := fmt.Sprintf("%s/data/storage/%s", s.path(), strings.ReplaceAll(oldEmail, "@", "_"))
	newDir := fmt.Sprintf("%s/data/storage/%s", s.path(), strings.ReplaceAll(req.Username, "@", "_"))
	if io.Exists(oldDir) && !io.Exists(newDir) {
		_ = os.Rename(oldDir, newDir)
	}

	// CLI 以 root 运行,修正数据目录属主避免服务写入失败
	if _, err = shell.Execf(ctx, "chown -R www:www %s/data", s.path()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新凭据文件,下次登录按新账号重新同步
	if err = io.Write(s.path()+"/credential", req.Username+"\n"+password+"\n", 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) ResetPassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ResetPassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	email, _ := s.credential()
	if email == "" {
		service.Error(w, http.StatusInternalServerError, s.t.Get("pgAdmin credential file not found"))
		return
	}

	// 改密后还要 chown 与回写凭据文件，中途取消会让面板存的密码与实际不符
	ctx := context.WithoutCancel(r.Context())

	// cli 为安装脚本生成的稳定入口,屏蔽上游命令名随大版本变化
	if out, err := shell.Execf(ctx, "%s/cli update-user '%s' --password '%s'", s.path(), email, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reset password: %v", errors.Join(err, errors.New(out))))
		return
	}
	// CLI 以 root 运行,修正数据目录属主避免服务写入失败
	if _, err = shell.Execf(ctx, "chown -R www:www %s/data", s.path()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(s.path()+"/credential", email+"\n"+req.Password+"\n", 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
