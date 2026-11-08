package service

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/webserver"
)

type WebsiteService struct {
	t           *gotext.Locale
	log         *slog.Logger
	websiteRepo *biz.WebsiteUsecase
	settingRepo *biz.SettingUsecase
}

func NewWebsiteService(settingUsecase *biz.SettingUsecase, websiteUsecase *biz.WebsiteUsecase, t *gotext.Locale, log *slog.Logger) *WebsiteService {
	return &WebsiteService{
		t:           t,
		log:         log,
		websiteRepo: websiteUsecase,
		settingRepo: settingUsecase,
	}
}

func (s *WebsiteService) GetRewrites(w http.ResponseWriter, r *http.Request) {
	rewrites, err := s.websiteRepo.GetRewrites()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, rewrites)
}

func (s *WebsiteService) GetDefaultConfig(w http.ResponseWriter, r *http.Request) {
	d, err := s.dialect()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	htmlPath := d.HTMLDir()

	index, _ := io.Read(filepath.Join(htmlPath, "index.html"))
	stop, _ := io.Read(filepath.Join(htmlPath, "stop.html"))
	notFound, _ := io.Read(filepath.Join(htmlPath, "404.html"))
	tlsVersions, _ := s.settingRepo.GetSlice(biz.SettingKeyWebsiteTLSVersions)
	listenIPv6, _ := s.settingRepo.GetBool(biz.SettingKeyWebsiteListenIPv6, false)

	s.log.Info(s.t.Get("website default config viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())))

	Success(w, chix.M{
		"index":        index,
		"stop":         stop,
		"not_found":    notFound,
		"tls_versions": tlsVersions,
		"listen_ipv6":  listenIPv6,
	})
}

func (s *WebsiteService) UpdateDefaultConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteDefaultConfig](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateDefaultConfig(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// dialect 取当前 Web 服务器方言
func (s *WebsiteService) dialect() (webserver.Dialect, error) {
	webServer, err := s.settingRepo.Get(biz.SettingKeyWebserver)
	if err != nil {
		return webserver.Dialect{}, err
	}

	return webserver.Get(webserver.Type(webServer))
}

// filterDefaultHolders 过滤出持有默认站点标志的网站
func filterDefaultHolders(d webserver.Dialect, websites []*biz.Website) []*biz.Website {
	var holders []*biz.Website
	for _, website := range websites {
		if vhost, err := d.NewStaticVhost(filepath.Join(app.Root, "sites", website.Name, "config")); err == nil && vhost.Default() {
			holders = append(holders, website)
		}
	}
	return holders
}

// setWebsiteDefault 增删网站的默认站点标志
func setWebsiteDefault(d webserver.Dialect, name string, isDefault bool) error {
	vhost, err := d.NewStaticVhost(filepath.Join(app.Root, "sites", name, "config"))
	if err != nil {
		return err
	}
	if err = vhost.SetDefault(isDefault); err != nil {
		return err
	}
	return vhost.Save()
}

// GetDefaultSite 获取当前默认站点,0 表示面板内置默认页
func (s *WebsiteService) GetDefaultSite(w http.ResponseWriter, r *http.Request) {
	var id uint
	d, err := s.dialect()
	if err != nil || !d.Features().DefaultSite {
		Success(w, chix.M{"id": id})
		return
	}

	websites, _, err := s.websiteRepo.List("all", "", 0, 1, 10000)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if holders := filterDefaultHolders(d, websites); len(holders) > 0 {
		id = holders[0].ID
	}

	Success(w, chix.M{"id": id})
}

// UpdateDefaultSite 切换默认站点
// 在内置默认配置与网站配置之间迁移默认站点标志,ID 为 0 表示恢复内置默认页
func (s *WebsiteService) UpdateDefaultSite(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteDefaultSite](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	d, err := s.dialect()
	if err != nil || !d.Features().DefaultSite {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("default site is not supported by the current web server"))
		return
	}

	websites, _, err := s.websiteRepo.List("all", "", 0, 1, 10000)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var target *biz.Website
	if req.ID > 0 {
		idx := slices.IndexFunc(websites, func(item *biz.Website) bool { return item.ID == req.ID })
		if idx < 0 {
			Error(w, http.StatusUnprocessableEntity, s.t.Get("website not found"))
			return
		}
		target = websites[idx]
		// 没有当前服务器的配置说明站点尚未重建，保存会写出一份默认值配置顶掉真实站点
		if !io.Exists(filepath.Join(app.Root, "sites", target.Name, "config", d.ConfigFile())) {
			Error(w, http.StatusUnprocessableEntity, s.t.Get("website %s has no config for the current web server, rebuild it first", target.Name))
			return
		}
	}

	holders := filterDefaultHolders(d, websites)

	// 备份待改文件,校验失败时整体回滚
	backups := make(map[string]string)
	backup := func(path string) {
		if content, err := io.Read(path); err == nil {
			backups[path] = content
		}
	}
	if conf := d.DefaultSiteConf(); conf != "" {
		backup(conf)
	}
	for _, website := range holders {
		backup(filepath.Join(app.Root, "sites", website.Name, "config", d.ConfigFile()))
	}
	if target != nil {
		backup(filepath.Join(app.Root, "sites", target.Name, "config", d.ConfigFile()))
	}
	restore := func() {
		for path, content := range backups {
			_ = io.Write(path, content, 0600)
		}
	}

	// 原有默认站点让位
	for _, website := range holders {
		if target != nil && website.ID == target.ID {
			continue
		}
		if err = setWebsiteDefault(d, website.Name, false); err != nil {
			restore()
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	if target != nil {
		if err = setWebsiteDefault(d, target.Name, true); err != nil {
			restore()
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	// 目标为空时内置默认页重新持有默认位
	if err = d.WriteDefaultSite(target == nil); err != nil {
		restore()
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if out, testErr := d.Test(r.Context()); testErr != nil {
		restore()
		Error(w, http.StatusInternalServerError, s.t.Get("config test failed: %v %s", testErr, out))
		return
	}
	// 配置已落盘且不再回滚，reload 断开取消链，否则磁盘配置与运行中的配置会不一致
	if err = d.Reload(context.WithoutCancel(r.Context())); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website default site updated"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(req.ID)), slog.String("name", lo.FromPtr(target).Name))

	Success(w, nil)
}

// UpdateCert 用于自动化工具更新证书
func (s *WebsiteService) UpdateCert(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateCert](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateCert(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// List 网站列表
func (s *WebsiteService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	websites, total, err := s.websiteRepo.List(req.Type, req.Keyword, req.ProjectID, req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website list viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("website_type", req.Type), slog.String("keyword", req.Keyword), slog.Uint64("project_id", uint64(req.ProjectID)), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": websites,
	})
}

func (s *WebsiteService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if len(req.Path) == 0 {
		req.Path, _ = s.settingRepo.Get(biz.SettingKeyWebsitePath)
		req.Path = filepath.Join(req.Path, req.Name, "public")
	}
	if protectedPath(req.Path) {
		Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
		return
	}

	_, warnings, err := s.websiteRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{"warnings": warnings})
}

func (s *WebsiteService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := s.websiteRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("website config viewed"), slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(req.ID)), slog.String("name", config.Name))

	Success(w, config)
}

func (s *WebsiteService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if protectedPath(req.Path) {
		Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
		return
	}

	if err = s.websiteRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) SwitchType(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteSwitchType](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.SwitchType(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.Delete(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) UpdateRemark(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateRemark](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateRemark(r.Context(), req.ID, req.Remark); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) ResetConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.ResetConfig(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) UpdateProject(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateProject](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateProject(r.Context(), req.ID, req.ProjectID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateStatus](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateStatus(r.Context(), req.ID, req.Status); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) UpdateExpireAt(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateExpireAt](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	var expireAt *time.Time
	if req.ExpireAt != "" {
		t, err := time.Parse(time.DateTime, req.ExpireAt)
		if err != nil {
			Error(w, http.StatusUnprocessableEntity, "%v", err)
			return
		}
		expireAt = &t
	}

	if err = s.websiteRepo.UpdateExpireAt(r.Context(), req.ID, expireAt); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) ObtainCert(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteObtainCert](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.ObtainCert(r.Context(), req.ID, req.DNSID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
