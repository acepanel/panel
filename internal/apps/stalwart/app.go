// Package stalwart 把 Stalwart 邮件服务器接成面板子应用：安装脚本只放二进制与 systemd 单元，
// 初始化与日常管理全部通过它的 JMAP 管理接口完成
package stalwart

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

const (
	slug = "stalwart"
	unit = "stalwart"
	// 管理接口与 WebUI 的明文 HTTP 端口，初始化后只绑定 127.0.0.1，由面板 nginx 反代对外
	httpPort = 8080
	// 安装脚本写进 env 文件的引导凭据变量名，初始化完成后由面板删除
	recoveryEnv = "STALWART_RECOVERY_ADMIN"
)

type App struct {
	t           *gotext.Locale
	log         *slog.Logger
	settingRepo biz.SettingRepo
	certRepo    biz.CertRepo
	certDNSRepo biz.CertDNSRepo
	website     *biz.WebsiteUsecase
	cert        *biz.CertUsecase
}

func NewApp(t *gotext.Locale, log *slog.Logger, settingRepo biz.SettingRepo, certRepo biz.CertRepo, certDNSRepo biz.CertDNSRepo, website *biz.WebsiteUsecase, cert *biz.CertUsecase) *App {
	s := &App{
		t:           t,
		log:         log,
		settingRepo: settingRepo,
		certRepo:    certRepo,
		certDNSRepo: certDNSRepo,
		website:     website,
		cert:        cert,
	}
	biz.RegisterCertHook(s.onCertRenewed)
	return s
}

func (s *App) Route(r chi.Router) {
	r.Get("/status", s.GetStatus)
	r.Post("/init", s.Init)
	r.Get("/check", s.Check)
	r.Get("/admin", s.GetAdmin)
	r.Post("/proxy", s.CreateProxy)

	r.Get("/domains", s.DomainList)
	r.Post("/domains", s.DomainCreate)
	r.Delete("/domains/{id}", s.DomainDelete)
	r.Post("/domains/{id}/catch_all", s.DomainCatchAll)
	r.Get("/domains/{id}/dns", s.DomainDNS)
	r.Post("/domains/{id}/dns/publish", s.DomainDNSPublish)

	r.Get("/accounts", s.AccountList)
	r.Post("/accounts", s.AccountCreate)
	r.Put("/accounts/{id}", s.AccountUpdate)
	r.Post("/accounts/{id}/password", s.AccountPassword)
	r.Delete("/accounts/{id}", s.AccountDelete)

	r.Get("/queue", s.QueueList)
	r.Post("/queue/{id}/retry", s.QueueRetry)
	r.Delete("/queue/{id}", s.QueueDelete)

	r.Get("/settings", s.GetSettings)
	r.Post("/settings/hostname", s.UpdateHostname)
	r.Post("/settings/cert", s.UpdateCert)
	r.Post("/settings/listeners", s.UpdateListeners)
	r.Post("/settings/relay", s.UpdateRelay)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, unit)
	return types.AggregateAppStatus(ok)
}

func (s *App) dir() string          { return filepath.Join(app.Root, "server", "stalwart") }
func (s *App) configPath() string   { return filepath.Join(s.dir(), "etc", "config.json") }
func (s *App) envPath() string      { return filepath.Join(s.dir(), "etc", "stalwart.env") }
func (s *App) dataDir() string      { return filepath.Join(s.dir(), "data") + "/" }
func (s *App) logDir() string       { return filepath.Join(s.dir(), "logs") + "/" }
func (s *App) resourcesDir() string { return filepath.Join(s.dir(), "resources") }

// initialized 以面板已保存管理员凭据且配置文件存在为准
func (s *App) initialized() bool {
	user, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminUser)
	return user != "" && io.Exists(s.configPath())
}

// adminClient 用初始化时保存的管理员凭据构造客户端
func (s *App) adminClient() (*client, error) {
	user, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminUser)
	pass, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminPassword)
	if user == "" || pass == "" || !io.Exists(s.configPath()) {
		return nil, errors.New(s.t.Get("mail server is not initialized"))
	}
	return newClient(user, pass), nil
}

// recoveryAdmin 读取安装脚本写入 env 文件的引导凭据
func (s *App) recoveryAdmin() (string, string, error) {
	content, err := io.Read(s.envPath())
	if err != nil {
		return "", "", err
	}
	for line := range strings.SplitSeq(content, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), recoveryEnv+"="); ok {
			user, pass, found := strings.Cut(strings.Trim(value, `"'`), ":")
			if found && user != "" && pass != "" {
				return user, pass, nil
			}
		}
	}
	return "", "", errors.New(s.t.Get("recovery credential not found in %s, please reinstall the app", s.envPath()))
}

// removeRecoveryAdmin 初始化完成后删掉引导凭据，它是一个绕过目录的全权限账号
func (s *App) removeRecoveryAdmin() error {
	content, err := io.Read(s.envPath())
	if err != nil {
		return err
	}
	var kept []string
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), recoveryEnv+"=") {
			continue
		}
		kept = append(kept, line)
	}
	return io.Write(s.envPath(), strings.TrimRight(strings.Join(kept, "\n"), "\n")+"\n", 0640)
}

// latestLogPath 返回当前日志文件，Stalwart 按天轮转文件名带日期后缀
func (s *App) latestLogPath() string {
	entries, err := os.ReadDir(s.logDir())
	if err != nil {
		return ""
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "stalwart.log") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return filepath.Join(s.logDir(), names[len(names)-1])
}

// GetStatus 返回初始化状态与概览，前端据此决定显示初始化向导还是管理页
func (s *App) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := Status{
		Installed:   io.Exists(filepath.Join(s.dir(), "bin", "stalwart")),
		Initialized: s.initialized(),
		HTTPPort:    httpPort,
		LogPath:     s.latestLogPath(),
	}
	status.Running, _ = systemctl.Status(r.Context(), unit)

	if status.Initialized && status.Running {
		if c, err := s.adminClient(); err == nil {
			if settings, err := getOne[swSystemSettings](r.Context(), c, "SystemSettings", singleton); err == nil {
				status.Hostname = settings.DefaultHostname
			}
			if result, err := c.query(r.Context(), "Domain", nil, 0, 0); err == nil {
				status.DomainCount = uint(len(result.IDs))
			}
			s.fillProxyStatus(r.Context(), c, &status)
		}
	}

	service.Success(w, status)
}

// Init 代替 Stalwart 的网页向导完成首次初始化：
// 无配置文件启动即进入引导模式，用引导凭据调用 x:Bootstrap/set 写入配置并拿到永久管理员，
// 随后删除引导凭据、按面板需要调整监听器与离线资源地址
func (s *App) Init(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Init](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if s.initialized() {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("mail server is already initialized"))
		return
	}
	recoveryUser, recoveryPass, err := s.recoveryAdmin()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// 服务已在引导模式下运行时 8080 是它自己占的，只在未启动时查冲突
	if running, _ := systemctl.Status(r.Context(), unit); !running {
		if occupied := s.occupiedPorts(r.Context()); len(occupied) > 0 {
			service.Error(w, http.StatusUnprocessableEntity, s.t.Get("ports %s are occupied by other programs, please stop them first", strings.Join(occupied, ", ")))
			return
		}
	}

	// 初始化要跨多次重启，中途取消会留下半初始化状态
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Minute)
	defer cancel()

	if err = systemctl.Start(ctx, unit); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	bootstrap := newClient(recoveryUser, recoveryPass)
	if err = bootstrap.waitReady(ctx, "x:Bootstrap/get", 60*time.Second); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	resp, err := bootstrap.set(ctx, "Bootstrap", map[string]any{"update": map[string]any{singleton: map[string]any{
		"defaultHostname":       strings.ToLower(req.Hostname),
		"defaultDomain":         strings.ToLower(req.Domain),
		"requestTlsCertificate": false, // 证书由面板签发后推送
		"generateDkimKeys":      true,
		"dataStore":             map[string]any{"@type": "RocksDb", "path": s.dataDir()},
		"blobStore":             map[string]any{"@type": "Default"},
		"searchStore":           map[string]any{"@type": "Default"},
		"inMemoryStore":         map[string]any{"@type": "Default"},
		"directory":             map[string]any{"@type": "Internal"},
		"dnsServer":             map[string]any{"@type": "Manual"},
		"tracer":                map[string]any{"@type": "Log", "path": s.logDir(), "prefix": "stalwart.log", "rotate": "daily", "ansi": false, "level": "info"},
	}}})
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	var admin struct {
		Username string `json:"username"`
		Secret   string `json:"secret"`
	}
	if raw, ok := resp.Updated[singleton]; ok {
		_ = jsonUnmarshal(raw, &admin)
	}
	if admin.Username == "" || admin.Secret == "" {
		service.Error(w, http.StatusInternalServerError, s.t.Get("mail server did not return administrator credentials"))
		return
	}
	if err = s.settingRepo.Set(biz.SettingKeyStalwartAdminUser, admin.Username); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.settingRepo.Set(biz.SettingKeyStalwartAdminPassword, admin.Secret); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 引导模式不会自动切换，必须重启才会读取新配置
	if err = systemctl.Stop(ctx, unit); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.removeRecoveryAdmin(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = systemctl.Start(ctx, unit); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	c := newClient(admin.Username, admin.Secret)
	if err = c.waitReady(ctx, "x:SystemSettings/get", 90*time.Second); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.applyPanelDefaults(ctx, c); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// 监听器改动只在启动时生效
	if err = systemctl.Restart(ctx, unit); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = c.waitReady(ctx, "x:SystemSettings/get", 90*time.Second); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail server initialized"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("hostname", req.Hostname), slog.String("domain", req.Domain))
	service.Success(w, nil)
}

// applyPanelDefaults 把默认监听与资源地址改成面板形态：
// 443 让给面板 web 服务器，HTTP 只听本机，WebUI 与反垃圾规则改用安装脚本预下载的本地文件
func (s *App) applyPanelDefaults(ctx context.Context, c *client) error {
	listeners, err := get[swListener](ctx, c, "NetworkListener", nil, []string{"name", "bind"})
	if err != nil {
		return err
	}
	var destroyIDs []string
	hasSubmission := false
	for _, l := range listeners {
		switch l.Name {
		case "https":
			destroyIDs = append(destroyIDs, l.ID)
		case "http":
			if err = c.update(ctx, "NetworkListener", l.ID, map[string]any{"bind": map[string]bool{fmt.Sprintf("127.0.0.1:%d", httpPort): true}}); err != nil {
				return err
			}
		case "submission":
			hasSubmission = true
		}
	}
	if err = c.destroy(ctx, "NetworkListener", destroyIDs); err != nil {
		return err
	}
	if !hasSubmission {
		if _, err = c.create(ctx, "NetworkListener", listenerSpec("submission")); err != nil {
			return err
		}
	}

	apps, err := get[swApplication](ctx, c, "Application", nil, []string{"urlPrefix"})
	if err != nil {
		return err
	}
	webui := filepath.Join(s.resourcesDir(), "webui.zip")
	for _, a := range apps {
		if a.URLPrefix["/admin"] && io.Exists(webui) {
			if err = c.update(ctx, "Application", a.ID, map[string]any{"resourceUrl": "file://" + webui}); err != nil {
				return err
			}
		}
	}
	rules := filepath.Join(s.resourcesDir(), "spam-filter-rules.json.gz")
	if io.Exists(rules) {
		if err = c.update(ctx, "SpamSettings", singleton, map[string]any{"spamFilterRulesUrl": "file://" + rules}); err != nil {
			return err
		}
	}

	// HTTP 只听本机、全部经面板反代进来，不信任转发头的话登录失败封禁会落在 127.0.0.1 上封掉反代自己
	if err = c.update(ctx, "Http", singleton, map[string]any{"clientIpSource": "xForwardedFor"}); err != nil {
		s.log.Warn(s.t.Get("failed to configure mail server proxy headers"), slog.String("type", biz.OperationTypeApp), slog.String("app", slug), slog.Any("err", err))
	}

	return nil
}

// occupiedPorts 检查邮件端口是否已被其它程序监听，常见是系统自带的 postfix
func (s *App) occupiedPorts(ctx context.Context) []string {
	var occupied []string
	dialer := net.Dialer{Timeout: time.Second}
	for _, port := range []string{"25", "465", "587", "993", "995", "4190", strconv.Itoa(httpPort)} {
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
		if err == nil {
			_ = conn.Close()
			occupied = append(occupied, port)
		}
	}
	return occupied
}

// GetAdmin 返回 Stalwart 管理员凭据，供用户登录它自带的 WebUI
func (s *App) GetAdmin(w http.ResponseWriter, r *http.Request) {
	user, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminUser)
	pass, _ := s.settingRepo.Get(biz.SettingKeyStalwartAdminPassword)
	if user == "" {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("mail server is not initialized"))
		return
	}

	s.log.Info(s.t.Get("app password viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("username", user))
	service.Success(w, map[string]string{"username": user, "password": pass})
}
