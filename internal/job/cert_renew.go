package job

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	pkgcert "github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tools"
)

// CertRenew 证书续签
type CertRenew struct {
	conf            *config.Config
	db              *gorm.DB
	log             *slog.Logger
	settingRepo     *biz.SettingUsecase
	certRepo        *biz.CertUsecase
	certAccountRepo *biz.CertAccountUsecase
	notifyRepo      *biz.NotifyUsecase
	t               *gotext.Locale
}

// NewCertRenew 构造证书续签任务
func NewCertRenew(certAccountUsecase *biz.CertAccountUsecase, certUsecase *biz.CertUsecase, notifyUsecase *biz.NotifyUsecase, settingUsecase *biz.SettingUsecase, conf *config.Config, db *gorm.DB, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "0 4 * * *",
		Task: &CertRenew{
			conf:            conf,
			db:              db,
			log:             log,
			settingRepo:     settingUsecase,
			certRepo:        certUsecase,
			certAccountRepo: certAccountUsecase,
			notifyRepo:      notifyUsecase,
			t:               t,
		},
	}
}

func (r *CertRenew) Run(ctx context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	var certs []biz.Cert
	if err := r.db.Preload("Websites").Preload("Account").Preload("DNS").Find(&certs).Error; err != nil {
		r.log.Warn(r.t.Get("failed to get certs"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return nil
	}

	for _, cert := range certs {
		// 面板重启会取消 ctx，剩余证书只会瞬间失败，继续跑只是刷出一串假告警
		if err := ctx.Err(); err != nil {
			return err
		}

		// 跳过上传类型或未开启自动续签的证书
		if cert.Type == "upload" || !cert.AutoRenewal {
			continue
		}

		// 刷新续签信息
		if cert.RenewalInfo.NeedsRefresh() {
			renewInfo, err := r.certRepo.RefreshRenewalInfo(ctx, cert.ID)
			if err != nil {
				r.log.Warn(r.t.Get("failed to refresh renewal info"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.Any("err", err))
				continue
			}
			cert.RenewalInfo = renewInfo
		}

		// 到达建议时间，续签证书
		if time.Now().After(cert.RenewalInfo.SelectedTime) {
			if _, err := r.certRepo.RenewWithProgressCallback(ctx, cert.ID, nil); err != nil {
				r.log.Warn(r.t.Get("failed to renew certificate"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains), slog.Any("err", err))
				// ctx 取消不是证书本身的问题，告警了也只是噪音
				if ctx.Err() == nil {
					r.notifyFailed(strings.Join(cert.Domains, ", "), err)
				}
			}
		}
	}

	// 面板证书续签
	if err := ctx.Err(); err != nil {
		return err
	}
	switch r.conf.HTTP.TLS {
	case "self-signed":
		// 自签证书续签
		crt, _ := os.ReadFile(filepath.Join(app.Root, "panel/storage/cert.pem"))
		decode, err := pkgcert.ParseCert(crt)
		if err == nil {
			if time.Until(decode.NotAfter) > 30*24*time.Hour {
				return nil
			}
		} else {
			r.log.Warn(r.t.Get("failed to parse panel certificate"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
		}

		newCrt, newKey, err := pkgcert.GenerateSelfSigned(tools.CollectLocalNames()) //nolint:contextcheck
		if err != nil {
			r.log.Warn(r.t.Get("failed to generate self-signed certificate"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			r.notifyFailed(r.t.Get("panel certificate"), err)
			return nil
		}
		if err = r.settingRepo.UpdateCert(&request.SettingCert{
			Cert: string(newCrt),
			Key:  string(newKey),
		}); err != nil {
			r.log.Warn(r.t.Get("failed to update panel certificate"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			r.notifyFailed(r.t.Get("panel certificate"), err)
			return nil
		}
		r.log.Info(r.t.Get("panel self-signed certificate renewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0))

	case "off", "custom":
		// off/custom 不需要自动续签

	default:
		// ACME 模式
		crt, _ := os.ReadFile(filepath.Join(app.Root, "panel/storage/cert.pem"))
		decode, err := pkgcert.ParseCert(crt)
		if err == nil {
			renewBefore := 2 * 24 * time.Hour
			if len(r.conf.HTTP.BindDomain) > 0 {
				renewBefore = 30 * 24 * time.Hour
			}
			if time.Until(decode.NotAfter) > renewBefore {
				return nil
			}
		} else {
			// 解析失败则继续续签流程，可能是证书格式不对或者文件不存在
			r.log.Warn(r.t.Get("failed to parse panel certificate"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
		}

		var user biz.User
		if err = r.db.First(&user).Error; err != nil {
			r.log.Warn(r.t.Get("failed to get a panel user"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			return nil
		}
		account, err := r.certAccountRepo.GetDefault(ctx, user.ID)
		if err != nil {
			r.log.Warn(r.t.Get("failed to get panel ACME account"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			r.notifyFailed(r.t.Get("panel certificate"), err)
			return nil
		}
		crt, key, err := r.certRepo.ObtainPanel(ctx, account, r.conf.HTTP.BindDomain)
		if err != nil {
			r.log.Warn(r.t.Get("failed to obtain panel certificate via ACME"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			r.notifyFailed(r.t.Get("panel certificate"), err)
			return nil
		}

		if err = r.settingRepo.UpdateCert(&request.SettingCert{
			Cert: string(crt),
			Key:  string(key),
		}); err != nil {
			r.log.Warn(r.t.Get("failed to update panel certificate"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Any("err", err))
			r.notifyFailed(r.t.Get("panel certificate"), err)
			return nil
		}

		r.log.Info(r.t.Get("panel certificate renewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0))
	}

	return nil
}

// notifyFailed 上报证书续签失败
func (r *CertRenew) notifyFailed(target string, err error) {
	r.notifyRepo.SendEvent(biz.NotifyEventCertRenew, r.t.Get("[AcePanel] Certificate Renewal Failed"), biz.NotifyBody(r.t.Get("certificate renewal failed"), [][2]string{
		{r.t.Get("Certificate"), target},
		{r.t.Get("Error"), err.Error()},
		{r.t.Get("Time"), time.Now().Format(time.DateTime)},
	}))
}
