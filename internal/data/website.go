package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/acme"
	"github.com/acepanel/panel/pkg/api"
	"github.com/acepanel/panel/pkg/cert"
	"github.com/acepanel/panel/pkg/embed"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/punycode"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/types"
	"github.com/acepanel/panel/pkg/webserver"
	webservertypes "github.com/acepanel/panel/pkg/webserver/types"
)

type websiteRepo struct {
	t              *gotext.Locale
	db             *gorm.DB
	log            *slog.Logger
	cache          biz.CacheRepo
	database       biz.DatabaseRepo
	databaseServer biz.DatabaseServerRepo
	databaseUser   biz.DatabaseUserRepo
	cert           biz.CertRepo
	certAccount    biz.CertAccountRepo
	setting        biz.SettingRepo
}

func NewWebsiteRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger, cache biz.CacheRepo, database biz.DatabaseRepo, databaseServer biz.DatabaseServerRepo, databaseUser biz.DatabaseUserRepo, cert biz.CertRepo, certAccount biz.CertAccountRepo, setting biz.SettingRepo) biz.WebsiteRepo {
	return &websiteRepo{
		t:              t,
		db:             db,
		log:            log,
		cache:          cache,
		database:       database,
		databaseServer: databaseServer,
		databaseUser:   databaseUser,
		cert:           cert,
		certAccount:    certAccount,
		setting:        setting,
	}
}

func (r *websiteRepo) GetRewrites() (map[string]string, error) {
	cached, err := r.cache.Get(biz.CacheKeyRewrites)
	if err != nil {
		return nil, err
	}

	var rewrites api.Rewrites
	if err = json.Unmarshal([]byte(cached), &rewrites); err != nil {
		return nil, err
	}

	rw := make(map[string]string)
	for rewrite := range slices.Values(rewrites) {
		rw[rewrite.Name] = rewrite.Content
	}

	return rw, nil
}

func (r *websiteRepo) UpdateDefaultConfig(req *request.WebsiteDefaultConfig) error {
	if err := io.Write(filepath.Join(app.Root, "server/nginx/html/index.html"), req.Index, 0644); err != nil {
		return err
	}
	if err := io.Write(filepath.Join(app.Root, "server/nginx/html/stop.html"), req.Stop, 0644); err != nil {
		return err
	}
	if req.NotFound != "" {
		if err := io.Write(filepath.Join(app.Root, "server/nginx/html/404.html"), req.NotFound, 0644); err != nil {
			return err
		}
	}
	if err := r.setting.SetSlice(biz.SettingKeyWebsiteTLSVersions, req.TLSVersions); err != nil {
		return err
	}
	if err := r.setting.Set(biz.SettingKeyWebsiteCipherSuites, req.CipherSuites); err != nil {
		return err
	}

	return r.reloadWebServer()
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
	// SSL
	if setting.SSL {
		sslConfig := vhost.SSLConfig()
		setting.HTTPRedirect = sslConfig.HTTPRedirect
		setting.HSTS = sslConfig.HSTS
		setting.OCSP = sslConfig.OCSP
		setting.SSLProtocols = sslConfig.Protocols
		setting.SSLCiphers = sslConfig.Ciphers
	}
	// 证书
	crt, _ := io.Read(filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem"))
	setting.SSLCert = crt
	key, _ := io.Read(filepath.Join(app.Root, "sites", website.Name, "config", "private.key"))
	setting.SSLKey = key
	// 解析证书信息
	if decode, err := cert.ParseCert(crt); err == nil {
		setting.SSLNotBefore = decode.NotBefore.Format(time.DateTime)
		setting.SSLNotAfter = decode.NotAfter.Format(time.DateTime)
		setting.SSLIssuer = decode.Issuer.CommonName
		setting.SSLOCSPServer = decode.OCSPServer
		setting.SSLDNSNames = decode.DNSNames
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
		setting.Rewrite = phpVhost.Config("010-rewrite.conf", "site")
	}

	// 反向代理网站特有
	if proxyVhost, ok := vhost.(webservertypes.ProxyVhost); ok {
		setting.Upstreams = proxyVhost.Upstreams()
		setting.Proxies = proxyVhost.Proxies()
	}

	return setting, err
}

func (r *websiteRepo) GetByName(name string) (*types.WebsiteSetting, error) {
	website := new(biz.Website)
	if err := r.db.Where("name", name).First(website).Error; err != nil {
		return nil, err
	}

	return r.Get(website.ID)
}

func (r *websiteRepo) List(typ string, page, limit uint) ([]*biz.Website, int64, error) {
	websites := make([]*biz.Website, 0)
	var total int64

	if err := r.db.Model(&biz.Website{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query := r.db
	if typ != "all" {
		query = query.Where("type = ?", typ)
	}
	if err := query.Order("id DESC").Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&websites).Error; err != nil {
		return nil, 0, err
	}

	// 取证书剩余有效时间和PHP版本
	for _, website := range websites {
		crt, _ := io.Read(filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem"))
		if decode, err := cert.ParseCert(crt); err == nil {
			hours := time.Until(decode.NotAfter).Hours()
			website.CertExpire = fmt.Sprintf("%.2f", hours/24)
		}
		if website.Type == biz.WebsiteTypePHP {
			website.PHP = r.getPHPVersion(website.Name)
		}
	}

	return websites, total, nil
}

func (r *websiteRepo) Create(req *request.WebsiteCreate) (*biz.Website, error) {
	w := &biz.Website{
		Name:   req.Name,
		Type:   biz.WebsiteType(req.Type),
		Status: true,
		Path:   req.Path,
		SSL:    false,
		Remark: req.Remark,
	}

	vhost, err := r.getVhost(w)
	if err != nil {
		return nil, err
	}

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
	var listens []webservertypes.Listen
	for _, listen := range req.Listens {
		listens = append(listens, webservertypes.Listen{Address: listen})
	}
	if err = vhost.SetListen(listens); err != nil {
		return nil, err
	}
	// 域名
	domains, err := punycode.EncodeDomains(req.Domains)
	if err != nil {
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
	// 404 页面
	// TODO 需要兼容 Apache
	if err = vhost.SetConfig("010-error-404.conf", "site", `error_page 404 /404.html;`); err != nil {
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
		if err = phpVhost.SetConfig("010-rewrite.conf", "site", ""); err != nil {
			return nil, err
		}
		// TODO 需要兼容 Apache
		if err = phpVhost.SetConfig("010-cache.conf", "site", `# browser cache
location ~ .*\.(bmp|jpg|jpeg|png|gif|svg|ico|tiff|webp|avif|heif|heic|jxl)$ {
    expires 30d;
    access_log /dev/null;
    error_log /dev/null;
}
location ~ .*\.(js|css|ttf|otf|woff|woff2|eot)$ {
    expires 6h;
    access_log /dev/null;
    error_log /dev/null;
}
# deny sensitive files
location ~ ^/(\.user.ini|\.htaccess|\.git|\.svn|\.env) {
    return 404;
}
`); err != nil {
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
	if err = io.Write(filepath.Join(req.Path, "index.html"), string(index), 0644); err != nil {
		return nil, err
	}
	var notFound []byte

	// 如果存在自定义 404 页面，则使用自定义的
	// TODO 需要兼容 Apache
	if io.Exists(filepath.Join(app.Root, "server/nginx/html/404.html")) {
		notFound, _ = os.ReadFile(filepath.Join(app.Root, "server/nginx/html/404.html"))
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

	if err = io.Write(filepath.Join(req.Path, "404.html"), string(notFound), 0644); err != nil {
		return nil, err
	}

	// 写配置
	if err = vhost.SetConfig("001-acme.conf", "site", ""); err != nil {
		return nil, err
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
	if err = io.Chmod(filepath.Join(app.Root, "sites", req.Name), 0755); err != nil {
		return nil, err
	}
	if err = io.Chmod(req.Path, 0755); err != nil {
		return nil, err
	}
	if err = io.Chown(req.Path, "www", "www"); err != nil {
		return nil, err
	}
	if err = io.Chmod(filepath.Join(app.Root, "sites", req.Name, "log"), 0701); err != nil {
		return nil, err
	}
	if err = io.Chmod(filepath.Join(app.Root, "sites", req.Name, "config"), 0600); err != nil {
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
		_, _ = shell.Execf(`chattr +i '%s'`, userIni)
	}

	// 创建面板网站
	if err = r.db.Create(w).Error; err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("website created", slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", 0), slog.String("name", req.Name), slog.String("website_type", req.Type), slog.String("path", req.Path))

	// 重载 Web 服务器
	if err = r.reloadWebServer(); err != nil {
		return nil, err
	}

	// 创建数据库
	name := "local_" + req.DBType
	if req.DB {
		server, err := r.databaseServer.GetByName(name)
		if err != nil {
			return nil, errors.New(r.t.Get("can't find %s database server, please add it first", name))
		}
		if err = r.database.Create(&request.DatabaseCreate{
			ServerID:   server.ID,
			Name:       req.DBName,
			CreateUser: true,
			Username:   req.DBUser,
			Password:   req.DBPassword,
			Host:       "localhost",
			Comment:    fmt.Sprintf("website %s", req.Name),
		}); err != nil {
			return nil, err
		}
	}

	return w, nil
}

func (r *websiteRepo) Update(req *request.WebsiteUpdate) error {
	website := new(biz.Website)
	if err := r.db.Where("id", req.ID).First(website).Error; err != nil {
		return err
	}

	vhost, err := r.getVhost(website)
	if err != nil {
		return err
	}

	// 监听地址
	if err = vhost.SetListen(req.Listens); err != nil {
		return err
	}
	// 域名
	domains, err := punycode.EncodeDomains(req.Domains)
	if err != nil {
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
		if _, err = cert.ParseCert(req.SSLCert); err != nil {
			return errors.New(r.t.Get("failed to parse certificate: %v", err))
		}
		if _, err = cert.ParseKey(req.SSLKey); err != nil {
			return errors.New(r.t.Get("failed to parse private key: %v", err))
		}
		quic := false
		for _, listen := range req.Listens {
			if slices.Contains(listen.Args, "quic") {
				quic = true
				break
			}
		}
		defaultTLSVersions, _ := r.setting.GetSlice(biz.SettingKeyWebsiteTLSVersions)
		defaultCipherSuites, _ := r.setting.Get(biz.SettingKeyWebsiteCipherSuites)
		if err = vhost.SetSSLConfig(&webservertypes.SSLConfig{
			Cert:         certPath,
			Key:          keyPath,
			Protocols:    lo.If(len(req.SSLProtocols) > 0, req.SSLProtocols).Else(defaultTLSVersions),
			Ciphers:      lo.If(req.SSLCiphers != "", req.SSLCiphers).Else(defaultCipherSuites),
			HSTS:         req.HSTS,
			OCSP:         req.OCSP,
			HTTPRedirect: req.HTTPRedirect,
			AltSvc:       lo.If(quic, `'h3=":$server_port"; ma=2592000'`).Else(``),
		}); err != nil {
			return err
		}
	} else {
		if err = vhost.ClearSSL(); err != nil {
			return err
		}
	}

	// PHP
	if phpVhost, ok := vhost.(webservertypes.PHPVhost); ok {
		if err = phpVhost.SetPHP(req.PHP); err != nil {
			return err
		}
		// 伪静态
		if err = phpVhost.SetConfig("010-rewrite.conf", "site", req.Rewrite); err != nil {
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
			_, _ = shell.Execf(`chattr +i '%s'`, userIni)
		} else {
			if io.Exists(userIni) {
				if err = io.Remove(userIni); err != nil {
					return err
				}
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

	// 保存配置
	if err = vhost.Save(); err != nil {
		return err
	}
	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("website updated", slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(req.ID)), slog.String("name", website.Name))

	return r.reloadWebServer()
}

func (r *websiteRepo) Delete(req *request.WebsiteDelete) error {
	website := new(biz.Website)
	if err := r.db.Preload("Cert").Where("id", req.ID).First(website).Error; err != nil {
		return err
	}
	if website.Cert != nil {
		return errors.New(r.t.Get("website %s has bound certificates, please delete the certificate first", website.Name))
	}

	_ = io.Remove(filepath.Join(app.Root, "sites", website.Name))

	if req.Path {
		_ = io.Remove(website.Path)
	}
	if req.DB {
		if mysql, err := r.databaseServer.GetByName("local_mysql"); err == nil {
			_ = r.databaseUser.DeleteByNames(mysql.ID, []string{website.Name})
			_ = r.database.Delete(mysql.ID, website.Name)
		}
		if postgres, err := r.databaseServer.GetByName("local_postgresql"); err == nil {
			_ = r.databaseUser.DeleteByNames(postgres.ID, []string{website.Name})
			_ = r.database.Delete(postgres.ID, website.Name)
		}
	}

	if err := r.db.Delete(website).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("website deleted", slog.String("type", biz.OperationTypeWebsite), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(req.ID)), slog.String("name", website.Name))

	return r.reloadWebServer()
}

func (r *websiteRepo) ClearLog(id uint) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return err
	}

	_, err := shell.Execf(`cat /dev/null > %s/sites/%s/log/access.log`, app.Root, website.Name)
	return err
}

func (r *websiteRepo) UpdateRemark(id uint, remark string) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return err
	}

	website.Remark = remark
	return r.db.Save(website).Error
}

func (r *websiteRepo) ResetConfig(id uint) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(&website).Error; err != nil {
		return err
	}

	// 清空配置
	_, err := shell.Execf(`rm -rf '%s'`, fmt.Sprintf("%s/sites/%s/config/*", app.Root, website.Name))
	if err != nil {
		return err
	}
	// 初始化配置
	vhost, err := r.getVhost(website)
	if err != nil {
		return err
	}
	// 重置配置
	if err = vhost.Reset(); err != nil {
		return err
	}
	// 运行目录
	if err = vhost.SetRoot(website.Path); err != nil {
		return err
	}
	// 日志
	if err = vhost.SetAccessLog(filepath.Join(app.Root, "sites", website.Name, "log", "access.log")); err != nil {
		return err
	}
	if err = vhost.SetErrorLog(filepath.Join(app.Root, "sites", website.Name, "log", "error.log")); err != nil {
		return err
	}
	// 保存配置
	if err = vhost.SetConfig("001-acme.conf", "site", ""); err != nil {
		return err
	}
	if err = vhost.Save(); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(app.Root, "sites", website.Name, "config", "fullchain.pem"), "", 0600); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(app.Root, "sites", website.Name, "config", "private.key"), "", 0600); err != nil {
		return err
	}
	// PHP 网站默认伪静态
	if website.Type == biz.WebsiteTypePHP {
		if err = io.Write(filepath.Join(app.Root, "sites", website.Name, "config", "site", "010-rewrite.conf"), "", 0600); err != nil {
			return err
		}
	}

	// 设置目录权限
	if err = io.Chown(website.Path, "root", "root"); err != nil {
		return err
	}
	if err = io.Chmod(filepath.Join(app.Root, "sites", website.Name), 0755); err != nil {
		return err
	}
	if err = io.Chmod(website.Path, 0755); err != nil {
		return err
	}
	if err = io.Chown(website.Path, "www", "www"); err != nil {
		return err
	}
	if err = io.Chmod(filepath.Join(app.Root, "sites", website.Name, "log"), 0701); err != nil {
		return err
	}
	if err = io.Chmod(filepath.Join(app.Root, "sites", website.Name, "config"), 0600); err != nil {
		return err
	}

	website.Status = true
	website.SSL = false
	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	return r.reloadWebServer()
}

func (r *websiteRepo) UpdateStatus(id uint, status bool) error {
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

	return r.reloadWebServer()
}

func (r *websiteRepo) UpdateCert(req *request.WebsiteUpdateCert) error {
	website := new(biz.Website)
	if err := r.db.Where("name", req.Name).First(&website).Error; err != nil {
		return err
	}

	if _, err := cert.ParseCert(req.Cert); err != nil {
		return errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err := cert.ParseKey(req.Key); err != nil {
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
		return r.reloadWebServer()
	}

	return nil
}

func (r *websiteRepo) ObtainCert(ctx context.Context, id uint) error {
	website, err := r.Get(id)
	if err != nil {
		return err
	}
	if slices.Contains(website.Domains, "*") {
		return errors.New(r.t.Get("not support one-key obtain wildcard certificate, please use Cert menu to obtain it with DNS method"))
	}

	account, err := r.certAccount.GetDefault(cast.ToUint(ctx.Value("user_id")))
	if err != nil {
		return err
	}

	newCert, err := r.cert.GetByWebsite(website.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newCert, err = r.cert.Create(&request.CertCreate{
				Type:        string(acme.KeyEC256),
				Domains:     website.Domains,
				AutoRenewal: true,
				AccountID:   account.ID,
				WebsiteID:   website.ID,
			})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	newCert.Domains = website.Domains
	if err = r.db.Save(newCert).Error; err != nil {
		return err
	}

	_, err = r.cert.ObtainAuto(newCert.ID)
	if err != nil {
		return err
	}

	return r.cert.Deploy(newCert.ID, website.ID)
}

func (r *websiteRepo) getVhost(website *biz.Website) (webservertypes.Vhost, error) {
	webServer, err := r.setting.Get(biz.SettingKeyWebserver)
	if err != nil {
		return nil, err
	}

	var vhost webservertypes.Vhost
	switch website.Type {
	case biz.WebsiteTypeProxy:
		vhost, err = webserver.NewProxyVhost(webserver.Type(webServer), filepath.Join(app.Root, "sites", website.Name, "config"))
	case biz.WebsiteTypePHP:
		vhost, err = webserver.NewPHPVhost(webserver.Type(webServer), filepath.Join(app.Root, "sites", website.Name, "config"))
	case biz.WebsiteTypeStatic:
		vhost, err = webserver.NewStaticVhost(webserver.Type(webServer), filepath.Join(app.Root, "sites", website.Name, "config"))
	default:
		return nil, errors.New(r.t.Get("unsupported website type: %s", website.Type))
	}
	if err != nil {
		return nil, err
	}

	return vhost, nil
}

func (r *websiteRepo) getPHPVersion(name string) uint {
	vhost, err := webserver.NewPHPVhost(webserver.TypeNginx, filepath.Join(app.Root, "sites", name, "config"))
	if err != nil {
		return 0
	}
	return vhost.PHP()
}

func (r *websiteRepo) reloadWebServer() error {
	webServer, err := r.setting.Get(biz.SettingKeyWebserver, "unknown")
	if err != nil {
		return err
	}
	switch webServer {
	case "nginx":
		if err = systemctl.Reload("nginx"); err != nil {
			_, err = shell.Execf("nginx -t")
			return err
		}
	case "apache":
		if err = systemctl.Reload("httpd"); err != nil {
			_, err = shell.Execf("apachectl configtest")
			return err
		}
	default:
		return errors.New(r.t.Get("unsupported web server: %s", webServer))
	}

	return nil
}
