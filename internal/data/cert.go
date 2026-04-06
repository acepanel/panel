package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	mholtacme "github.com/mholt/acmez/v3/acme"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/acme"
	pkgcert "github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

type certRepo struct {
	t           *gotext.Locale
	db          *gorm.DB
	log         *slog.Logger
	settingRepo biz.SettingRepo
}

func NewCertRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger, settingRepo biz.SettingRepo) biz.CertRepo {
	return &certRepo{
		t:           t,
		db:          db,
		log:         log,
		settingRepo: settingRepo,
	}
}

func (r *certRepo) List(page, limit uint) ([]*types.CertList, int64, error) {
	var certs []*biz.Cert
	var total int64
	err := r.db.Model(&biz.Cert{}).Preload("Website").Preload("Account").Preload("DNS").Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&certs).Error

	list := make([]*types.CertList, 0)
	for cert := range slices.Values(certs) {
		item := &types.CertList{
			ID:          cert.ID,
			AccountID:   cert.AccountID,
			WebsiteID:   cert.WebsiteID,
			DNSID:       cert.DNSID,
			Type:        cert.Type,
			Domains:     cert.Domains,
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
			// 合并 DNSNames 和 IPAddresses
			item.DNSNames = decode.DNSNames
			for _, ip := range decode.IPAddresses {
				item.DNSNames = append(item.DNSNames, ip.String())
			}
		}
		list = append(list, item)
	}

	return list, total, err
}

func (r *certRepo) Get(id uint) (*biz.Cert, error) {
	cert := new(biz.Cert)
	err := r.db.Model(&biz.Cert{}).Preload("Website").Preload("Account").Preload("DNS").Where("id = ?", id).First(cert).Error
	return cert, err
}

func (r *certRepo) GetByWebsite(WebsiteID uint) (*biz.Cert, error) {
	cert := new(biz.Cert)
	err := r.db.Model(&biz.Cert{}).Preload("Website").Preload("Account").Preload("DNS").Where("website_id = ?", WebsiteID).First(cert).Error
	return cert, err
}

func (r *certRepo) Upload(ctx context.Context, req *request.CertUpload) (*biz.Cert, error) {
	info, err := pkgcert.ParseCert([]byte(req.Cert))
	if err != nil {
		return nil, errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err = pkgcert.ParseKey([]byte(req.Key)); err != nil {
		return nil, errors.New(r.t.Get("failed to parse private key: %v", err))
	}

	// 合并 DNSNames 和 IPAddresses
	domains := info.DNSNames
	for _, ip := range info.IPAddresses {
		domains = append(domains, ip.String())
	}

	cert := &biz.Cert{
		Type:    "upload",
		Domains: domains,
		Cert:    req.Cert,
		Key:     req.Key,
	}
	if err = r.db.Create(cert).Error; err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("cert uploaded", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)))

	return cert, nil
}

func (r *certRepo) Create(ctx context.Context, req *request.CertCreate) (*biz.Cert, error) {
	cert := &biz.Cert{
		AccountID:   req.AccountID,
		WebsiteID:   req.WebsiteID,
		DNSID:       req.DNSID,
		Type:        req.Type,
		Domains:     req.Domains,
		AutoRenewal: req.AutoRenewal,
	}
	if err := r.db.Create(cert).Error; err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("cert created", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.String("cert_type", req.Type))

	return cert, nil
}

func (r *certRepo) Update(ctx context.Context, req *request.CertUpdate) error {
	info, err := pkgcert.ParseCert([]byte(req.Cert))
	if err == nil && req.Type == "upload" {
		// 合并 DNSNames 和 IPAddresses
		req.Domains = info.DNSNames
		for _, ip := range info.IPAddresses {
			req.Domains = append(req.Domains, ip.String())
		}
	}
	if req.Type == "upload" && req.AutoRenewal {
		return errors.New(r.t.Get("upload certificate cannot be set to auto renewal"))
	}

	if err = r.db.Model(&biz.Cert{}).Where("id = ?", req.ID).Select("*").Updates(&biz.Cert{
		ID:          req.ID,
		AccountID:   req.AccountID,
		WebsiteID:   req.WebsiteID,
		DNSID:       req.DNSID,
		Type:        req.Type,
		Cert:        req.Cert,
		Key:         req.Key,
		Script:      req.Script,
		Domains:     req.Domains,
		AutoRenewal: req.AutoRenewal,
	}).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cert updated", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(req.ID)))

	return nil
}

func (r *certRepo) Delete(ctx context.Context, id uint) error {
	if err := r.db.Model(&biz.Cert{}).Where("id = ?", id).Delete(&biz.Cert{}).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cert deleted", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)))

	return nil
}

func (r *certRepo) ObtainAuto(id uint) (*acme.Certificate, error) {
	return r.ObtainAutoWithProgressCallback(context.Background(), id, nil)
}

func (r *certRepo) ObtainAutoWithProgressCallback(ctx context.Context, id uint, progressCallback func(string)) (*acme.Certificate, error) {
	report := func(msg string) {
		if progressCallback != nil {
			progressCallback(msg)
		}
	}

	report(r.t.Get("preparing ACME client"))
	cert, err := r.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := r.getClient(cert)
	if err != nil {
		return nil, err
	}

	webServer, _ := r.settingRepo.Get(biz.SettingKeyWebserver)

	if cert.DNS != nil {
		client.UseDns(cert.DNS.Type, cert.DNS.Data, acme.DnsOption{
			DnsServer:        cert.DNS.Data.DnsServer,
			SkipVerify:       cert.DNS.Data.SkipVerify,
			ProgressCallback: progressCallback,
		})
	} else {
		if cert.Website == nil {
			return nil, errors.New(r.t.Get("this certificate is not associated with a website and cannot be obtained. You can try to obtain it manually"))
		}
		hasWildcard := slices.ContainsFunc(cert.Domains, func(d string) bool {
			return strings.Contains(d, "*")
		})
		if hasWildcard {
			return nil, errors.New(r.t.Get("wildcard domains cannot use HTTP verification"))
		}
		conf := fmt.Sprintf("%s/sites/%s/config/site/001-acme.conf", app.Root, cert.Website.Name)
		client.UseHTTP(conf, webServer)
	}

	report(r.t.Get("issuing certificate, domains: %s", strings.Join(cert.Domains, ", ")))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ssl, err := client.ObtainCertificate(ctx, cert.Domains, acme.KeyType(cert.Type))
	if err != nil {
		return nil, err
	}

	report(r.t.Get("obtaining and saving certificate"))
	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = r.db.Save(cert).Error; err != nil {
		return nil, err
	}

	if cert.Website != nil {
		report(r.t.Get("deploying certificate to website"))
		return &ssl, r.Deploy(cert.ID, cert.WebsiteID, false)
	}

	if err = r.runScript(cert); err != nil {
		return nil, err
	}

	return &ssl, nil
}

func (r *certRepo) ObtainPanel(account *biz.CertAccount, ips []string) ([]byte, []byte, error) {
	client, err := acme.NewPrivateKeyAccount(account.Email, account.PrivateKey, acme.CALetsEncrypt, nil, r.log)
	if err != nil {
		return nil, nil, err
	}

	webServer, _ := r.settingRepo.Get(biz.SettingKeyWebserver)
	confPath := filepath.Join(app.Root, "server/nginx/conf/acme.conf")
	if webServer == "apache" {
		confPath = filepath.Join(app.Root, "server/apache/conf/extra/acme.conf")
	}
	client.UsePanel(ips, confPath, webServer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ssl, err := client.ObtainIPCertificate(ctx, ips, acme.KeyEC256)
	if err != nil {
		return nil, nil, err
	}

	return ssl.ChainPEM, ssl.PrivateKey, nil
}

func (r *certRepo) ObtainSelfSigned(id uint) error {
	cert, err := r.Get(id)
	if err != nil {
		return err
	}

	crt, key, err := pkgcert.GenerateSelfSigned(cert.Domains)
	if err != nil {
		return err
	}

	cert.Cert = string(crt)
	cert.Key = string(key)
	if err = r.db.Save(cert).Error; err != nil {
		return err
	}

	if cert.Website != nil {
		return r.Deploy(cert.ID, cert.WebsiteID, false)
	}

	if err = r.runScript(cert); err != nil {
		return err
	}

	return nil
}

func (r *certRepo) Renew(id uint) (*acme.Certificate, error) {
	return r.RenewWithProgressCallback(context.Background(), id, nil)
}

func (r *certRepo) RenewWithProgressCallback(ctx context.Context, id uint, progressCallback func(string)) (*acme.Certificate, error) {
	report := func(msg string) {
		if progressCallback != nil {
			progressCallback(msg)
		}
	}

	report(r.t.Get("preparing renewal"))
	cert, err := r.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := r.getClient(cert)
	if err != nil {
		return nil, err
	}

	if cert.CertURL == "" {
		return nil, errors.New(r.t.Get("this certificate has not been obtained successfully and cannot be renewed"))
	}

	webServer, _ := r.settingRepo.Get(biz.SettingKeyWebserver)

	if cert.DNS != nil {
		client.UseDns(cert.DNS.Type, cert.DNS.Data, acme.DnsOption{
			DnsServer:        cert.DNS.Data.DnsServer,
			SkipVerify:       cert.DNS.Data.SkipVerify,
			ProgressCallback: progressCallback,
		})
	} else {
		if cert.Website == nil {
			return nil, errors.New(r.t.Get("this certificate is not associated with a website and cannot be obtained. You can try to obtain it manually"))
		} else {
			for _, domain := range cert.Domains {
				if strings.Contains(domain, "*") {
					return nil, errors.New(r.t.Get("wildcard domains cannot use HTTP verification"))
				}
			}
			conf := fmt.Sprintf("%s/sites/%s/config/site/001-acme.conf", app.Root, cert.Website.Name)
			client.UseHTTP(conf, webServer)
		}
	}

	report(r.t.Get("renewing certificate, domains: %s", strings.Join(cert.Domains, ", ")))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	ssl, err := client.RenewCertificate(ctx, cert.CertURL, cert.Domains, acme.KeyType(cert.Type))
	if err != nil {
		// 续签失败，尝试重签
		report(r.t.Get("renewal failed, attempting re-issuance"))
		ssl, err = client.ObtainCertificate(ctx, cert.Domains, acme.KeyType(cert.Type))
		if err != nil {
			return nil, err
		}
	}

	report(r.t.Get("obtaining and saving certificate"))
	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = r.db.Save(cert).Error; err != nil {
		return nil, err
	}

	if cert.Website != nil {
		report(r.t.Get("deploying certificate to website"))
		return &ssl, r.Deploy(cert.ID, cert.WebsiteID, false)
	}

	return &ssl, nil
}

func (r *certRepo) RefreshRenewalInfo(id uint) (mholtacme.RenewalInfo, error) {
	cert, err := r.Get(id)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}
	client, err := r.getClient(cert)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	crt, err := pkgcert.ParseCert([]byte(cert.Cert))
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	renewInfo, err := client.GetRenewalInfo(ctx, crt)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	cert.RenewalInfo = renewInfo
	if err = r.db.Save(cert).Error; err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	return renewInfo, nil
}

func (r *certRepo) Deploy(ID, WebsiteID uint, enableHTTPS bool) error {
	cert, err := r.Get(ID)
	if err != nil {
		return err
	}

	if cert.Cert == "" || cert.Key == "" {
		return errors.New(r.t.Get("this certificate has not been obtained successfully and cannot be deployed"))
	}

	website := new(biz.Website)
	if err = r.db.Where("id", WebsiteID).First(website).Error; err != nil {
		return err
	}
	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	certPath := filepath.Join(configDir, "fullchain.pem")
	keyPath := filepath.Join(configDir, "private.key")
	if err = io.Write(certPath, cert.Cert, 0600); err != nil {
		return err
	}
	if err = io.Write(keyPath, cert.Key, 0600); err != nil {
		return err
	}

	// 开启 HTTPS
	if enableHTTPS && !website.SSL {
		vhost, vhostErr := r.getVhost(website)
		if vhostErr != nil {
			return vhostErr
		}

		// 添加 443 监听
		listens := vhost.Listen()
		hasSSL := slices.ContainsFunc(listens, func(l webservertypes.Listen) bool {
			return slices.Contains(l.Args, "ssl")
		})
		if !hasSSL {
			webServer, _ := r.settingRepo.Get(biz.SettingKeyWebserver)
			args := []string{"ssl"}
			if webServer != "apache" {
				args = append(args, "quic")
			}
			listens = append(listens, webservertypes.Listen{Address: "443", Args: args})
			if err = vhost.SetListen(listens); err != nil {
				return err
			}
		}

		// 配置 SSL
		defaultTLSVersions, _ := r.settingRepo.GetSlice(biz.SettingKeyWebsiteTLSVersions)
		if err = vhost.SetSSLConfig(&webservertypes.SSLConfig{
			Cert:      certPath,
			Key:       keyPath,
			Protocols: defaultTLSVersions,
		}); err != nil {
			return err
		}

		if err = vhost.Save(); err != nil {
			return err
		}

		website.SSL = true
		if err = r.db.Save(website).Error; err != nil {
			return err
		}
	}

	webServer, _ := r.settingRepo.Get(biz.SettingKeyWebserver)
	if webServer == "apache" {
		if err = systemctl.Reload("apache"); err != nil {
			_, err = shell.Execf("apachectl -t")
			return err
		}
	} else {
		if err = systemctl.Reload("nginx"); err != nil {
			_, err = shell.Execf("nginx -t")
			return err
		}
	}

	return nil
}

// getVhost 根据网站类型获取虚拟主机配置
func (r *certRepo) getVhost(website *biz.Website) (webservertypes.Vhost, error) {
	webServer, err := r.settingRepo.Get(biz.SettingKeyWebserver)
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	var vhost webservertypes.Vhost
	switch website.Type {
	case biz.WebsiteTypeProxy:
		vhost, err = webserver.NewProxyVhost(webserver.Type(webServer), configDir)
	case biz.WebsiteTypePHP:
		vhost, err = webserver.NewPHPVhost(webserver.Type(webServer), configDir)
	case biz.WebsiteTypeStatic:
		vhost, err = webserver.NewStaticVhost(webserver.Type(webServer), configDir)
	default:
		return nil, errors.New(r.t.Get("unsupported website type: %s", website.Type))
	}
	if err != nil {
		return nil, err
	}

	return vhost, nil
}

func (r *certRepo) runScript(cert *biz.Cert) error {
	if cert.Script == "" {
		return nil
	}

	f, err := os.CreateTemp("", "cert-deploy-*.sh")
	if err != nil {
		return err
	}

	// 替换变量
	cert.Script = strings.ReplaceAll(cert.Script, "{cert}", cert.Cert)
	cert.Script = strings.ReplaceAll(cert.Script, "{key}", cert.Key)

	if _, err = f.WriteString(cert.Script); err != nil {
		return err
	}

	_ = f.Chmod(0755)
	_ = f.Close()
	defer func(name string) { _ = os.Remove(name) }(f.Name())

	_, err = shell.Execf("bash " + f.Name())
	return err
}

func (r *certRepo) getClient(cert *biz.Cert) (*acme.Client, error) {
	if cert.Account == nil {
		return nil, errors.New(r.t.Get("this certificate is not associated with an ACME account and cannot be obtained"))
	}

	var ca string
	var eab *acme.EAB
	switch cert.Account.CA {
	case "googlecn":
		ca = acme.CAGoogleCN
		eab = &acme.EAB{KeyID: cert.Account.Kid, MACKey: cert.Account.HmacEncoded}
	case "google":
		ca = acme.CAGoogle
		eab = &acme.EAB{KeyID: cert.Account.Kid, MACKey: cert.Account.HmacEncoded}
	case "letsencrypt":
		ca = acme.CALetsEncrypt
	case "litessl":
		ca = acme.CALiteSSL
	case "buypass":
		ca = acme.CABuypass
	case "zerossl":
		ca = acme.CAZeroSSL
		eab = &acme.EAB{KeyID: cert.Account.Kid, MACKey: cert.Account.HmacEncoded}
	case "sslcom":
		ca = acme.CASSLcom
		eab = &acme.EAB{KeyID: cert.Account.Kid, MACKey: cert.Account.HmacEncoded}
	}

	return acme.NewPrivateKeyAccount(cert.Account.Email, cert.Account.PrivateKey, ca, eab, r.log)
}
