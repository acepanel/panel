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

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/acme"
	pkgcert "github.com/acepanel/panel/pkg/cert"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/types"
)

type certRepo struct {
	t      *gotext.Locale
	db     *gorm.DB
	log    *slog.Logger
	client *acme.Client
}

func NewCertRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.CertRepo {
	return &certRepo{
		t:   t,
		db:  db,
		log: log,
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
		if decode, err := pkgcert.ParseCert(cert.Cert); err == nil {
			item.NotBefore = decode.NotBefore
			item.NotAfter = decode.NotAfter
			item.Issuer = decode.Issuer.CommonName
			item.OCSPServer = decode.OCSPServer
			item.DNSNames = decode.DNSNames
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

func (r *certRepo) Upload(req *request.CertUpload) (*biz.Cert, error) {
	info, err := pkgcert.ParseCert(req.Cert)
	if err != nil {
		return nil, errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err = pkgcert.ParseKey(req.Key); err != nil {
		return nil, errors.New(r.t.Get("failed to parse private key: %v", err))
	}

	cert := &biz.Cert{
		Type:    "upload",
		Domains: info.DNSNames,
		Cert:    req.Cert,
		Key:     req.Key,
	}
	if err = r.db.Create(cert).Error; err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("cert uploaded", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(cert.ID)))

	return cert, nil
}

func (r *certRepo) Create(req *request.CertCreate) (*biz.Cert, error) {
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
	r.log.Info("cert created", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(cert.ID)), slog.String("cert_type", req.Type))

	return cert, nil
}

func (r *certRepo) Update(req *request.CertUpdate) error {
	info, err := pkgcert.ParseCert(req.Cert)
	if err == nil && req.Type == "upload" {
		req.Domains = info.DNSNames
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
	r.log.Info("cert updated", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(req.ID)))

	return nil
}

func (r *certRepo) Delete(id uint) error {
	if err := r.db.Model(&biz.Cert{}).Where("id = ?", id).Delete(&biz.Cert{}).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cert deleted", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(id)))

	return nil
}

func (r *certRepo) ObtainAuto(id uint) (*acme.Certificate, error) {
	cert, err := r.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := r.getClient(cert)
	if err != nil {
		return nil, err
	}

	if cert.DNS != nil {
		client.UseDns(cert.DNS.Type, cert.DNS.Data)
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
			client.UseHTTP(conf)
		}
	}

	ssl, err := client.ObtainCertificate(context.Background(), cert.Domains, acme.KeyType(cert.Type))
	if err != nil {
		return nil, err
	}

	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = r.db.Save(cert).Error; err != nil {
		return nil, err
	}

	if cert.Website != nil {
		return &ssl, r.Deploy(cert.ID, cert.WebsiteID)
	}

	if err = r.runScript(cert); err != nil {
		return nil, err
	}

	return &ssl, nil
}

func (r *certRepo) ObtainManual(id uint) (*acme.Certificate, error) {
	cert, err := r.Get(id)
	if err != nil {
		return nil, err
	}

	if r.client == nil {
		return nil, errors.New(r.t.Get("please retry the manual obtain operation"))
	}

	ssl, err := r.client.ObtainCertificateManual()
	if err != nil {
		return nil, err
	}

	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = r.db.Save(cert).Error; err != nil {
		return nil, err
	}

	if cert.Website != nil {
		return &ssl, r.Deploy(cert.ID, cert.WebsiteID)
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
	client.UsePanel(ips, filepath.Join(app.Root, "server/nginx/conf/acme.conf"))

	ssl, err := client.ObtainIPCertificate(context.Background(), ips, acme.KeyEC256)
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

	crt, key, err := pkgcert.GenerateSelfSignedRSA(cert.Domains)
	if err != nil {
		return err
	}

	cert.Cert = string(crt)
	cert.Key = string(key)
	if err = r.db.Save(cert).Error; err != nil {
		return err
	}

	if cert.Website != nil {
		return r.Deploy(cert.ID, cert.WebsiteID)
	}

	if err = r.runScript(cert); err != nil {
		return err
	}

	return nil
}

func (r *certRepo) Renew(id uint) (*acme.Certificate, error) {
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

	if cert.DNS != nil {
		client.UseDns(cert.DNS.Type, cert.DNS.Data)
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
			client.UseHTTP(conf)
		}
	}

	ssl, err := client.RenewCertificate(context.Background(), cert.CertURL, cert.Domains, acme.KeyType(cert.Type))
	if err != nil {
		// 续签失败，尝试重签
		ssl, err = client.ObtainCertificate(context.Background(), cert.Domains, acme.KeyType(cert.Type))
		if err != nil {
			return nil, err
		}
	}

	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = r.db.Save(cert).Error; err != nil {
		return nil, err
	}

	if cert.Website != nil {
		return &ssl, r.Deploy(cert.ID, cert.WebsiteID)
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

	crt, err := pkgcert.ParseCert(cert.Cert)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	renewInfo, err := client.GetRenewalInfo(context.Background(), crt)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	cert.RenewalInfo = renewInfo
	if err = r.db.Save(cert).Error; err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	return renewInfo, nil
}

func (r *certRepo) ManualDNS(id uint) ([]acme.DNSRecord, error) {
	cert, err := r.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := r.getClient(cert)
	if err != nil {
		return nil, err
	}

	client.UseManualDns()
	records, err := client.GetDNSRecords(context.Background(), cert.Domains, acme.KeyType(cert.Type))
	if err != nil {
		return nil, err
	}

	// 15 分钟后清理客户端
	r.client = client
	time.AfterFunc(15*time.Minute, func() {
		r.client = nil
	})

	return records, nil
}

func (r *certRepo) Deploy(ID, WebsiteID uint) error {
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
	if err = io.Write(fmt.Sprintf("%s/sites/%s/config/fullchain.pem", app.Root, website.Name), cert.Cert, 0600); err != nil {
		return err
	}
	if err = io.Write(fmt.Sprintf("%s/sites/%s/config/private.key", app.Root, website.Name), cert.Key, 0600); err != nil {
		return err
	}
	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return err
	}

	return nil
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
	if err = f.Chmod(0755); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
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
