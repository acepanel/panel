package service

import (
	"encoding/json"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type SettingService struct {
	t               *gotext.Locale
	db              *gorm.DB
	settingRepo     *biz.SettingUsecase
	certRepo        *biz.CertUsecase
	certAccountRepo *biz.CertAccountUsecase
}

func NewSettingService(i do.Injector) (*SettingService, error) {
	return &SettingService{
		t:               do.MustInvoke[*gotext.Locale](i),
		db:              do.MustInvoke[*gorm.DB](i),
		settingRepo:     do.MustInvoke[*biz.SettingUsecase](i),
		certRepo:        do.MustInvoke[*biz.CertUsecase](i),
		certAccountRepo: do.MustInvoke[*biz.CertAccountUsecase](i),
	}, nil
}

func (s *SettingService) Get(w http.ResponseWriter, r *http.Request) {
	setting, err := s.settingRepo.GetPanel()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, setting)
}

func (s *SettingService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.SettingPanel](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	restart := false
	if restart, err = s.settingRepo.UpdatePanel(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if restart {
		tools.RestartPanel()
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
		Success(w, nil)
		return
	}

	// ACME 模式
	ip, err := s.settingRepo.Get(biz.SettingKeyPublicIPs)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	var ips []string
	if err = json.Unmarshal([]byte(ip), &ips); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if len(ips) == 0 {
		Error(w, http.StatusBadRequest, s.t.Get("please set public ips first"))
		return
	}

	var user biz.User
	if err = s.db.First(&user).Error; err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	account, err := s.certAccountRepo.GetDefault(user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	crt, key, err := s.certRepo.ObtainPanel(account, ips)
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

	Success(w, nil)
}

// GetMemo 获取便签内容
func (s *SettingService) GetMemo(w http.ResponseWriter, r *http.Request) {
	memo, err := s.settingRepo.Get(biz.SettingKeyMemo)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	Success(w, nil)
}
