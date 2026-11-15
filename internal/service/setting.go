package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type SettingService struct {
	t               *gotext.Locale
	log             *slog.Logger
	settingRepo     *biz.SettingUsecase
	userRepo        *biz.UserUsecase
	certRepo        *biz.CertUsecase
	certAccountRepo *biz.CertAccountUsecase
}

func NewSettingService(
	certAccountUsecase *biz.CertAccountUsecase,
	certUsecase *biz.CertUsecase,
	settingUsecase *biz.SettingUsecase,
	userUsecase *biz.UserUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *SettingService {
	return &SettingService{
		t:               t,
		log:             log,
		settingRepo:     settingUsecase,
		userRepo:        userUsecase,
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
		s.log.Info(s.t.Get("panel certificate obtained"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("tls", conf.HTTP.TLS), slog.Any("domains", cert.SANs(info)))
		Success(w, nil)
		return
	}

	// ACME 模式
	crt, key, err := obtainPanelACME(r.Context(), s.t, s.userRepo, s.certAccountRepo, s.certRepo, conf.HTTP.BindDomain)
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
	s.log.Info(s.t.Get("panel certificate obtained"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("tls", conf.HTTP.TLS), slog.Any("domains", cert.SANs(info)))

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
	s.log.Info(s.t.Get("panel certificate updated"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Any("domains", cert.SANs(info)), slog.Time("not_after", info.NotAfter))

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

// obtainPanelACME 用首个用户的默认 ACME 账户为面板签发证书
func obtainPanelACME(
	ctx context.Context,
	t *gotext.Locale,
	userRepo *biz.UserUsecase,
	certAccountRepo *biz.CertAccountUsecase,
	certRepo *biz.CertUsecase,
	domains []string,
) ([]byte, []byte, error) {
	user, err := userRepo.First()
	if err != nil {
		return nil, nil, errors.New(t.Get("Failed to get a panel user: %v", err))
	}
	account, err := certAccountRepo.GetDefault(ctx, user.ID)
	if err != nil {
		return nil, nil, errors.New(t.Get("Failed to get ACME account: %v", err))
	}
	return certRepo.ObtainPanel(ctx, account, domains)
}
