package stalwart

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/types"
)

// 每个邮件域名下需要经面板 Web 服务器反代到 Stalwart 的子域：客户端自动配置与 MTA-STS 策略
var proxySubdomains = []string{"autoconfig", "autodiscover", "mta-sts", "ua-auto-config"}

func proxyUpstream() string { return fmt.Sprintf("http://127.0.0.1:%d", httpPort) }

// proxyDomains 返回反代站点应覆盖的全部主机名：服务器主机名加每个邮件域的子域
func (s *App) proxyDomains(ctx context.Context, c *client) (string, []string, error) {
	settings, err := getOne[swSystemSettings](ctx, c, "SystemSettings", singleton)
	if err != nil {
		return "", nil, err
	}
	domains, err := get[swDomain](ctx, c, "Domain", nil, []string{"name"})
	if err != nil {
		return "", nil, err
	}
	hosts := []string{settings.DefaultHostname}
	for _, d := range domains {
		for _, sub := range proxySubdomains {
			hosts = append(hosts, sub+"."+d.Name)
		}
	}
	return settings.DefaultHostname, lo.Uniq(hosts), nil
}

// proxyWebsite 找到一键创建的反代站点：先按记录的 ID，再按主机名兜底，都没有返回 nil
func (s *App) proxyWebsite(hostname string) *types.WebsiteSetting {
	if id, _ := s.settingRepo.GetInt(biz.SettingKeyStalwartProxyWebsite); id > 0 {
		if website, err := s.website.Get(uint(id)); err == nil {
			return website
		}
	}
	if website, err := s.website.GetByName(hostname); err == nil {
		_ = s.settingRepo.Set(biz.SettingKeyStalwartProxyWebsite, cast.ToString(website.ID))
		return website
	}
	return nil
}

func (s *App) fillProxyStatus(ctx context.Context, c *client, status *Status) {
	hostname, domains, err := s.proxyDomains(ctx, c)
	if err != nil {
		return
	}
	status.ProxyDomains = domains
	website := s.proxyWebsite(hostname)
	if website == nil {
		return
	}
	status.ProxyWebsiteID = website.ID
	status.ProxyWebsiteName = website.Name
	status.ProxyMissingDomains = lo.Filter(domains, func(d string, _ int) bool {
		return !slices.Contains(website.Domains, d)
	})
}

// CreateProxy 一键创建（或同步）反代站点：没有就以主机名建一个 proxy 类型网站指向 Stalwart 的 HTTP 监听，
// 已有则补齐新增域名的子域；选了证书就部署并开启 HTTPS，邮件服务还没绑证书时顺带绑上
func (s *App) CreateProxy(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProxyCreate](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	hostname, domains, err := s.proxyDomains(r.Context(), c)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 建站、部署证书、重载 Web 服务器是多步落盘操作，中途取消会留下半成品
	ctx := context.WithoutCancel(r.Context())
	var warnings []string
	website := s.proxyWebsite(hostname)
	created := website == nil
	if created {
		websitePath, _ := s.settingRepo.Get(biz.SettingKeyWebsitePath)
		w2, warns, err := s.website.Create(ctx, &request.WebsiteCreate{
			Type:    string(biz.WebsiteTypeProxy),
			Name:    hostname,
			Listens: []string{"80"},
			Domains: domains,
			Path:    filepath.Join(websitePath, hostname, "public"),
			Proxy:   proxyUpstream(),
			Remark:  s.t.Get("Stalwart mail server reverse proxy"),
		})
		if err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		warnings = warns
		if err = s.settingRepo.Set(biz.SettingKeyStalwartProxyWebsite, cast.ToString(w2.ID)); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if website, err = s.website.Get(w2.ID); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	} else if missing := lo.Filter(domains, func(d string, _ int) bool { return !slices.Contains(website.Domains, d) }); len(missing) > 0 {
		update, err := toWebsiteUpdate(website)
		if err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		update.Domains = append(update.Domains, missing...)
		if err = s.website.Update(ctx, update); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	certBound := false
	if req.CertID > 0 {
		if err = s.cert.Deploy(ctx, req.CertID, []uint{website.ID}, true); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if bound, _ := s.settingRepo.GetInt(biz.SettingKeyStalwartCertID); bound == 0 {
			if cert, err := s.certRepo.Get(req.CertID); err == nil {
				if err = s.pushCert(ctx, c, cert); err != nil {
					warnings = append(warnings, s.t.Get("failed to bind certificate to mail server: %v", err))
				} else if err = s.settingRepo.Set(biz.SettingKeyStalwartCertID, cast.ToString(cert.ID)); err == nil {
					certBound = true
				}
			}
		}
	}

	s.log.Info(s.t.Get("mail server proxy website synced"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.Bool("created", created), slog.Uint64("website_id", uint64(website.ID)), slog.String("name", website.Name), slog.Any("domains", domains), slog.Uint64("cert_id", uint64(req.CertID)), slog.Bool("cert_bound", certBound), slog.String("upstream", proxyUpstream()))
	service.Success(w, chix.M{"warnings": warnings})
}

// toWebsiteUpdate 网站设置与更新请求字段同名，走一次 JSON 转换即可原样回写
func toWebsiteUpdate(setting *types.WebsiteSetting) (*request.WebsiteUpdate, error) {
	raw, err := json.Marshal(setting)
	if err != nil {
		return nil, err
	}
	var update request.WebsiteUpdate
	if err = json.Unmarshal(raw, &update); err != nil {
		return nil, err
	}
	update.Domains = lo.Uniq(lo.Map(update.Domains, func(d string, _ int) string { return strings.ToLower(d) }))
	return &update, nil
}
