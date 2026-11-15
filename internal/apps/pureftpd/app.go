package pureftpd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

// passwordPipe 把密码源源不断地喂给 pure-pw 的两次密码提示
// 密码取自环境变量 FTP_PASSWORD，不进命令行也就不受 shell 元字符影响
const passwordPipe = `while printf '%%s\n' "$FTP_PASSWORD"; do :; done | `

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
		return
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

	s.log.Info(s.t.Get("ftp user list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))
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
	if _, err = shell.ExecfWithEnv(ctx, s.passwordEnv(req.Password), passwordPipe+`pure-pw useradd %s -u www -g www -d %s`, shell.Quote(req.Username), shell.Quote(req.Path)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf(ctx, "pure-pw mkdb"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("ftp user created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.String("username", req.Username), slog.String("path", req.Path))
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

	s.log.Warn(s.t.Get("ftp user deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.String("username", req.Username))
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
	if _, err = shell.ExecfWithEnv(ctx, s.passwordEnv(req.Password), passwordPipe+`pure-pw passwd %s -m`, shell.Quote(req.Username)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf(ctx, "pure-pw mkdb"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("ftp user password updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.String("username", req.Username))
	service.Success(w, nil)
}

// GetPort 获取端口
func (s *App) GetPort(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(s.ConfPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get port: %v", err))
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.String("path", s.ConfPath()))
	service.Success(w, s.Port(config))
}

// UpdatePort 设置端口
func (s *App) UpdatePort(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdatePort](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := s.ConfPath()
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
	running, err := common.OpenPort(ctx, req.Port, "")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(ctx, "pure-ftpd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app port updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.Uint64("port", uint64(req.Port)), slog.Bool("firewall_opened", running))
	service.Success(w, nil)
}

// GetConfigTune 获取 Pure-FTPd 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	confPath := s.ConfPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := new(ConfigTune)
	confval.FTP.Read(config, s.tuneFields(tune))

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.String("path", confPath))
	service.Success(w, tune)
}

// UpdateConfigTune 更新 Pure-FTPd 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := s.ConfPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = confval.FTP.Write(config, s.tuneFields(req))

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "pure-ftpd"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "pureftpd"), slog.String("max_clients_number", req.MaxClientsNumber), slog.String("max_clients_per_ip", req.MaxClientsPerIP), slog.String("max_idle_time", req.MaxIdleTime), slog.String("max_load", req.MaxLoad), slog.String("passive_port_range", req.PassivePortRange), slog.String("anonymous_only", req.AnonymousOnly), slog.String("no_anonymous", req.NoAnonymous), slog.String("max_disk_usage", req.MaxDiskUsage))
	service.Success(w, nil)
}

// ConfPath 主配置文件路径
func (s *App) ConfPath() string {
	return app.Root + "/server/pure-ftpd/etc/pure-ftpd.conf"
}

// Port 从配置内容解析监听端口，Bind 形如 "0.0.0.0,21"，未指定端口时为默认的 21
func (s *App) Port(config string) int {
	bind := strings.Trim(confval.FTP.Get(config, "Bind"), `"'`)
	if _, port, ok := strings.Cut(bind, ","); ok {
		return cast.ToInt(strings.TrimSpace(port))
	}

	return 21
}

func (s *App) passwordEnv(password string) []string {
	return []string{"FTP_PASSWORD=" + password}
}

func (s *App) tuneFields(t *ConfigTune) []confval.Field {
	return []confval.Field{
		{Key: "MaxClientsNumber", Value: &t.MaxClientsNumber},
		{Key: "MaxClientsPerIP", Value: &t.MaxClientsPerIP},
		{Key: "MaxIdleTime", Value: &t.MaxIdleTime},
		{Key: "MaxLoad", Value: &t.MaxLoad},
		{Key: "PassivePortRange", Value: &t.PassivePortRange},
		{Key: "AnonymousOnly", Value: &t.AnonymousOnly},
		{Key: "NoAnonymous", Value: &t.NoAnonymous},
		{Key: "MaxDiskUsage", Value: &t.MaxDiskUsage},
	}
}
