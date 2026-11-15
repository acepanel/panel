package data

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/acme"
	pkgcert "github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

type certRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

func NewCertRepo(db *gorm.DB, t *gotext.Locale, log *slog.Logger) biz.CertRepo {
	return &certRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

func (r *certRepo) List(page, limit uint) ([]*types.CertList, int64, error) {
	var certs []*biz.Cert
	var total int64
	err := r.db.Model(&biz.Cert{}).Preload("Websites").Preload("Account").Preload("DNS").Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&certs).Error

	list := lo.Map(certs, func(cert *biz.Cert, _ int) *types.CertList {
		item := &types.CertList{
			ID:          cert.ID,
			AccountID:   cert.AccountID,
			WebsiteIDs:  cert.WebsiteIDs(),
			DNSID:       cert.DNSID,
			Type:        cert.Type,
			Domains:     cert.Domains,
			Alias:       cert.Alias,
			AutoRenewal: cert.AutoRenewal,
			NextRenewal: cert.RenewalInfo.SelectedTime,
			Cert:        cert.Cert,
			Key:         cert.Key,
			CertURL:     cert.CertURL,
			Script:      cert.Script,
			CreatedAt:   cert.CreatedAt,
			UpdatedAt:   cert.UpdatedAt,
		}
		if decode, err := pkgcert.ParseCert([]byte(cert.Cert)); err == nil {
			item.NotBefore = decode.NotBefore
			item.NotAfter = decode.NotAfter
			item.Issuer = decode.Issuer.CommonName
			item.OCSPServer = decode.OCSPServer
			item.DNSNames = pkgcert.SANs(decode)
		}
		return item
	})

	return list, total, err
}

func (r *certRepo) Get(id uint) (*biz.Cert, error) {
	cert := new(biz.Cert)
	err := r.db.Model(&biz.Cert{}).Preload("Websites").Preload("Account").Preload("DNS").Where("id = ?", id).First(cert).Error
	return cert, err
}

func (r *certRepo) GetByWebsite(websiteID uint) (*biz.Cert, error) {
	cert := new(biz.Cert)
	err := r.db.Model(&biz.Cert{}).Preload("Websites").Preload("Account").Preload("DNS").
		Joins("JOIN websites ON websites.cert_id = certs.id").
		Where("websites.id = ?", websiteID).First(cert).Error
	return cert, err
}

func (r *certRepo) Create(req *request.CertCreate) (*biz.Cert, error) {
	cert := &biz.Cert{
		AccountID:   req.AccountID,
		DNSID:       req.DNSID,
		Type:        req.Type,
		Domains:     req.Domains,
		Alias:       req.Alias,
		AutoRenewal: req.AutoRenewal,
	}
	if err := r.db.Create(cert).Error; err != nil {
		return nil, err
	}
	if err := r.BindWebsites(cert.ID, req.WebsiteIDs); err != nil {
		return nil, err
	}

	return cert, nil
}

func (r *certRepo) CreateUploaded(cert *biz.Cert) error {
	return r.db.Create(cert).Error
}

func (r *certRepo) Update(req *request.CertUpdate) error {
	if err := r.db.Model(&biz.Cert{}).Where("id = ?", req.ID).
		Select("*").Omit("Websites", "RenewalInfo", "CertURL", "CreatedAt").Updates(&biz.Cert{
		ID:          req.ID,
		AccountID:   req.AccountID,
		DNSID:       req.DNSID,
		Type:        req.Type,
		Cert:        req.Cert,
		Key:         req.Key,
		Script:      req.Script,
		Domains:     req.Domains,
		Alias:       req.Alias,
		AutoRenewal: req.AutoRenewal,
	}).Error; err != nil {
		return err
	}

	return r.BindWebsites(req.ID, req.WebsiteIDs)
}

func (r *certRepo) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&biz.Website{}).Where("cert_id = ?", id).Update("cert_id", 0).Error; err != nil {
			return err
		}
		return tx.Delete(&biz.Cert{ID: id}).Error
	})
}

func (r *certRepo) Save(cert *biz.Cert) error {
	// 跳过关联，避免连带回写网站表
	return r.db.Omit("Websites").Save(cert).Error
}

func (r *certRepo) GetClient(ctx context.Context, cert *biz.Cert) (*acme.Client, error) {
	if cert.Account == nil {
		return nil, errors.New(r.t.Get("this certificate is not associated with an ACME account and cannot be obtained"))
	}

	return acme.NewPrivateKeyAccount(ctx, cert.Account.Email, cert.Account.PrivateKey, cert.Account.Directory(), cert.Account.EAB(), r.log)
}

func (r *certRepo) GenerateSelfSigned(domains []string) ([]byte, []byte, error) {
	return pkgcert.GenerateSelfSigned(domains)
}

func (r *certRepo) RunScript(ctx context.Context, cert *biz.Cert) error {
	if cert.Script == "" {
		return nil
	}

	f, err := os.CreateTemp("", "cert-deploy-*.sh")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()

	// 替换变量
	script := strings.ReplaceAll(cert.Script, "{cert}", cert.Cert)
	script = strings.ReplaceAll(script, "{key}", cert.Key)

	_, err = f.WriteString(script)
	_ = f.Chmod(0755)
	_ = f.Close()
	if err != nil {
		return err
	}

	_, err = shell.Execf(ctx, "bash %s", f.Name())
	return err
}

func (r *certRepo) ObtainPanel(
	ctx context.Context,
	account *biz.CertAccount,
	names []string,
	webServer string,
) ([]byte, []byte, error) {
	client, err := acme.NewPrivateKeyAccount(ctx, account.Email, account.PrivateKey, acme.CALetsEncrypt, nil, r.log)
	if err != nil {
		return nil, nil, err
	}

	// 未安装 Web 服务器时不传 writer，由内置 HTTP 服务器完成验证
	var conf string
	var writer acme.HTTPChallengeWriter
	if d, err := webserver.Get(webserver.Type(webServer)); err == nil {
		conf, writer = d.PanelACMEConf(), d
	}
	client.UsePanel(conf, writer)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	ssl, err := client.ObtainCertificate(ctx, names, acme.KeyEC256)
	if err != nil {
		return nil, nil, err
	}

	return ssl.ChainPEM, ssl.PrivateKey, nil
}

// LoadWebsites 根据 ID 列表加载网站
func (r *certRepo) LoadWebsites(websiteIDs []uint) ([]*biz.Website, error) {
	websiteIDs = lo.Uniq(websiteIDs)
	if len(websiteIDs) == 0 {
		return nil, nil
	}

	var websites []*biz.Website
	if err := r.db.Where("id IN ?", websiteIDs).Find(&websites).Error; err != nil {
		return nil, err
	}
	if len(websites) != len(websiteIDs) {
		return nil, errors.New(r.t.Get("some of the selected websites do not exist"))
	}

	return websites, nil
}

// BindWebsites 绑定证书的部署网站
func (r *certRepo) BindWebsites(certID uint, websiteIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&biz.Website{}).Where("cert_id = ?", certID).Update("cert_id", 0).Error; err != nil {
			return err
		}
		return tx.Model(&biz.Website{}).Where("id IN ?", websiteIDs).Update("cert_id", certID).Error
	})
}

// HTTPConfs 构建证书关联网站的「域名 → acme 配置文件」映射以及全部配置文件列表
func (r *certRepo) HTTPConfs(cert *biz.Cert, webServer string) (map[string]string, []string) {
	confs := make(map[string]string)
	fallback := make([]string, 0, len(cert.Websites))

	d, dErr := webserver.Get(webserver.Type(webServer))
	for _, website := range cert.Websites {
		conf := filepath.Join(app.Root, "sites", website.Name, "config", "site", "001-acme.conf")
		fallback = append(fallback, conf)

		if dErr != nil {
			continue
		}
		vhost, err := newVhost(d, website)
		if err != nil {
			continue
		}
		for _, name := range vhost.ServerName() {
			confs[strings.ToLower(tools.UnwrapIPv6(name))] = conf
		}
	}

	return confs, fallback
}

// WriteCertFiles 写入证书与私钥文件
func (r *certRepo) WriteCertFiles(cert *biz.Cert, certPath, keyPath string) error {
	if err := io.Write(certPath, cert.Cert, 0600); err != nil {
		return err
	}
	if err := io.Write(keyPath, cert.Key, 0600); err != nil {
		return err
	}
	return nil
}

// EnableWebsiteSSL 为网站开启 HTTPS
func (r *certRepo) EnableWebsiteSSL(
	website *biz.Website,
	certPath, keyPath, webServer string,
	tlsVersions []string,
	listenIPv6 bool,
) error {
	d, err := webserver.Get(webserver.Type(webServer))
	if err != nil {
		return err
	}
	vhost, err := newVhost(d, website)
	if err != nil {
		return err
	}

	// 添加 443 监听
	if err = vhost.SetListen(d.MergeHTTPSListens(vhost.Listen(), listenIPv6)); err != nil {
		return err
	}

	// 配置 SSL
	if err = vhost.SetSSLConfig(&webservertypes.SSLConfig{
		Cert:      certPath,
		Key:       keyPath,
		Protocols: tlsVersions,
	}); err != nil {
		return err
	}

	if err = vhost.Save(); err != nil {
		return err
	}

	website.SSL = true
	if err = r.db.Model(website).Update("ssl", true).Error; err != nil {
		return err
	}

	return nil
}

// ReloadWebserver 重载 Web 服务器
func (r *certRepo) ReloadWebserver(ctx context.Context, webServer string) error {
	d, err := webserver.Get(webserver.Type(webServer))
	if err != nil {
		return err
	}

	return d.ReloadIfRunning(ctx)
}
