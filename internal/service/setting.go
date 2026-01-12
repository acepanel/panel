package service

import (
	"encoding/json"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/tools"
)

type SettingService struct {
	t               *gotext.Locale
	db              *gorm.DB
	settingRepo     biz.SettingRepo
	certRepo        biz.CertRepo
	certAccountRepo biz.CertAccountRepo
}

func NewSettingService(t *gotext.Locale, db *gorm.DB, setting biz.SettingRepo, cert biz.CertRepo, certAccount biz.CertAccountRepo) *SettingService {
	return &SettingService{
		t:               t,
		db:              db,
		settingRepo:     setting,
		certRepo:        cert,
		certAccountRepo: certAccount,
	}
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

	tools.RestartPanel()

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

	tools.RestartPanel()

	Success(w, nil)
}
