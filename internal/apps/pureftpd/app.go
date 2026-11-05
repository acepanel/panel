package pureftpd

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
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

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "pure-ftpd")
	return types.AggregateAppStatus(ok)
}

// List 获取用户列表
func (s *App) List(w http.ResponseWriter, r *http.Request) {
	listRaw, err := shell.Execf(r.Context(), "pure-pw list")
	if err != nil {
		service.Success(w, chix.M{
			"total": 0,
			"items": []User{},
		})
	}

	userRe := regexp.MustCompile(`(\S+)\s+(\S+)`)
	users := lo.FilterMap(strings.Split(listRaw, "\n"), func(v string, _ int) (User, bool) {
		if len(v) == 0 {
			return User{}, false
		}
		match := userRe.FindStringSubmatch(v)
		return User{
			Username: match[1],
			Path:     strings.Replace(match[2], "/./", "/", 1),
		}, true
	})

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

	// mkdb 被取消会让文本库已改、二进制库未重建，FTP 认证与面板显示不一致
	ctx := context.WithoutCancel(r.Context())
	if _, err = shell.Execf(ctx, `yes '%s' | pure-pw useradd '%s' -u www -g www -d '%s'`, req.Password, req.Username, req.Path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf(ctx, "pure-pw mkdb"); err != nil {
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

	// mkdb 被取消会让二进制库里仍留着已删的用户，FTP 照旧认证通过
	ctx := context.WithoutCancel(r.Context())
	if _, err = shell.Execf(ctx, "pure-pw userdel '%s' -m", req.Username); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf(ctx, "pure-pw mkdb"); err != nil {
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

	// mkdb 被取消会让二进制库仍是旧密码，FTP 认的还是改前的密码
	ctx := context.WithoutCancel(r.Context())
	if _, err = shell.Execf(ctx, `yes '%s' | pure-pw passwd '%s' -m`, req.Password, req.Username); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf(ctx, "pure-pw mkdb"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetPort 获取端口
func (s *App) GetPort(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(app.Root + "/server/pure-ftpd/etc/pure-ftpd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get port: %v", err))
		return
	}

	bind := strings.Trim(confval.FTP.Get(config, "Bind"), `"'`)
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

	confPath := app.Root + "/server/pure-ftpd/etc/pure-ftpd.conf"
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	config = confval.FTP.Set(config, "Bind", fmt.Sprintf(`"0.0.0.0,%d"`, req.Port))
	if err = io.Write(confPath, config, 0644); err != nil {
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

	if err = systemctl.Restart(ctx, "pure-ftpd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetConfigTune 获取 Pure-FTPd 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(app.Root + "/server/pure-ftpd/etc/pure-ftpd.conf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		MaxClientsNumber: confval.FTP.Get(config, "MaxClientsNumber"),
		MaxClientsPerIP:  confval.FTP.Get(config, "MaxClientsPerIP"),
		MaxIdleTime:      confval.FTP.Get(config, "MaxIdleTime"),
		MaxLoad:          confval.FTP.Get(config, "MaxLoad"),
		PassivePortRange: confval.FTP.Get(config, "PassivePortRange"),
		AnonymousOnly:    confval.FTP.Get(config, "AnonymousOnly"),
		NoAnonymous:      confval.FTP.Get(config, "NoAnonymous"),
		MaxDiskUsage:     confval.FTP.Get(config, "MaxDiskUsage"),
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

	confPath := app.Root + "/server/pure-ftpd/etc/pure-ftpd.conf"
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = confval.FTP.Set(config, "MaxClientsNumber", req.MaxClientsNumber)
	config = confval.FTP.Set(config, "MaxClientsPerIP", req.MaxClientsPerIP)
	config = confval.FTP.Set(config, "MaxIdleTime", req.MaxIdleTime)
	config = confval.FTP.Set(config, "MaxLoad", req.MaxLoad)
	config = confval.FTP.Set(config, "PassivePortRange", req.PassivePortRange)
	config = confval.FTP.Set(config, "AnonymousOnly", req.AnonymousOnly)
	config = confval.FTP.Set(config, "NoAnonymous", req.NoAnonymous)
	config = confval.FTP.Set(config, "MaxDiskUsage", req.MaxDiskUsage)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "pure-ftpd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
