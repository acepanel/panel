package service

import (
	"crypto/x509"
	"log/slog"
	"net"
	"net/http"
	"slices"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type SettingService struct {
	t               *gotext.Locale
	log             *slog.Logger
	db              *gorm.DB
	settingRepo     *biz.SettingUsecase
	certRepo        *biz.CertUsecase
	certAccountRepo *biz.CertAccountUsecase
}

func NewSettingService(certAccountUsecase *biz.CertAccountUsecase, certUsecase *biz.CertUsecase, settingUsecase *biz.SettingUsecase, db *gorm.DB, t *gotext.Locale, log *slog.Logger) *SettingService {
	return &SettingService{
		t:               t,
		log:             log,
		db:              db,
		settingRepo:     settingUsecase,
		certRepo:        certUsecase,
		certAccountRepo: certAccountUsecase,
	}
}

func (s *SettingService) Get(w http.ResponseWriter, r *http.Request) {
	setting, err := s.settingRepo.GetPanel()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("panel settings viewed"), slog.String("type", biz.OperationTypeSetting), slog.Uint64("operator_id", biz.OperatorID(r.Context())))

	Success(w, setting)
}

func (s *SettingService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SettingPanel](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	restart, err := s.settingRepo.UpdatePanel(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if restart {
		tools.RestartPanel(r.Context())
	}

	Success(w, chix.M{
		"restart": restart,
	})
}

func (s *SettingService) ObtainCert(w http.ResponseWriter, r *http.Request) {
	// 自签模式
	conf, err := config.Load()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if conf.HTTP.TLS == "self-signed" {
		//nolint:contextcheck
		crt, key, err := cert.GenerateSelfSigned(tools.CollectLocalNames())
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		if err = s.settingRepo.UpdateCert(&request.SettingCert{
			Cert: string(crt),
			Key:  string(key),
		}); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		info, _ := cert.ParseCert(crt)
		s.log.Info(s.t.Get("panel certificate obtained"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("tls", conf.HTTP.TLS), slog.Any("domains", certNames(info)))
		Success(w, nil)
		return
	}

	// ACME 模式
	var user biz.User
	if err = s.db.First(&user).Error; err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	account, err := s.certAccountRepo.GetDefault(r.Context(), user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	crt, key, err := s.certRepo.ObtainPanel(r.Context(), account, conf.HTTP.BindDomain)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.settingRepo.UpdateCert(&request.SettingCert{
		Cert: string(crt),
		Key:  string(key),
	}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	info, _ := cert.ParseCert(crt)
	s.log.Info(s.t.Get("panel certificate obtained"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("tls", conf.HTTP.TLS), slog.Any("domains", certNames(info)))

	Success(w, nil)
}

// UpdateCert 用于自动化工具更新证书
func (s *SettingService) UpdateCert(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SettingCert](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.settingRepo.UpdateCert(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	info, _ := cert.ParseCert([]byte(req.Cert))
	s.log.Info(s.t.Get("panel certificate updated"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Any("domains", certNames(info)), slog.Time("not_after", info.NotAfter))

	Success(w, nil)
}

// GetMemo 获取便签内容
func (s *SettingService) GetMemo(w http.ResponseWriter, r *http.Request) {
	memo, err := s.settingRepo.Get(biz.SettingKeyMemo)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("memo viewed"), slog.String("type", biz.OperationTypeSetting), slog.Uint64("operator_id", biz.OperatorID(r.Context())))

	Success(w, memo)
}

// UpdateMemo 更新便签内容
func (s *SettingService) UpdateMemo(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SettingMemo](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.settingRepo.Set(biz.SettingKeyMemo, req.Content); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("memo updated"), slog.String("type", biz.OperationTypeSetting), slog.Uint64("operator_id", biz.OperatorID(r.Context())))

	Success(w, nil)
}

func certNames(info x509.Certificate) []string {
	return slices.Concat(info.DNSNames, lo.Map(info.IPAddresses, func(ip net.IP, _ int) string {
		return ip.String()
	}))
}
