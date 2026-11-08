package fail2ban

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
	webserver "github.com/acepanel/panel/v3/pkg/webserver/types"
)

type App struct {
	t           *gotext.Locale
	log         *slog.Logger
	websiteRepo biz.WebsiteRepo
}

func NewApp(t *gotext.Locale, log *slog.Logger, websiteRepo biz.WebsiteRepo) *App {
	return &App{
		t:           t,
		log:         log,
		websiteRepo: websiteRepo,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/jails", s.List)
	r.Post("/jails", s.Create)
	r.Post("/jails/{name}", s.Update)
	r.Delete("/jails/{name}", s.Delete)
	r.Get("/jails/{name}/ban", s.BanList)
	r.Post("/unban", s.Unban)
	r.Post("/white_list", s.SetWhiteList)
	r.Get("/white_list", s.GetWhiteList)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "fail2ban")
	return types.AggregateAppStatus(ok)
}

// List 所有规则
func (s *App) List(w http.ResponseWriter, r *http.Request) {
	jails, err := listJails()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := service.Paginate(r, jails)

	s.log.Info(s.t.Get("fail2ban jail list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))
	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// Create 添加规则
func (s *App) Create(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Add](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	jail := Jail{
		Enabled:  true,
		MaxRetry: req.MaxRetry,
		FindTime: req.FindTime,
		BanTime:  req.BanTime,
	}

	switch req.Type {
	case "website":
		website, err := s.websiteRepo.GetByName(req.WebsiteName)
		if err != nil {
			service.Error(w, http.StatusUnprocessableEntity, "%v", err)
			return
		}

		ports := lo.FilterMap(website.Listens, func(listen webserver.Listen, _ int) (string, bool) {
			port, err := cast.ToIntE(listen.Address)
			return strconv.Itoa(port), err == nil
		})

		jail.Name = req.WebsiteName + "-" + req.WebsiteMode
		jail.Filter = panelFilterPrefix + jail.Name
		jail.Port = strings.Join(ports, ",")
		jail.LogPath = app.Root + "/sites/" + website.Name + "/log/access.log"

	case "service":
		var filter, port string
		var err error
		switch req.Name {
		case "ssh":
			filter = "sshd"
			port, err = shell.Execf(r.Context(), "cat /etc/ssh/sshd_config | grep 'Port ' | awk '{print $2}' | paste -sd ','")
		case "mysql":
			filter = "mysqld-auth"
			port, err = shell.Execf(r.Context(), "cat %s/server/mysql/conf/my.cnf | grep 'port' | head -n 1 | awk '{print $3}'", app.Root)
		case "pure-ftpd":
			filter = "pure-ftpd"
			port, err = shell.Execf(r.Context(), `cat %s/server/pure-ftpd/etc/pure-ftpd.conf | grep "Bind" | awk '{print $2}' | awk -F "," '{print $2}'`, app.Root)
		default:
			service.Error(w, http.StatusUnprocessableEntity, s.t.Get("unknown service"))
			return
		}
		if len(port) == 0 || err != nil {
			service.Error(w, http.StatusUnprocessableEntity, s.t.Get("get service port failed, please check if it is installed"))
			return
		}

		jail.Name = req.Name
		jail.Filter = filter
		jail.Port = port
	}

	if io.Exists(jailPath(jail.Name)) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("rule already exists"))
		return
	}

	// 网站规则的过滤器由面板生成，服务规则复用 fail2ban 自带的
	if req.Type == "website" {
		failRegex := `^<HOST>\s-.*HTTP/.*$`
		if req.WebsiteMode == "path" {
			failRegex = `^<HOST>\s-.*\s` + req.WebsitePath + `.*HTTP/.*$`
		}
		filter := "[Definition]\nfailregex = " + failRegex + "\nignoreregex =\n"
		if err = io.Write(filterPath(jail.Filter), filter, 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = writeJail(jail); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("fail2ban jail created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.String("name", jail.Name), slog.String("jail_type", req.Type), slog.String("port", jail.Port), slog.Int("max_retry", req.MaxRetry), slog.Int("find_time", req.FindTime), slog.Int("ban_time", req.BanTime), slog.String("website_path", req.WebsitePath))
	service.Success(w, nil)
}

// Update 修改规则
func (s *App) Update(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Update](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	jail, err := readJail(req.Name)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("rule not found"))
		return
	}

	jail.Enabled = req.Enabled
	jail.MaxRetry = req.MaxRetry
	jail.FindTime = req.FindTime
	jail.BanTime = req.BanTime

	if err = writeJail(jail); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("fail2ban jail updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.String("name", jail.Name), slog.Bool("enabled", req.Enabled), slog.Int("max_retry", req.MaxRetry), slog.Int("find_time", req.FindTime), slog.Int("ban_time", req.BanTime))
	service.Success(w, nil)
}

// Delete 删除规则
func (s *App) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[JailName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	jail, err := readJail(req.Name)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("rule not found"))
		return
	}

	// 规则与配套过滤器要一起删掉，中途取消会留下孤立的过滤器文件
	ctx := context.WithoutCancel(r.Context())
	if err = io.Remove(jailPath(jail.Name)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// 面板生成的过滤器随规则一起删除，fail2ban 自带的保留
	if strings.HasPrefix(jail.Filter, panelFilterPrefix) {
		if err = io.Remove(filterPath(jail.Filter)); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = s.reload(ctx); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("fail2ban jail deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.String("app", "fail2ban"), slog.String("name", jail.Name), slog.String("filter", jail.Filter), slog.String("port", jail.Port))
	service.Success(w, nil)
}

// BanList 获取封禁列表
func (s *App) BanList(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[JailName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	out, err := shell.Execf(r.Context(), "fail2ban-client status %s", req.Name)
	if err != nil {
		// 规则文件在但 fail2ban 没加载，多是写入后没重载，重启 fail2ban 即可
		if strings.Contains(out, "does not exist") {
			service.Error(w, http.StatusUnprocessableEntity, s.t.Get("rule %s is not running, please try restarting Fail2ban", req.Name))
			return
		}
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get the status of rule %s: %v", req.Name, err))
		return
	}

	// 输出形如 "   |- Currently banned:	1"，行首有树状前缀，按首个冒号切分后比对尾部
	var currentlyBan, totalBan, bannedIP string
	for line := range strings.SplitSeq(out, "\n") {
		label, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		switch {
		case strings.HasSuffix(label, "Currently banned"):
			currentlyBan = strings.TrimSpace(value)
		case strings.HasSuffix(label, "Total banned"):
			totalBan = strings.TrimSpace(value)
		case strings.HasSuffix(label, "Banned IP list"):
			bannedIP = strings.TrimSpace(value)
		}
	}

	list := lo.Map(strings.Fields(bannedIP), func(ip string, _ int) map[string]string {
		return map[string]string{
			"name": req.Name,
			"ip":   ip,
		}
	})

	s.log.Info(s.t.Get("fail2ban ban list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.String("jail", req.Name))
	service.Success(w, chix.M{
		"currently_ban": currentlyBan,
		"total_ban":     totalBan,
		"baned_list":    list,
	})
}

// Unban 解封
func (s *App) Unban(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Unban](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 解封一旦下发就无法回滚，中途取消只会让面板报错而 IP 实际已解封
	if _, err = shell.Execf(context.WithoutCancel(r.Context()), "fail2ban-client set %s unbanip %s", req.Name, req.IP); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("fail2ban ip unbanned"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.String("jail", req.Name), slog.String("ip", req.IP))
	service.Success(w, nil)
}

// SetWhiteList 设置白名单
func (s *App) SetWhiteList(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SetWhiteList](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	raw, _ := io.Read(jailLocal)
	if err = io.Write(jailLocal, confval.SectionINI.SetIn(raw, "DEFAULT", "ignoreip", req.IP), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("fail2ban whitelist saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.String("path", jailLocal), slog.String("ignore_ip", req.IP))
	service.Success(w, nil)
}

// GetWhiteList 获取白名单
func (s *App) GetWhiteList(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.Read(jailLocal)

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "fail2ban"), slog.String("path", jailLocal))
	service.Success(w, confval.SectionINI.GetIn(raw, "DEFAULT", "ignoreip"))
}

func (s *App) reload(ctx context.Context) error {
	// 规则已落盘，重载不跟随请求取消，否则磁盘规则与运行中的 fail2ban 不一致
	_, err := shell.Execf(context.WithoutCancel(ctx), "fail2ban-client reload")
	return err
}
