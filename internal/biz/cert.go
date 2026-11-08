package biz

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	mholtacme "github.com/mholt/acmez/v3/acme"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/acme"
	pkgcert "github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
)

type Cert struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	AccountID   uint                  `gorm:"not null;default:0" json:"account_id"`                      // 关联的 ACME 账户 ID
	DNSID       uint                  `gorm:"not null;default:0" json:"dns_id"`                          // 关联的 DNS ID
	Type        string                `gorm:"not null;default:''" json:"type"`                           // 证书类型 (P256, P384, 2048, 3072, 4096)
	Domains     []string              `gorm:"not null;default:'[]';serializer:json" json:"domains"`      // 域名
	Alias       map[string]string     `gorm:"not null;default:'{}';serializer:json" json:"alias"`        // DNS 验证别名
	AutoRenewal bool                  `gorm:"not null;default:false" json:"auto_renewal"`                // 自动续签
	RenewalInfo mholtacme.RenewalInfo `gorm:"not null;default:'{}';serializer:json" json:"renewal_info"` // 续签信息
	CertURL     string                `gorm:"not null;default:''" json:"cert_url"`                       // 证书 URL (续签时使用)
	Cert        string                `gorm:"not null;default:''" json:"cert"`                           // 证书内容
	Key         string                `gorm:"not null;default:''" json:"key"`                            // 私钥内容
	Script      string                `gorm:"not null;default:''" json:"script"`                         // 部署脚本
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`

	Websites []*Website   `gorm:"foreignKey:CertID" json:"websites"` // 部署的网站
	Account  *CertAccount `gorm:"foreignKey:AccountID" json:"account"`
	DNS      *CertDNS     `gorm:"foreignKey:DNSID" json:"dns"`
}

// WebsiteIDs 取证书关联的网站 ID 列表
func (r *Cert) WebsiteIDs() []uint {
	return lo.Map(r.Websites, func(website *Website, _ int) uint {
		return website.ID
	})
}

type CertRepo interface {
	List(page, limit uint) ([]*types.CertList, int64, error)
	Get(id uint) (*Cert, error)
	GetByWebsite(WebsiteID uint) (*Cert, error)
	Create(req *request.CertCreate) (*Cert, error)
	CreateUploaded(cert *Cert) error
	Update(req *request.CertUpdate) error
	Delete(id uint) error
	Save(cert *Cert) error
	GetClient(ctx context.Context, cert *Cert) (*acme.Client, error)
	GenerateSelfSigned(domains []string) ([]byte, []byte, error)
	RunScript(ctx context.Context, cert *Cert) error
	ObtainPanel(ctx context.Context, account *CertAccount, names []string, webServer string) ([]byte, []byte, error)
	LoadWebsites(websiteIDs []uint) ([]*Website, error)
	BindWebsites(certID uint, websiteIDs []uint) error
	HTTPConfs(cert *Cert, webServer string) (map[string]string, []string)
	WriteCertFiles(cert *Cert, certPath, keyPath string) error
	EnableWebsiteSSL(website *Website, certPath, keyPath, webServer string, tlsVersions []string, listenIPv6 bool) error
	ReloadWebserver(ctx context.Context, webServer string) error
}

type CertUsecase struct {
	repo    CertRepo
	setting SettingRepo
	t       *gotext.Locale
	log     *slog.Logger
}

func NewCertUsecase(t *gotext.Locale, log *slog.Logger, certRepo CertRepo, settingRepo SettingRepo) *CertUsecase {
	return &CertUsecase{
		repo:    certRepo,
		setting: settingRepo,
		t:       t,
		log:     log,
	}
}

func (uc *CertUsecase) List(page, limit uint) ([]*types.CertList, int64, error) {
	return uc.repo.List(page, limit)
}

func (uc *CertUsecase) Get(id uint) (*Cert, error) {
	return uc.repo.Get(id)
}

func (uc *CertUsecase) GetByWebsite(websiteID uint) (*Cert, error) {
	return uc.repo.GetByWebsite(websiteID)
}

func (uc *CertUsecase) Upload(ctx context.Context, req *request.CertUpload) (*Cert, error) {
	info, err := pkgcert.ParseCert([]byte(req.Cert))
	if err != nil {
		return nil, errors.New(uc.t.Get("failed to parse certificate: %v", err))
	}
	if _, err = pkgcert.ParseKey([]byte(req.Key)); err != nil {
		return nil, errors.New(uc.t.Get("failed to parse private key: %v", err))
	}

	// 合并 DNSNames 和 IPAddresses
	domains := info.DNSNames
	for _, ip := range info.IPAddresses {
		domains = append(domains, ip.String())
	}

	cert := &Cert{
		Type:    "upload",
		Domains: domains,
		Cert:    req.Cert,
		Key:     req.Key,
	}
	if err = uc.repo.CreateUploaded(cert); err != nil {
		return nil, err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert uploaded"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", domains), slog.Time("not_after", info.NotAfter))

	return cert, nil
}

func (uc *CertUsecase) Create(ctx context.Context, req *request.CertCreate) (*Cert, error) {
	cert, err := uc.repo.Create(req)
	if err != nil {
		return nil, err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert created"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", req.Domains), slog.String("cert_type", req.Type), slog.Bool("auto_renewal", req.AutoRenewal), slog.Any("website_ids", req.WebsiteIDs))

	return cert, nil
}

func (uc *CertUsecase) Update(ctx context.Context, req *request.CertUpdate) error {
	info, err := pkgcert.ParseCert([]byte(req.Cert))
	if err == nil && req.Type == "upload" {
		// 合并 DNSNames 和 IPAddresses
		req.Domains = info.DNSNames
		for _, ip := range info.IPAddresses {
			req.Domains = append(req.Domains, ip.String())
		}
	}
	if req.Type == "upload" && req.AutoRenewal {
		return errors.New(uc.t.Get("upload certificate cannot be set to auto renewal"))
	}

	if err = uc.repo.Update(req); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert updated"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.Any("domains", req.Domains), slog.String("cert_type", req.Type), slog.Bool("auto_renewal", req.AutoRenewal), slog.Any("website_ids", req.WebsiteIDs))

	return nil
}

func (uc *CertUsecase) Delete(ctx context.Context, id uint) error {
	cert, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("cert deleted"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.Any("domains", cert.Domains), slog.String("cert_type", cert.Type))

	return nil
}

func (uc *CertUsecase) ObtainAutoWithProgressCallback(ctx context.Context, id uint, progressCallback func(string)) (*acme.Certificate, error) {
	report := func(msg string) {
		if progressCallback != nil {
			progressCallback(msg)
		}
	}

	report(uc.t.Get("initializing ACME client"))
	cert, err := uc.repo.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := uc.repo.GetClient(ctx, cert)
	if err != nil {
		return nil, err
	}

	webServer, _ := uc.setting.Get(SettingKeyWebserver)

	if cert.DNS != nil {
		client.UseDns(cert.DNS.Type, cert.DNS.Data, acme.DnsOption{
			Alias:            cert.Alias,
			DnsServer:        cert.DNS.Data.DnsServer,
			SkipVerify:       cert.DNS.Data.SkipVerify,
			ProgressCallback: progressCallback,
		})
	} else {
		if len(cert.Websites) == 0 {
			return nil, errors.New(uc.t.Get("this certificate is not associated with a website and cannot be obtained. You can try to obtain it manually"))
		}
		hasWildcard := slices.ContainsFunc(cert.Domains, func(d string) bool {
			return strings.Contains(d, "*")
		})
		if hasWildcard {
			return nil, errors.New(uc.t.Get("wildcard domains cannot use HTTP verification"))
		}
		d, err := webserver.Get(webserver.Type(webServer))
		if err != nil {
			return nil, err
		}
		confs, fallback := uc.repo.HTTPConfs(cert, webServer)
		client.UseHTTP(confs, fallback, d)
	}

	report(uc.t.Get("issuing certificate, domains: %s", strings.Join(cert.Domains, ", ")))
	// 超时只覆盖 ACME 交互，后续的部署与用户脚本不该被签发耗时挤占额度
	obtainCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	ssl, err := client.ObtainCertificate(obtainCtx, cert.Domains, acme.KeyType(cert.Type))
	cancel()
	if err != nil {
		return nil, err
	}

	report(uc.t.Get("obtaining and saving certificate"))
	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = uc.repo.Save(cert); err != nil {
		return nil, err
	}

	if len(cert.Websites) > 0 {
		report(uc.t.Get("deploying certificate to website"))
		if _, _, err = uc.deploy(ctx, cert.ID, cert.WebsiteIDs(), false); err != nil {
			return nil, err
		}
	}

	// 绑定网站与部署脚本是两种独立的分发方式，证书可能还要同步到面板之外的服务
	if err = uc.repo.RunScript(ctx, cert); err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("cert obtained"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.String("cert_type", cert.Type), slog.String("challenge", lo.Ternary(cert.DNS != nil, "dns", "http")), slog.Any("website_ids", cert.WebsiteIDs()))

	return &ssl, nil
}

func (uc *CertUsecase) ObtainPanel(ctx context.Context, account *CertAccount, domains []string) ([]byte, []byte, error) {
	names := domains
	if len(names) == 0 {
		var err error
		names, err = uc.setting.GetSlice(SettingKeyPublicIPs)
		if err != nil {
			return nil, nil, err
		}
		if len(names) == 0 {
			return nil, nil, errors.New(uc.t.Get("Please set the panel IP in settings first for ACME certificate generation"))
		}
	}

	webServer, _ := uc.setting.Get(SettingKeyWebserver)
	return uc.repo.ObtainPanel(ctx, account, names, webServer)
}

func (uc *CertUsecase) ObtainSelfSigned(ctx context.Context, id uint) error {
	cert, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	crt, key, err := uc.repo.GenerateSelfSigned(cert.Domains)
	if err != nil {
		return err
	}

	cert.Cert = string(crt)
	cert.Key = string(key)
	if err = uc.repo.Save(cert); err != nil {
		return err
	}

	if len(cert.Websites) > 0 {
		if _, _, err = uc.deploy(ctx, cert.ID, cert.WebsiteIDs(), false); err != nil {
			return err
		}
	}

	if err = uc.repo.RunScript(ctx, cert); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("self-signed cert generated"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.Any("website_ids", cert.WebsiteIDs()))

	return nil
}

func (uc *CertUsecase) RenewWithProgressCallback(ctx context.Context, id uint, progressCallback func(string)) (*acme.Certificate, error) {
	report := func(msg string) {
		if progressCallback != nil {
			progressCallback(msg)
		}
	}

	report(uc.t.Get("preparing renewal"))
	cert, err := uc.repo.Get(id)
	if err != nil {
		return nil, err
	}

	client, err := uc.repo.GetClient(ctx, cert)
	if err != nil {
		return nil, err
	}

	if cert.CertURL == "" {
		return nil, errors.New(uc.t.Get("this certificate has not been obtained successfully and cannot be renewed"))
	}

	webServer, _ := uc.setting.Get(SettingKeyWebserver)

	if cert.DNS != nil {
		client.UseDns(cert.DNS.Type, cert.DNS.Data, acme.DnsOption{
			Alias:            cert.Alias,
			DnsServer:        cert.DNS.Data.DnsServer,
			SkipVerify:       cert.DNS.Data.SkipVerify,
			ProgressCallback: progressCallback,
		})
	} else {
		if len(cert.Websites) == 0 {
			return nil, errors.New(uc.t.Get("this certificate is not associated with a website and cannot be obtained. You can try to obtain it manually"))
		}
		for _, domain := range cert.Domains {
			if strings.Contains(domain, "*") {
				return nil, errors.New(uc.t.Get("wildcard domains cannot use HTTP verification"))
			}
		}
		d, err := webserver.Get(webserver.Type(webServer))
		if err != nil {
			return nil, err
		}
		confs, fallback := uc.repo.HTTPConfs(cert, webServer)
		client.UseHTTP(confs, fallback, d)
	}

	report(uc.t.Get("renewing certificate, domains: %s", strings.Join(cert.Domains, ", ")))
	// 超时只覆盖 ACME 交互，后续的部署不该被续签耗时挤占额度
	renewCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	ssl, err := client.RenewCertificate(renewCtx, cert.CertURL, cert.Domains, acme.KeyType(cert.Type))
	cancel()
	reissued := err != nil
	if reissued {
		// 续签失败，尝试重签，重签独立计时，避免续签耗时过长导致重签必定超时
		report(uc.t.Get("renewal failed, attempting re-issuance"))
		obtainCtx, obtainCancel := context.WithTimeout(ctx, 10*time.Minute)
		ssl, err = client.ObtainCertificate(obtainCtx, cert.Domains, acme.KeyType(cert.Type))
		obtainCancel()
		if err != nil {
			return nil, err
		}
	}

	report(uc.t.Get("obtaining and saving certificate"))
	cert.RenewalInfo = *ssl.RenewalInfo
	cert.CertURL = ssl.URL
	cert.Cert = string(ssl.ChainPEM)
	cert.Key = string(ssl.PrivateKey)
	if err = uc.repo.Save(cert); err != nil {
		return nil, err
	}

	if len(cert.Websites) > 0 {
		report(uc.t.Get("deploying certificate to website"))
		if _, _, err = uc.deploy(ctx, cert.ID, cert.WebsiteIDs(), false); err != nil {
			return nil, err
		}
	}

	if err = uc.repo.RunScript(ctx, cert); err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("cert renewed"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.Bool("reissued", reissued), slog.Any("website_ids", cert.WebsiteIDs()))

	return &ssl, nil
}

func (uc *CertUsecase) RefreshRenewalInfo(ctx context.Context, id uint) (mholtacme.RenewalInfo, error) {
	cert, err := uc.repo.Get(id)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}
	client, err := uc.repo.GetClient(ctx, cert)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	crt, err := pkgcert.ParseCert([]byte(cert.Cert))
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	infoCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	renewInfo, err := client.GetRenewalInfo(infoCtx, crt)
	if err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	cert.RenewalInfo = renewInfo
	if err = uc.repo.Save(cert); err != nil {
		return mholtacme.RenewalInfo{}, err
	}

	return renewInfo, nil
}

func (uc *CertUsecase) Deploy(ctx context.Context, id uint, websiteIDs []uint, enableHTTPS bool) error {
	cert, websites, err := uc.deploy(ctx, id, websiteIDs, enableHTTPS)
	if err != nil {
		return err
	}

	names := lo.Map(websites, func(website *Website, _ int) string {
		return website.Name
	})
	uc.log.Info(uc.t.Get("cert deployed"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.Any("websites", names), slog.Any("website_ids", websiteIDs), slog.Bool("enable_https", enableHTTPS))

	return nil
}

// deploy 不记日志，供签发、续签内部复用，避免一次签发产生两条操作日志
func (uc *CertUsecase) deploy(ctx context.Context, id uint, websiteIDs []uint, enableHTTPS bool) (*Cert, []*Website, error) {
	cert, err := uc.repo.Get(id)
	if err != nil {
		return nil, nil, err
	}

	if cert.Cert == "" || cert.Key == "" {
		return nil, nil, errors.New(uc.t.Get("this certificate has not been obtained successfully and cannot be deployed"))
	}

	websites, err := uc.repo.LoadWebsites(websiteIDs)
	if err != nil {
		return nil, nil, err
	}

	// 建立关联，使续签后能自动部署到这些网站
	if err = uc.repo.BindWebsites(cert.ID, websiteIDs); err != nil {
		return nil, nil, err
	}

	webServer, webServerErr := uc.setting.Get(SettingKeyWebserver)
	tlsVersions, _ := uc.setting.GetSlice(SettingKeyWebsiteTLSVersions)
	listenIPv6, err := uc.setting.GetBool(SettingKeyWebsiteListenIPv6, false)
	if err != nil {
		return nil, nil, err
	}

	for _, website := range websites {
		configDir := filepath.Join(app.Root, "sites", website.Name, "config")
		certPath := filepath.Join(configDir, "fullchain.pem")
		keyPath := filepath.Join(configDir, "private.key")
		if err = uc.repo.WriteCertFiles(cert, certPath, keyPath); err != nil {
			return nil, nil, err
		}

		// 开启 HTTPS
		if enableHTTPS && !website.SSL {
			if webServerErr != nil {
				return nil, nil, webServerErr
			}
			if err = uc.repo.EnableWebsiteSSL(website, certPath, keyPath, webServer, tlsVersions, listenIPv6); err != nil {
				return nil, nil, err
			}
		}
	}

	// 证书与 vhost 已落盘且不再回滚，reload 断开取消链，否则磁盘配置与运行中的配置会不一致
	if err = uc.repo.ReloadWebserver(context.WithoutCancel(ctx), webServer); err != nil {
		return nil, nil, err
	}

	return cert, websites, nil
}

func (uc *CertUsecase) Save(cert *Cert) error {
	return uc.repo.Save(cert)
}
