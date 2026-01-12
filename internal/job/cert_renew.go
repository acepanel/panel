package job

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	pkgcert "github.com/acepanel/panel/pkg/cert"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/tools"
)

// CertRenew 证书续签
type CertRenew struct {
	conf            *config.Config
	db              *gorm.DB
	log             *slog.Logger
	settingRepo     biz.SettingRepo
	certRepo        biz.CertRepo
	certAccountRepo biz.CertAccountRepo
}

func NewCertRenew(conf *config.Config, db *gorm.DB, log *slog.Logger, setting biz.SettingRepo, cert biz.CertRepo, certAccount biz.CertAccountRepo) *CertRenew {
	return &CertRenew{
		conf:            conf,
		db:              db,
		log:             log,
		settingRepo:     setting,
		certRepo:        cert,
		certAccountRepo: certAccount,
	}
}

func (r *CertRenew) Run() {
	if app.Status != app.StatusNormal {
		return
	}

	var certs []biz.Cert
	if err := r.db.Preload("Website").Preload("Account").Preload("DNS").Find(&certs).Error; err != nil {
		r.log.Warn("failed to get certs", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return
	}

	for _, cert := range certs {
		// 跳过上传类型或未开启自动续签的证书
		if cert.Type == "upload" || !cert.AutoRenewal {
			continue
		}

		// 刷新续签信息
		if cert.RenewalInfo.NeedsRefresh() {
			renewInfo, err := r.certRepo.RefreshRenewalInfo(cert.ID)
			if err != nil {
				r.log.Warn("failed to refresh renewal info", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
				continue
			}
			cert.RenewalInfo = renewInfo
		}

		// 到达建议时间，续签证书
		if time.Now().After(cert.RenewalInfo.SelectedTime) {
			if _, err := r.certRepo.Renew(cert.ID); err != nil {
				r.log.Warn("failed to renew cert", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			}
		}
	}

	// 面板证书续签
	if r.conf.HTTP.ACME {
		decode, err := pkgcert.ParseCert(filepath.Join(app.Root, "panel/storage/cert.pem"))
		if err != nil {
			r.log.Warn("failed to parse panel cert", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}
		// 结束时间大于 2 天不续签
		if time.Until(decode.NotAfter) > 24*2*time.Hour {
			return
		}

		ip, err := r.settingRepo.Get(biz.SettingKeyPublicIPs)
		if err != nil {
			r.log.Warn("failed to get panel IP", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}
		var ips []string
		if err = json.Unmarshal([]byte(ip), &ips); err != nil || len(ips) == 0 {
			r.log.Warn("panel public IPs not set", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}

		var user biz.User
		if err = r.db.First(&user).Error; err != nil {
			r.log.Warn("failed to get a panel user", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}
		account, err := r.certAccountRepo.GetDefault(user.ID)
		if err != nil {
			r.log.Warn("failed to get panel ACME account", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}
		crt, key, err := r.certRepo.ObtainPanel(account, ips)
		if err != nil {
			r.log.Warn("failed to obtain ACME cert", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}

		if err = r.settingRepo.UpdateCert(&request.SettingCert{
			Cert: string(crt),
			Key:  string(key),
		}); err != nil {
			r.log.Warn("failed to update panel cert", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return
		}

		r.log.Info("panel cert renewed successfully", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0))
		tools.RestartPanel()
	}

}
