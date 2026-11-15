package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/embed"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/punycode"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

type websiteRepo struct {
	t       *gotext.Locale
	db      *gorm.DB
	setting biz.SettingRepo
}

func NewWebsiteRepo(db *gorm.DB, t *gotext.Locale, settingRepo biz.SettingRepo) biz.WebsiteRepo {
	return &websiteRepo{
		t:       t,
		db:      db,
		setting: settingRepo,
	}
}

func (r *websiteRepo) GetRewrites() (map[string]string, error) {
	d, err := r.dialect()
	if err != nil {
		return make(map[string]string), nil //nolint:nilerr
	}

	entries, err := embed.RewritesFS.ReadDir(filepath.Join("rewrites", d.RewritesDir()))
	if err != nil {
		return make(map[string]string), nil //nolint:nilerr
	}

	rw := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if content, err := embed.RewritesFS.ReadFile(filepath.Join("rewrites", d.RewritesDir(), entry.Name())); err == nil {
			rw[strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))] = string(content)
		}
	}

	return rw, nil
}

func (r *websiteRepo) UpdateDefaultConfig(ctx context.Context, req *request.WebsiteDefaultConfig) error {
	d, err := r.dialect()
	if err != nil {
		return err
	}
	htmlPath := d.HTMLDir()

	if err = io.Write(filepath.Join(htmlPath, "index.html"), req.Index, 0644); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(htmlPath, "stop.html"), req.Stop, 0644); err != nil {
		return err
	}
	if req.NotFound != "" {
		if err = io.Write(filepath.Join(htmlPath, "404.html"), req.NotFound, 0644); err != nil {
			return err
		}
	}
	if err = r.setting.SetSlice(biz.SettingKeyWebsiteTLSVersions, req.TLSVersions); err != nil {
		return err
	}
	if err = r.setting.Set(biz.SettingKeyWebsiteListenIPv6, strconv.FormatBool(req.ListenIPv6)); err != nil {
		return err
	}

	return r.ReloadWebServer(ctx)
}

func (r *websiteRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.Website{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *websiteRepo) Get(id uint) (*types.WebsiteSetting, error) {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return nil, err
	}

	vhost, err := r.getVhost(website)
	if err != nil {
		return nil, err
	}

	return r.loadSetting(website, vhost)
}

// loadSetting 从 vhost 读取站点的完整设置
func (r *websiteRepo) loadSetting(website *biz.Website, vhost webservertypes.Vhost) (*types.WebsiteSetting, error) {
	var err error
	setting := new(types.WebsiteSetting)
	setting.ID = website.ID
	setting.Name = website.Name
	setting.Type = string(website.Type)
	setting.Path = website.Path
	setting.SSL = website.SSL
	// 监听地址
	setting.Listens = vhost.Listen()
	// 域名
	domains := vhost.ServerName()
	domains = lo.Map(domains, func(d string, _ int) string { return tools.UnwrapIPv6(d) })
	domains, err = punycode.DecodeDomains(domains)
	if err != nil {
		return nil, err
	}
	setting.Domains = domains
	// 运行目录
	setting.Root = vhost.Root()
	// 默认文档
	setting.Index = vhost.Index()
	// 防跨站
	if website.Type == biz.WebsiteTypePHP && io.Exists(filepath.Join(setting.Root, ".user.ini")) {
		userIni, _ := io.Read(filepath.Join(setting.Root, ".user.ini"))
		if strings.Contains(userIni, "open_basedir") {
			setting.OpenBasedir = true
		}
	}
	// SSL，切换 Web 服务器后配置可能尚未生成
	if sslConfig := vhost.SSLConfig(); setting.SSL && sslConfig != nil {
		setting.HTTPRedirect = sslConfig.HTTPRedirect
		setting.HSTS = sslConfig.HSTS
		setting.OCSP = sslConfig.OCSP
		setting.SSLProtocols = sslConfig.Protocols
	}
	// 证书
	crt, _ := os.ReadFile(filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem"))
	setting.SSLCert = string(crt)
	key, _ := os.ReadFile(filepath.Join(app.Root, "sites", website.Name, "config", "private.key"))
	setting.SSLKey = string(key)
	// 解析证书信息
	if decode, err := cert.ParseCert(crt); err == nil {
		setting.SSLNotBefore = decode.NotBefore.Format(time.DateTime)
		setting.SSLNotAfter = decode.NotAfter.Format(time.DateTime)
		setting.SSLIssuer = decode.Issuer.CommonName
		setting.SSLOCSPServer = decode.OCSPServer
		setting.SSLDNSNames = cert.SANs(decode)
	}
	// 访问日志
	if setting.AccessLog = vhost.AccessLog(); setting.AccessLog == "" {
		setting.AccessLog = fmt.Sprintf("%s/sites/%s/log/access.log", app.Root, website.Name)
	}
	// 错误日志
	if setting.ErrorLog = vhost.ErrorLog(); setting.ErrorLog == "" {
		setting.ErrorLog = fmt.Sprintf("%s/sites/%s/log/error.log", app.Root, website.Name)
	}

	// PHP 网站特有
	if phpVhost, ok := vhost.(webservertypes.PHPVhost); ok {
		setting.PHP = phpVhost.PHP()
		// 伪静态
		setting.Rewrite = phpVhost.Config("010-rewrite.conf", webservertypes.ScopeSite)
	}

	// 反向代理
	if proxyVhost, ok := vhost.(webservertypes.VhostProxy); ok {
		setting.Upstreams = proxyVhost.Upstreams()
		setting.Proxies = proxyVhost.Proxies()
	}

	// 重定向配置
	if redirectVhost, ok := vhost.(webservertypes.VhostRedirect); ok {
		setting.Redirects = redirectVhost.Redirects()
	}

	// 高级设置（限流限速、真实 IP、基本认证）
	setting.RateLimit = vhost.RateLimit()
	setting.RealIP = vhost.RealIP()
	// 读取基本认证规则及用户列表
	for _, auth := range vhost.BasicAuth() {
		setting.BasicAuth = append(setting.BasicAuth, types.WebsiteBasicAuth{
			Path:  auth.Path,
			Users: r.readBasicAuthUsers(auth.UserFile),
		})
	}

	// 自定义配置
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	setting.CustomConfigs = r.getCustomConfigs(configDir)

	// 访问统计
	setting.StatEnabled = vhost.Config("021-stats-log.conf", webservertypes.ScopeSite) != ""
	setting.LSCache = vhost.Config("020-lscache.conf", webservertypes.ScopeSite) != ""

	return setting, err
}

func (r *websiteRepo) GetByName(name string) (*types.WebsiteSetting, error) {
	website := new(biz.Website)
	if err := r.db.Where("name", name).First(website).Error; err != nil {
		return nil, err
	}

	return r.Get(website.ID)
}

func (r *websiteRepo) List(typ, keyword string, projectID, page, limit uint) ([]*biz.Website, int64, error) {
	websites := make([]*biz.Website, 0)
	var total int64

	query := r.db.Model(&biz.Website{})
	if typ != "" && typ != "all" {
		query = query.Where("type = ?", typ)
	}
	if keyword != "" {
		query = query.Where("name LIKE ? OR remark LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id DESC").Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&websites).Error; err != nil {
		return nil, 0, err
	}

	// 取证书剩余有效时间和PHP版本
	d, dErr := r.dialect()
	for _, website := range websites {
		crt, _ := os.ReadFile(filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem"))
		if decode, err := cert.ParseCert(crt); err == nil {
			hours := time.Until(decode.NotAfter).Hours()
			website.CertExpire = fmt.Sprintf("%.2f", hours/24)
		}
		if dErr != nil {
			continue
		}
		vhost, err := newVhost(d, website)
		if err != nil {
			continue
		}
		if php, ok := vhost.(webservertypes.PHPVhost); ok {
			website.PHP = php.PHP()
		}
		// 获取域名
		if domains, err := punycode.DecodeDomains(vhost.ServerName()); err == nil {
			website.Domains = domains
		}
	}

	return websites, total, nil
}

func (r *websiteRepo) All() ([]*biz.Website, error) {
	websites := make([]*biz.Website, 0)
	if err := r.db.Order("id DESC").Find(&websites).Error; err != nil {
		return nil, err
	}

	return websites, nil
}

func (r *websiteRepo) Create(ctx context.Context, req *request.WebsiteCreate) (*biz.Website, error) {
	w := &biz.Website{
		Name:      req.Name,
		Type:      biz.WebsiteType(req.Type),
		Status:    true,
		Path:      req.Path,
		SSL:       false,
		Remark:    req.Remark,
		ProjectID: req.ProjectID,
	}

	d, err := r.dialect()
	if err != nil {
		return nil, err
	}
	vhost, err := newVhost(d, w)
	if err != nil {
		return nil, err
	}

	// 配置一落盘就会被 Web 服务器加载，记录必须先入库，且失败时连同目录一起回滚，
	// 否则会留下面板看不见、却已对外服务的站点
	if err = r.db.Create(w).Error; err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = r.db.Delete(w).Error
			_ = io.Remove(filepath.Join(app.Root, "sites", req.Name))
		}
	}()

	// 创建配置文件目录
	if err = r.makeConfigDirs(filepath.Join(app.Root, "sites", req.Name, "config")); err != nil {
		return nil, err
	}
	// 创建日志目录
	if err = os.MkdirAll(filepath.Join(app.Root, "sites", req.Name, "log"), 0755); err != nil {
		return nil, err
	}

	// 监听地址
	listens := lo.Map(req.Listens, func(listen string, _ int) webservertypes.Listen {
		return webservertypes.Listen{Address: listen}
	})
	listenIPv6, err := r.setting.GetBool(biz.SettingKeyWebsiteListenIPv6, false)
	if err != nil {
		return nil, err
	}
	if d.Features().IPv6Listen && listenIPv6 {
		listens = webserver.WithIPv6Listens(listens)
	}
	if err = vhost.SetListen(listens); err != nil {
		return nil, err
	}
	// 域名
	domains, err := punycode.EncodeDomains(req.Domains)
	if err != nil {
		return nil, err
	}
	domains = lo.Map(domains, func(d string, _ int) string { return tools.WrapIPv6(d) })
	if err = r.checkDomains(d, req.Name, domains, listens); err != nil {
		return nil, err
	}
	if err = vhost.SetServerName(domains); err != nil {
		return nil, err
	}
	// 运行目录
	if err = vhost.SetRoot(req.Path); err != nil {
		return nil, err
	}
	// 日志
	if err = vhost.SetAccessLog(filepath.Join(app.Root, "sites", req.Name, "log", "access.log")); err != nil {
		return nil, err
	}
	if err = vhost.SetErrorLog(filepath.Join(app.Root, "sites", req.Name, "log", "error.log")); err != nil {
		return nil, err
	}
	// 404 页面、PHP 缓存或静态 SPA 回退
	if err = r.writeTypeConfigs(d, vhost, w.Type); err != nil {
		return nil, err
	}

	// 反代站点的整站根规则
	if proxyVhost, ok := vhost.(webservertypes.VhostProxy); ok && w.Type == biz.WebsiteTypeProxy {
		if err = proxyVhost.SetProxies([]webservertypes.Proxy{
			{
				Location: "^~ /",
				Pass:     req.Proxy,
				Host:     webservertypes.DefaultProxyHost(req.Proxy, nil),
			},
		}); err != nil {
			return nil, err
		}
	}

	// PHP 支持
	if phpVhost, ok := vhost.(webservertypes.PHPVhost); ok {
		if err = phpVhost.SetPHP(req.PHP); err != nil {
			return nil, err
		}
		if err = phpVhost.SetIndex([]string{"index.php", "index.html"}); err != nil {
			return nil, err
		}
		if err = phpVhost.SetRawConfig("010-rewrite.conf", webservertypes.ScopeSite, ""); err != nil {
			return nil, err
		}
	}

	// 初始化网站目录
	if err = os.MkdirAll(req.Path, 0755); err != nil {
		return nil, err
	}
	var index []byte
	switch app.Locale {
	case "zh_CN":
		index, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "index_zh_CN.html"))
	case "zh_TW":
		index, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "index_zh_TW.html"))
	default:
		index, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "index.html"))
	}
	if err != nil {
		return nil, errors.New(r.t.Get("failed to get index template file: %v", err))
	}
	// 目录可能已有用户文件，存在则不覆盖
	indexPath := filepath.Join(req.Path, "index.html")
	if !io.Exists(indexPath) {
		if err = io.Write(indexPath, string(index), 0644); err != nil {
			return nil, err
		}
	}
	var notFound []byte

	// 如果存在自定义 404 页面，则使用自定义的
	custom404Path := filepath.Join(d.HTMLDir(), "404.html")
	if io.Exists(custom404Path) {
		notFound, _ = os.ReadFile(custom404Path)
	} else {
		switch app.Locale {
		case "zh_CN":
			notFound, _ = embed.WebsiteFS.ReadFile(filepath.Join("website", "404_zh_CN.html"))
		case "zh_TW":
			notFound, _ = embed.WebsiteFS.ReadFile(filepath.Join("website", "404_zh_TW.html"))
		default:
			notFound, _ = embed.WebsiteFS.ReadFile(filepath.Join("website", "404.html"))
		}
	}

	notFoundPath := filepath.Join(req.Path, "404.html")
	if !io.Exists(notFoundPath) {
		if err = io.Write(notFoundPath, string(notFound), 0644); err != nil {
			return nil, err
		}
	}

	// 写配置
	if err = vhost.SetConfig("001-acme.conf", webservertypes.ScopeSite, ""); err != nil {
		return nil, err
	}

	// 访问统计默认启用
	if d.Features().Stat {
		if err = r.enableStat(d, vhost, req.Name); err != nil {
			return nil, err
		}
	}

	if err = vhost.Save(); err != nil {
		return nil, err
	}

	if err = io.Write(filepath.Join(app.Root, "sites", req.Name, "config", "fullchain.pem"), "", 0600); err != nil {
		return nil, err
	}
	if err = io.Write(filepath.Join(app.Root, "sites", req.Name, "config", "private.key"), "", 0600); err != nil {
		return nil, err
	}

	// 设置目录权限
	// sites/site_name 0755 root
	// sites/site_name/config 0600 root
	// sites/site_name/log 0701 root
	// sites/site_name/public 0755 www
	if err = io.ChmodR(ctx, filepath.Join(app.Root, "sites", req.Name), 0755); err != nil {
		return nil, err
	}
	if err = io.ChmodR(ctx, req.Path, 0755); err != nil {
		return nil, err
	}
	_ = io.ChownR(ctx, req.Path, "www", "www")
	if err = io.ChmodR(ctx, filepath.Join(app.Root, "sites", req.Name, "log"), 0701); err != nil {
		return nil, err
	}
	if err = io.ChmodR(ctx, filepath.Join(app.Root, "sites", req.Name, "config"), 0600); err != nil {
		return nil, err
	}

	// PHP 网站默认开启防跨站
	if req.Type == "php" {
		userIni := filepath.Join(req.Path, ".user.ini")
		if !io.Exists(userIni) {
			if err = io.Write(userIni, fmt.Sprintf("open_basedir=%s:/tmp/", req.Path), 0644); err != nil {
				return nil, err
			}
		}
		_, _ = shell.Execf(ctx, `chattr +i '%s'`, userIni)
	}

	return w, nil
}

func (r *websiteRepo) Update(ctx context.Context, req *request.WebsiteUpdate) (*biz.Website, error) {
	website := new(biz.Website)
	if err := r.db.Where("id", req.ID).First(website).Error; err != nil {
		return nil, err
	}

	if err := r.applyUpdate(ctx, req, website); err != nil {
		return nil, err
	}

	return website, nil
}

func (r *websiteRepo) applyUpdate(ctx context.Context, req *request.WebsiteUpdate, website *biz.Website) error {
	d, err := r.dialect()
	if err != nil {
		return err
	}
	vhost, err := newVhost(d, website)
	if err != nil {
		return err
	}

	// 监听地址
	if req.SSL && !website.SSL {
		listenIPv6, getErr := r.setting.GetBool(biz.SettingKeyWebsiteListenIPv6, false)
		if getErr != nil {
			return getErr
		}
		req.Listens = d.MergeHTTPSListens(req.Listens, listenIPv6)
	}
	// 关闭 HTTPS 时移除 SSL 专用监听（含 IPv6），避免残留 ssl/quic 参数导致 nginx 无法启动
	if !req.SSL {
		req.Listens = lo.Filter(req.Listens, func(l webservertypes.Listen, _ int) bool {
			return !slices.Contains(l.Args, "ssl") && !slices.Contains(l.Args, "quic")
		})
	}
	if err = vhost.SetListen(req.Listens); err != nil {
		return err
	}
	// 域名
	domains, err := punycode.EncodeDomains(req.Domains)
	if err != nil {
		return err
	}
	domains = lo.Map(domains, func(d string, _ int) string { return tools.WrapIPv6(d) })
	if err = r.checkDomains(d, website.Name, domains, req.Listens); err != nil {
		return err
	}
	if err = vhost.SetServerName(domains); err != nil {
		return err
	}
	// 首页文件
	if err = vhost.SetIndex(req.Index); err != nil {
		return err
	}
	// 运行目录
	if !io.Exists(req.Root) {
		return errors.New(r.t.Get("runtime directory does not exist"))
	}
	// .user.ini 的内容由运行目录与网站目录决定，趁两者还是旧值先比较
	dirChanged := req.Root != vhost.Root() || req.Path != website.Path
	if err = vhost.SetRoot(req.Root); err != nil {
		return err
	}
	// 运行目录
	if !io.Exists(req.Path) {
		return errors.New(r.t.Get("website directory does not exist"))
	}
	website.Path = req.Path
	// SSL
	certPath := filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem")
	keyPath := filepath.Join(app.Root, "sites", website.Name, "config", "private.key")
	if err = io.Write(certPath, req.SSLCert, 0600); err != nil {
		return err
	}
	if err = io.Write(keyPath, req.SSLKey, 0600); err != nil {
		return err
	}
	website.SSL = req.SSL
	if req.SSL {
		if _, err = cert.ParseCert([]byte(req.SSLCert)); err != nil {
			return errors.New(r.t.Get("failed to parse certificate: %v", err))
		}
		if _, err = cert.ParseKey([]byte(req.SSLKey)); err != nil {
			return errors.New(r.t.Get("failed to parse private key: %v", err))
		}
		// 检查证书是否已存在于面板的证书管理中，如果不存在则作为本地证书上传
		existing := new(biz.Cert)
		err = r.db.Where("TRIM(cert, char(9) || char(10) || char(13) || ' ') = ?", strings.TrimSpace(req.SSLCert)).First(existing).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			certInfo, _ := cert.ParseCert([]byte(req.SSLCert))
			existing = &biz.Cert{
				Type:    "upload",
				Domains: cert.SANs(certInfo),
				Cert:    strings.TrimSpace(req.SSLCert),
				Key:     strings.TrimSpace(req.SSLKey),
			}
			if err = r.db.Create(existing).Error; err != nil {
				return err
			}
		}
		// 绑定证书，使续签后能自动部署
		website.CertID = existing.ID
		quic := false
		for _, listen := range req.Listens {
			if slices.Contains(listen.Args, "quic") {
				quic = true
				break
			}
		}
		defaultTLSVersions, _ := r.setting.GetSlice(biz.SettingKeyWebsiteTLSVersions)
		if err = vhost.SetSSLConfig(&webservertypes.SSLConfig{
			Cert:         certPath,
			Key:          keyPath,
			Protocols:    lo.If(len(req.SSLProtocols) > 0, req.SSLProtocols).Else(defaultTLSVersions),
			HSTS:         req.HSTS,
			OCSP:         req.OCSP,
			HTTPRedirect: req.HTTPRedirect,
			AltSvc:       lo.If(quic, `h3=":$server_port"; ma=2592000`).Else(``),
		}); err != nil {
			return err
		}
	} else {
		if err = vhost.ClearSSL(); err != nil {
			return err
		}
		// 关闭 HTTPS 后不应再被证书续签覆盖
		website.CertID = 0
	}

	// PHP
	if phpVhost, ok := vhost.(webservertypes.PHPVhost); ok {
		if err = phpVhost.SetPHP(req.PHP); err != nil {
			return err
		}
		// 伪静态
		if err = phpVhost.SetRawConfig("010-rewrite.conf", webservertypes.ScopeSite, req.Rewrite); err != nil {
			return err
		}
		// 防跨站
		if !strings.HasSuffix(req.Root, "/") {
			req.Root += "/"
		}
		userIni := filepath.Join(req.Root, ".user.ini")
		if req.OpenBasedir {
			if content, _ := io.Read(userIni); dirChanged || !strings.Contains(content, "open_basedir") {
				// 之前没有开启，或者修改了目录，重新写入
				if err = io.Write(userIni, fmt.Sprintf("open_basedir=%s:%s:/tmp/", req.Root, req.Path), 0644); err != nil {
					return err
				}
			}
			_, _ = shell.Execf(ctx, `chattr +i '%s'`, userIni)
		} else if io.Exists(userIni) {
			if err = io.Remove(userIni); err != nil {
				return err
			}
		}
	}

	// 反向代理
	if proxyVhost, ok := vhost.(webservertypes.VhostProxy); ok {
		if err = proxyVhost.SetUpstreams(req.Upstreams); err != nil {
			return err
		}
		if err = proxyVhost.SetProxies(req.Proxies); err != nil {
			return err
		}
	}

	// 重定向配置
	if redirectVhost, ok := vhost.(webservertypes.VhostRedirect); ok {
		if err = redirectVhost.SetRedirects(req.Redirects); err != nil {
			return err
		}
	}

	// 高级设置（日志路径、限流限速、真实 IP、基本认证）
	// 日志路径
	if req.AccessLog != "" {
		if err = vhost.SetAccessLog(req.AccessLog); err != nil {
			return err
		}
	}
	if req.ErrorLog != "" {
		if err = vhost.SetErrorLog(req.ErrorLog); err != nil {
			return err
		}
	}
	// 限流限速
	if req.RateLimit != nil {
		if err = vhost.SetRateLimit(req.RateLimit); err != nil {
			return err
		}
	} else {
		if err = vhost.ClearRateLimit(); err != nil {
			return err
		}
	}
	// 真实 IP 配置
	if req.RealIP != nil {
		if err = vhost.SetRealIP(req.RealIP); err != nil {
			return err
		}
	} else {
		if err = vhost.ClearRealIP(); err != nil {
			return err
		}
	}
	// 基本认证：每条规则一个独立的 htpasswd 文件
	r.removeBasicAuthFiles(website.Name)
	auths := make([]webservertypes.BasicAuth, 0, len(req.BasicAuth))
	for i, rule := range req.BasicAuth {
		htpasswdPath := filepath.Join(app.Root, "sites", website.Name, fmt.Sprintf("htpasswd_%d", i))
		if err = r.writeBasicAuthUsers(d, htpasswdPath, rule.Users); err != nil {
			return err
		}
		// 路径归一化：空 -> "/"，去尾部斜杠
		auths = append(auths, webservertypes.BasicAuth{Path: "/" + strings.Trim(rule.Path, "/ "), UserFile: htpasswdPath})
	}
	if err = vhost.SetBasicAuth(auths); err != nil {
		return err
	}

	// 访问统计
	if d.Features().Stat {
		if req.StatEnabled {
			if err = r.enableStat(d, vhost, website.Name); err != nil {
				return err
			}
		} else {
			_ = vhost.RemoveConfig("010-stat-format.conf", webservertypes.ScopeShared)
			_ = vhost.RemoveConfig("021-stats-log.conf", webservertypes.ScopeSite)
		}
	}

	// LiteSpeed 页面缓存
	if d.Features().LSCache {
		if req.LSCache {
			if err = vhost.SetConfig("020-lscache.conf", webservertypes.ScopeSite, d.LSCacheConf(website.Name)); err != nil {
				return err
			}
		} else {
			_ = vhost.RemoveConfig("020-lscache.conf", webservertypes.ScopeSite)
		}
	}

	// 自定义配置
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	if err = r.saveCustomConfigs(configDir, req.CustomConfigs); err != nil {
		return err
	}

	// 保存配置
	if err = vhost.Save(); err != nil {
		return err
	}
	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	return nil
}

func (r *websiteRepo) GetForDelete(id uint) (*biz.Website, error) {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return nil, err
	}
	return website, nil
}

func (r *websiteRepo) RemoveFiles(ctx context.Context, name string, removePath bool) error {
	if removePath {
		_ = io.Remove(filepath.Join(app.Root, "sites", name))
	} else {
		// 仅删除配置和日志
		_ = io.Remove(filepath.Join(app.Root, "sites", name, "config"))
		_ = io.Remove(filepath.Join(app.Root, "sites", name, "log"))
		r.removeBasicAuthFiles(name)
	}
	return nil
}

func (r *websiteRepo) Delete(website *biz.Website) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(website).Error; err != nil {
			return err
		}

		// HTTP 验证依赖网站，证书失去全部网站后无法继续自动续签
		return tx.Model(&biz.Cert{}).
			Where("id = ? AND dns_id = 0", website.CertID).
			Where("NOT EXISTS (SELECT 1 FROM websites WHERE cert_id = ?)", website.CertID).
			Update("auto_renewal", false).Error
	})
}

func (r *websiteRepo) ReloadWebServer(ctx context.Context) error {
	d, err := r.dialect()
	if err != nil {
		return err
	}

	// 调用方都是配置已落盘、只差 reload 的临界区，断开取消链，否则磁盘配置与运行中的配置会不一致
	return d.ReloadIfRunning(context.WithoutCancel(ctx))
}

func (r *websiteRepo) UpdateRemark(id uint, remark string) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return err
	}

	website.Remark = remark
	return r.db.Save(website).Error
}

func (r *websiteRepo) UpdateStatus(ctx context.Context, id uint, status bool) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(&website).Error; err != nil {
		return err
	}

	vhost, err := r.getVhost(website)
	if err != nil {
		return err
	}
	if err = vhost.SetEnable(status); err != nil {
		return err
	}
	if err = vhost.Save(); err != nil {
		return err
	}

	website.Status = status
	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	return r.ReloadWebServer(ctx)
}

func (r *websiteRepo) UpdateExpireAt(id uint, expireAt *time.Time) error {
	return r.db.Model(&biz.Website{}).Where("id = ?", id).Update("expire_at", expireAt).Error
}

func (r *websiteRepo) UpdateCert(ctx context.Context, req *request.WebsiteUpdateCert) error {
	website := new(biz.Website)
	if err := r.db.Where("name", req.Name).First(&website).Error; err != nil {
		return err
	}

	if _, err := cert.ParseCert([]byte(req.Cert)); err != nil {
		return errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err := cert.ParseKey([]byte(req.Key)); err != nil {
		return errors.New(r.t.Get("failed to parse private key: %v", err))
	}

	certPath := filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem")
	keyPath := filepath.Join(app.Root, "sites", website.Name, "config", "private.key")
	if err := io.Write(certPath, req.Cert, 0600); err != nil {
		return err
	}
	if err := io.Write(keyPath, req.Key, 0600); err != nil {
		return err
	}

	if website.SSL {
		return r.ReloadWebServer(ctx)
	}

	return nil
}

func (r *websiteRepo) UpdateProject(id, projectID uint) error {
	return r.db.Model(&biz.Website{}).Where("id = ?", id).Update("project_id", projectID).Error
}

func (r *websiteRepo) UnbindProject(projectID uint) error {
	return r.db.Model(&biz.Website{}).Where("project_id = ?", projectID).Update("project_id", 0).Error
}
