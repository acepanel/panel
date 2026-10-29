package data

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
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
		// 合并 DNSNames 和 IPAddresses
		setting.SSLDNSNames = decode.DNSNames
		for _, ip := range decode.IPAddresses {
			setting.SSLDNSNames = append(setting.SSLDNSNames, ip.String())
		}
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

	// 反向代理网站特有
	if proxyVhost, ok := vhost.(webservertypes.ProxyVhost); ok {
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
	if err = os.MkdirAll(filepath.Join(app.Root, "sites", req.Name, "config", "site"), 0600); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(app.Root, "sites", req.Name, "config", "shared"), 0600); err != nil {
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
	if err = writeTypeConfigs(d, vhost, w.Type); err != nil {
		return nil, err
	}

	// 反向代理支持
	if proxyVhost, ok := vhost.(webservertypes.ProxyVhost); ok {
		if err = proxyVhost.SetProxies([]webservertypes.Proxy{
			{
				Location: "^~ /",
				Pass:     req.Proxy,
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

func (r *websiteRepo) SwitchType(ctx context.Context, req *request.WebsiteSwitchType) (*biz.Website, error) {
	website := new(biz.Website)
	if err := r.db.Where("id", req.ID).First(website).Error; err != nil {
		return nil, err
	}

	targetType := biz.WebsiteType(req.Type)
	if targetType == website.Type {
		return nil, errors.New(r.t.Get("website type is unchanged"))
	}

	setting, err := r.Get(req.ID)
	if err != nil {
		return nil, err
	}
	d, err := r.dialect()
	if err != nil {
		return nil, err
	}

	customConfigs := lo.Map(setting.CustomConfigs, func(config types.WebsiteCustomConfig, _ int) request.WebsiteCustomConfig {
		return request.WebsiteCustomConfig{
			Name:    config.Name,
			Scope:   config.Scope,
			Content: config.Content,
		}
	})

	update := &request.WebsiteUpdate{
		ID:            website.ID,
		Listens:       setting.Listens,
		Domains:       setting.Domains,
		Path:          setting.Path,
		Root:          setting.Root,
		Index:         []string{"index.html"},
		SSL:           setting.SSL,
		SSLCert:       setting.SSLCert,
		SSLKey:        setting.SSLKey,
		HSTS:          setting.HSTS,
		OCSP:          setting.OCSP,
		HTTPRedirect:  setting.HTTPRedirect,
		SSLProtocols:  setting.SSLProtocols,
		Redirects:     setting.Redirects,
		StatEnabled:   setting.StatEnabled,
		AccessLog:     setting.AccessLog,
		ErrorLog:      setting.ErrorLog,
		RateLimit:     setting.RateLimit,
		RealIP:        setting.RealIP,
		BasicAuth:     setting.BasicAuth,
		CustomConfigs: customConfigs,
	}
	switch targetType {
	case biz.WebsiteTypePHP:
		update.Index = []string{"index.php", "index.html"}
		update.PHP = req.PHP
		update.OpenBasedir = true
	case biz.WebsiteTypeProxy:
		update.Proxies = []webservertypes.Proxy{{
			Location: "^~ /",
			Pass:     req.Proxy,
		}}
	default:
		// 静态站点沿用默认配置
	}

	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	backupDir := fmt.Sprintf("%s.switch-backup-%d", configDir, time.Now().UnixNano())
	if err = os.Rename(configDir, backupDir); err != nil {
		return nil, err
	}
	// 回滚需还原备份的配置目录，被取消会把站点配置留在中间态
	restoreCtx := context.WithoutCancel(ctx)
	if err = os.MkdirAll(filepath.Join(configDir, "site"), 0600); err != nil {
		_ = io.Remove(configDir)
		_ = os.Rename(backupDir, configDir)
		return nil, err
	}
	if err = os.MkdirAll(filepath.Join(configDir, "shared"), 0600); err != nil {
		_ = io.Remove(configDir)
		_ = os.Rename(backupDir, configDir)
		return nil, err
	}

	oldType := website.Type
	oldPath := website.Path
	oldSSL := website.SSL
	oldUpdatedAt := website.UpdatedAt
	userIniPath := filepath.Join(setting.Root, ".user.ini")
	oldUserIni, userIniErr := os.ReadFile(userIniPath)
	if userIniErr != nil && !os.IsNotExist(userIniErr) {
		_ = io.Remove(configDir)
		_ = os.Rename(backupDir, configDir)
		return nil, userIniErr
	}
	oldUserIniExists := userIniErr == nil
	databaseUpdated := false
	restore := func(switchErr error) (*biz.Website, error) {
		var restoreErr error
		restoreErr = errors.Join(restoreErr, io.Remove(configDir))
		restoreErr = errors.Join(restoreErr, os.Rename(backupDir, configDir))
		if databaseUpdated {
			restoreErr = errors.Join(restoreErr, r.db.Model(&biz.Website{}).Where("id = ?", website.ID).UpdateColumns(map[string]any{
				"type":       oldType,
				"path":       oldPath,
				"ssl":        oldSSL,
				"updated_at": oldUpdatedAt,
			}).Error)
		}
		restoreErr = errors.Join(restoreErr, io.Remove(userIniPath))
		if oldUserIniExists {
			restoreErr = errors.Join(restoreErr, io.Write(userIniPath, string(oldUserIni), 0644))
			if setting.OpenBasedir {
				_, attrErr := shell.Execf(restoreCtx, `chattr +i '%s'`, userIniPath)
				restoreErr = errors.Join(restoreErr, attrErr)
			}
		}
		return nil, errors.Join(switchErr, restoreErr)
	}

	website.Type = targetType
	if err = r.applyUpdate(ctx, update, website); err != nil {
		return restore(err)
	}
	databaseUpdated = true

	vhost, err := newVhost(d, website)
	if err != nil {
		return restore(err)
	}
	if err = vhost.SetConfig("001-acme.conf", webservertypes.ScopeSite, ""); err != nil {
		return restore(err)
	}
	if err = writeTypeConfigs(d, vhost, targetType); err != nil {
		return restore(err)
	}
	if err = vhost.SetEnable(website.Status); err != nil {
		return restore(err)
	}
	if err = vhost.Save(); err != nil {
		return restore(err)
	}

	if oldType == biz.WebsiteTypePHP && targetType != biz.WebsiteTypePHP && setting.OpenBasedir {
		if err = io.Remove(userIniPath); err != nil {
			return restore(err)
		}
	}
	if err = r.ReloadWebServer(ctx); err != nil {
		_, switchErr := restore(err)
		if reloadErr := r.ReloadWebServer(restoreCtx); reloadErr != nil {
			switchErr = errors.Join(switchErr, reloadErr)
		}
		return nil, switchErr
	}

	_ = io.Remove(backupDir)
	return website, nil
}

// Rebuild 用上一个 Web 服务器留下的配置为当前 Web 服务器重建站点配置，返回是否重建及需人工处理的提示
func (r *websiteRepo) Rebuild(ctx context.Context, website *biz.Website) (bool, []string, error) {
	d, err := r.dialect()
	if err != nil {
		return false, nil, err
	}
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	source, ok := sourceDialect(configDir)
	if !ok || source.Type == d.Type {
		return false, nil, nil
	}

	oldVhost, err := newVhost(source, website)
	if err != nil {
		return false, nil, err
	}
	setting, err := r.loadSetting(website, oldVhost)
	if err != nil {
		return false, nil, err
	}

	// 目标不支持的 IPv6 监听直接丢弃
	listens := setting.Listens
	if !d.Features().IPv6Listen {
		listens = lo.Filter(listens, func(listen webservertypes.Listen, _ int) bool {
			return !strings.HasPrefix(listen.Address, "[")
		})
	}
	if len(listens) == 0 {
		listens = []webservertypes.Listen{{Address: "80"}}
	}
	customProxy := false
	for i := range setting.Proxies {
		customProxy = customProxy || setting.Proxies[i].Custom != ""
		setting.Proxies[i].Custom = ""
	}
	index := setting.Index
	if len(index) == 0 {
		index = []string{"index.html"}
		if website.Type == biz.WebsiteTypePHP {
			index = []string{"index.php", "index.html"}
		}
	}
	update := &request.WebsiteUpdate{
		ID:           website.ID,
		Listens:      listens,
		Domains:      setting.Domains,
		Path:         setting.Path,
		Root:         setting.Root,
		Index:        index,
		SSL:          setting.SSL,
		SSLCert:      setting.SSLCert,
		SSLKey:       setting.SSLKey,
		HSTS:         setting.HSTS,
		OCSP:         setting.OCSP,
		HTTPRedirect: setting.HTTPRedirect,
		SSLProtocols: setting.SSLProtocols,
		PHP:          setting.PHP,
		OpenBasedir:  setting.OpenBasedir,
		Upstreams:    setting.Upstreams,
		Proxies:      setting.Proxies,
		Redirects:    setting.Redirects,
		BasicAuth:    setting.BasicAuth,
		StatEnabled:  d.Features().Stat,
	}

	// 只迁移各服务器通用的设置，伪静态、自定义配置（含代理规则的自定义指令）、限流与真实 IP 是各自的语法或语义，直接丢弃
	var notes []string
	if setting.Rewrite != "" || len(setting.CustomConfigs) > 0 || customProxy || setting.RateLimit != nil || setting.RealIP != nil {
		notes = append(notes, r.t.Get("rewrite rules, custom configs, rate limit and real IP settings are not migrated"))
	}

	backupDir := fmt.Sprintf("%s.rebuild-backup-%d", configDir, time.Now().UnixNano())
	if err = os.Rename(configDir, backupDir); err != nil {
		return false, nil, err
	}
	restore := func(rebuildErr error) (bool, []string, error) {
		removeErr := io.Remove(configDir)
		return false, nil, errors.Join(rebuildErr, removeErr, os.Rename(backupDir, configDir))
	}
	for _, scope := range []string{"site", "shared"} {
		if err = os.MkdirAll(filepath.Join(configDir, scope), 0600); err != nil {
			return restore(err)
		}
	}
	if err = r.applyUpdate(ctx, update, website); err != nil {
		return restore(err)
	}
	vhost, err := newVhost(d, website)
	if err != nil {
		return restore(err)
	}
	if err = vhost.SetConfig("001-acme.conf", webservertypes.ScopeSite, ""); err != nil {
		return restore(err)
	}
	if err = writeTypeConfigs(d, vhost, website.Type); err != nil {
		return restore(err)
	}
	if err = vhost.SetEnable(website.Status); err != nil {
		return restore(err)
	}
	if err = vhost.Save(); err != nil {
		return restore(err)
	}

	_ = io.Remove(backupDir)
	return true, notes, nil
}

// sourceDialect 按站点目录里最新修改的主配置文件识别上一个 Web 服务器
func sourceDialect(configDir string) (webserver.Dialect, bool) {
	var latest webserver.Dialect
	var modTime time.Time
	for _, t := range webserver.Types() {
		d, err := webserver.Get(t)
		if err != nil {
			continue
		}
		info, err := os.Stat(filepath.Join(configDir, d.ConfigFile()))
		if err != nil || (!modTime.IsZero() && !info.ModTime().After(modTime)) {
			continue
		}
		latest, modTime = d, info.ModTime()
	}

	return latest, !modTime.IsZero()
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
			sans := certInfo.DNSNames
			for _, ip := range certInfo.IPAddresses {
				sans = append(sans, ip.String())
			}
			existing = &biz.Cert{
				Type:    "upload",
				Domains: sans,
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
			if !io.Exists(userIni) || req.Root != website.Path {
				// 之前没有开启，或者修改了运行目录，重新写入
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
	if proxyVhost, ok := vhost.(webservertypes.ProxyVhost); ok {
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
		if err = writeBasicAuthUsers(d, htpasswdPath, rule.Users); err != nil {
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

func (r *websiteRepo) UpdateRemark(id uint, remark string) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return err
	}

	website.Remark = remark
	return r.db.Save(website).Error
}

func (r *websiteRepo) UpdateProject(id, projectID uint) error {
	return r.db.Model(&biz.Website{}).Where("id = ?", id).Update("project_id", projectID).Error
}

func (r *websiteRepo) UnbindProject(projectID uint) error {
	return r.db.Model(&biz.Website{}).Where("project_id = ?", projectID).Update("project_id", 0).Error
}

func (r *websiteRepo) ResetConfig(ctx context.Context, id uint) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(&website).Error; err != nil {
		return err
	}

	d, err := r.dialect()
	if err != nil {
		return err
	}

	setting, err := r.Get(id)
	if err != nil {
		return err
	}
	listens := lo.Filter(setting.Listens, func(listen webservertypes.Listen, _ int) bool {
		if slices.Contains(listen.Args, "ssl") || slices.Contains(listen.Args, "quic") {
			return false
		}
		port := listen.Address
		if strings.Contains(listen.Address, ":") {
			if _, parsedPort, splitErr := net.SplitHostPort(listen.Address); splitErr == nil {
				port = parsedPort
			}
		}
		return !website.SSL || port != "443"
	})
	if len(listens) == 0 {
		listens = []webservertypes.Listen{{Address: "80"}}
	}

	update := &request.WebsiteUpdate{
		ID:          website.ID,
		Listens:     listens,
		Domains:     setting.Domains,
		Path:        website.Path,
		Root:        setting.Root,
		Index:       []string{"index.html"},
		SSL:         false,
		StatEnabled: d.Features().Stat,
		AccessLog:   filepath.Join(app.Root, "sites", website.Name, "log", "access.log"),
		ErrorLog:    filepath.Join(app.Root, "sites", website.Name, "log", "error.log"),
	}
	switch website.Type {
	case biz.WebsiteTypePHP:
		update.Index = []string{"index.php", "index.html"}
		update.PHP = setting.PHP
		update.OpenBasedir = true
	case biz.WebsiteTypeProxy:
		update.Upstreams = setting.Upstreams
		if len(setting.Proxies) > 0 && setting.Proxies[0].Pass != "" {
			update.Proxies = []webservertypes.Proxy{{
				Location: "^~ /",
				Pass:     setting.Proxies[0].Pass,
			}}
		}
	default:
	}

	// 配置目录是删掉重建的，中途取消会让站点配置凭空消失且无处可恢复，这段不可取消
	resetCtx := context.WithoutCancel(ctx)
	if website.Type == biz.WebsiteTypePHP {
		if err = io.Remove(filepath.Join(setting.Root, ".user.ini")); err != nil {
			return err
		}
	}
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	backupDir := fmt.Sprintf("%s.reset-backup-%d", configDir, time.Now().UnixNano())
	if err = os.Rename(configDir, backupDir); err != nil {
		return err
	}
	restore := func(resetErr error) error {
		return errors.Join(resetErr, io.Remove(configDir), os.Rename(backupDir, configDir))
	}
	for _, scope := range []string{"site", "shared"} {
		if err = os.MkdirAll(filepath.Join(configDir, scope), 0600); err != nil {
			return restore(err)
		}
	}

	website.Status = true
	if err = r.applyUpdate(resetCtx, update, website); err != nil {
		return restore(err)
	}

	vhost, err := newVhost(d, website)
	if err != nil {
		return restore(err)
	}
	if err = vhost.SetConfig("001-acme.conf", webservertypes.ScopeSite, ""); err != nil {
		return restore(err)
	}
	if err = writeTypeConfigs(d, vhost, website.Type); err != nil {
		return restore(err)
	}
	if err = vhost.Save(); err != nil {
		return restore(err)
	}
	if err = io.ChmodR(resetCtx, configDir, 0600); err != nil {
		return restore(err)
	}
	_ = io.Remove(backupDir)

	return r.ReloadWebServer(resetCtx)
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

// customConfigStartNum 自定义配置起始序号
const customConfigStartNum = 800

// customConfigEndNum 自定义配置结束序号
const customConfigEndNum = 999

// getCustomConfigs 获取网站自定义配置列表
func (r *websiteRepo) getCustomConfigs(configDir string) []types.WebsiteCustomConfig {
	var configs []types.WebsiteCustomConfig

	// 从 site 和 shared 目录读取自定义配置
	for _, scope := range []string{"site", "shared"} {
		scopeDir := filepath.Join(configDir, scope)
		entries, err := os.ReadDir(scopeDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			// 匹配文件名格式: 800-999-name.conf
			name := entry.Name()
			if !strings.HasSuffix(name, ".conf") {
				continue
			}
			// 解析序号
			parts := strings.SplitN(name, "-", 2)
			if len(parts) < 2 {
				continue
			}
			num, err := strconv.Atoi(parts[0])
			if err != nil || num < customConfigStartNum || num > customConfigEndNum {
				continue
			}
			// 提取配置名称（去掉序号前缀和.conf后缀）
			configName := strings.TrimSuffix(parts[1], ".conf")
			if configName == "" {
				continue
			}
			// 读取配置内容
			content, err := io.Read(filepath.Join(scopeDir, name))
			if err != nil {
				continue
			}

			configs = append(configs, types.WebsiteCustomConfig{
				Name:    configName,
				Scope:   scope,
				Content: content,
			})
		}
	}

	return configs
}

// saveCustomConfigs 保存网站自定义配置
func (r *websiteRepo) saveCustomConfigs(configDir string, configs []request.WebsiteCustomConfig) error {
	if err := r.clearCustomConfigs(configDir); err != nil {
		return err
	}

	// 分别跟踪 site 和 shared 目录的序号
	siteNum := customConfigStartNum
	sharedNum := customConfigStartNum

	for _, cfg := range configs {
		var num int
		switch cfg.Scope {
		case "site":
			num = siteNum
			siteNum++
		case "shared":
			num = sharedNum
			sharedNum++
		default:
			return fmt.Errorf("invalid config scope: %s", cfg.Scope)
		}

		if num > customConfigEndNum {
			return errors.New(r.t.Get("maximum number of custom configurations reached (limit: %d)", customConfigEndNum-customConfigStartNum+1))
		}

		fileName := fmt.Sprintf("%03d-%s.conf", num, cfg.Name)
		filePath := filepath.Join(configDir, cfg.Scope, fileName)

		if err := io.Write(filePath, cfg.Content, 0600); err != nil {
			return fmt.Errorf("failed to write custom config: %w", err)
		}
	}

	return nil
}

// clearCustomConfigs 清除网站自定义配置文件
func (r *websiteRepo) clearCustomConfigs(configDir string) error {
	for _, scope := range []string{"site", "shared"} {
		scopeDir := filepath.Join(configDir, scope)
		entries, err := os.ReadDir(scopeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".conf") {
				continue
			}
			parts := strings.SplitN(name, "-", 2)
			if len(parts) < 2 {
				continue
			}
			num, err := strconv.Atoi(parts[0])
			if err != nil || num < customConfigStartNum || num > customConfigEndNum {
				continue
			}
			filePath := filepath.Join(scopeDir, name)
			if err = os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove custom config: %w", err)
			}
		}
	}

	return nil
}

// dialect 取当前 Web 服务器方言
func (r *websiteRepo) dialect() (webserver.Dialect, error) {
	webServer, err := r.setting.Get(biz.SettingKeyWebserver)
	if err != nil {
		return webserver.Dialect{}, err
	}

	return webserver.Get(webserver.Type(webServer))
}

// listenPorts 取监听端口，忽略 `80`、`0.0.0.0:80`、`[::]:80` 的地址形式差异
func listenPorts(listens []webservertypes.Listen) []string {
	return lo.Uniq(lo.Map(listens, func(listen webservertypes.Listen, _ int) string {
		if _, port, err := net.SplitHostPort(listen.Address); err == nil {
			return port
		}
		return listen.Address
	}))
}

// newVhost 按网站类型构造站点 vhost
func newVhost(d webserver.Dialect, website *biz.Website) (webservertypes.Vhost, error) {
	return d.NewVhost(string(website.Type), filepath.Join(app.Root, "sites", website.Name, "config"))
}

func (r *websiteRepo) getVhost(website *biz.Website) (webservertypes.Vhost, error) {
	d, err := r.dialect()
	if err != nil {
		return nil, err
	}

	return newVhost(d, website)
}

// writeTypeConfigs 写入站点类型对应的默认片段：404 页面、PHP 缓存或静态 SPA 回退
func writeTypeConfigs(d webserver.Dialect, vhost webservertypes.Vhost, typ biz.WebsiteType) error {
	if err := vhost.SetConfig("010-error-404.conf", webservertypes.ScopeSite, d.ErrorPageConf()); err != nil {
		return err
	}
	switch typ {
	case biz.WebsiteTypePHP:
		return vhost.SetConfig("010-cache.conf", webservertypes.ScopeSite, d.PHPCacheConf())
	case biz.WebsiteTypeStatic:
		return vhost.SetRawConfig("800-spa.conf", webservertypes.ScopeSite, d.SPAConf())
	default:
		// 反代站点无需额外片段
	}

	return nil
}

func (r *websiteRepo) ReloadWebServer(ctx context.Context) error {
	d, err := r.dialect()
	if err != nil {
		return err
	}

	// 调用方都是配置已落盘、只差 reload 的临界区，断开取消链，否则磁盘配置与运行中的配置会不一致
	return d.ReloadIfRunning(context.WithoutCancel(ctx))
}

// readBasicAuthUsers 读取 htpasswd 文件中的用户列表
func (r *websiteRepo) readBasicAuthUsers(htpasswdPath string) map[string]string {
	if !io.Exists(htpasswdPath) {
		return make(map[string]string)
	}

	file, err := os.Open(htpasswdPath)
	if err != nil {
		return make(map[string]string)
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	users := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// htpasswd 格式: username:{PLAIN}password或直接username:password
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			users[parts[0]] = strings.TrimPrefix(parts[1], "{PLAIN}")
		}
	}
	if scanner.Err() != nil {
		return make(map[string]string)
	}

	return users
}

// removeBasicAuthFiles 删除站点的所有 htpasswd 文件
func (r *websiteRepo) removeBasicAuthFiles(siteName string) {
	matches, _ := filepath.Glob(filepath.Join(app.Root, "sites", siteName, "htpasswd*"))
	for _, match := range matches {
		_ = io.Remove(match)
	}
}

// writeBasicAuthUsers 将用户凭证写入 htpasswd 文件
func writeBasicAuthUsers(d webserver.Dialect, htpasswdPath string, users map[string]string) error {
	var lines []string
	for username, password := range users {
		if username == "" || password == "" {
			continue
		}
		lines = append(lines, d.HTPasswdLine(username, password))
	}

	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	return io.Write(htpasswdPath, content, 0644) // 必须 0644，Nginx 在运行中以 www 用户读取
}

// enableStat 共享级片段只有部分服务器需要
func (r *websiteRepo) enableStat(d webserver.Dialect, vhost webservertypes.Vhost, name string) error {
	shared, site := d.StatConf(name)
	if shared != "" {
		if err := vhost.SetConfig("010-stat-format.conf", webservertypes.ScopeShared, shared); err != nil {
			return err
		}
	}
	return vhost.SetConfig("021-stats-log.conf", webservertypes.ScopeSite, site)
}

// checkDomains 域名与监听端口同时重叠才算被占用：nginx 只是警告，Caddy 会整份配置拒载
func (r *websiteRepo) checkDomains(d webserver.Dialect, exclude string, domains []string, listens []webservertypes.Listen) error {
	ports := listenPorts(listens)
	var websites []*biz.Website
	if err := r.db.Find(&websites).Error; err != nil {
		return err
	}
	for _, website := range websites {
		// 切换服务器重建途中，尚未重建的站点没有当前方言的配置，跳过以免拿默认域名误判
		if website.Name == exclude || !io.Exists(filepath.Join(app.Root, "sites", website.Name, "config", d.ConfigFile())) {
			continue
		}
		vhost, err := newVhost(d, website)
		if err != nil || !lo.Some(listenPorts(vhost.Listen()), ports) {
			continue
		}
		for _, domain := range vhost.ServerName() {
			if slices.ContainsFunc(domains, func(d string) bool { return strings.EqualFold(d, domain) }) {
				return errors.New(r.t.Get("domain %s is already used by website %s", domain, website.Name))
			}
		}
	}
	return nil
}
