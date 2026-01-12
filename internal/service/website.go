package service

import (
	"net/http"
	"path/filepath"

	"github.com/libtnb/chix"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/io"
)

type WebsiteService struct {
	websiteRepo biz.WebsiteRepo
	settingRepo biz.SettingRepo
}

func NewWebsiteService(website biz.WebsiteRepo, setting biz.SettingRepo) *WebsiteService {
	return &WebsiteService{
		websiteRepo: website,
		settingRepo: setting,
	}
}

func (s *WebsiteService) GetRewrites(w http.ResponseWriter, r *http.Request) {
	rewrites, err := s.websiteRepo.GetRewrites()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, rewrites)
}

func (s *WebsiteService) GetDefaultConfig(w http.ResponseWriter, r *http.Request) {
	index, _ := io.Read(filepath.Join(app.Root, "server/nginx/html/index.html"))
	stop, _ := io.Read(filepath.Join(app.Root, "server/nginx/html/stop.html"))
	notFound, _ := io.Read(filepath.Join(app.Root, "server/nginx/html/404.html"))
	tlsVersions, _ := s.settingRepo.GetSlice(biz.SettingKeyWebsiteTLSVersions)
	cipherSuites, _ := s.settingRepo.Get(biz.SettingKeyWebsiteCipherSuites)

	Success(w, chix.M{
		"index":         index,
		"stop":          stop,
		"not_found":     notFound,
		"tls_versions":  tlsVersions,
		"cipher_suites": cipherSuites,
	})
}

func (s *WebsiteService) UpdateDefaultConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteDefaultConfig](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateDefaultConfig(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// UpdateCert 用于自动化工具更新证书
func (s *WebsiteService) UpdateCert(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateCert](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateCert(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// List 网站列表
func (s *WebsiteService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	websites, total, err := s.websiteRepo.List(req.Type, req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": websites,
	})
}

func (s *WebsiteService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if len(req.Path) == 0 {
		req.Path, _ = s.settingRepo.Get(biz.SettingKeyWebsitePath)
		req.Path = filepath.Join(req.Path, req.Name, "public")
	}

	if _, err = s.websiteRepo.Create(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := s.websiteRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, config)
}

func (s *WebsiteService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.Delete(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) ClearLog(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.ClearLog(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) UpdateRemark(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateRemark](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateRemark(req.ID, req.Remark); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) ResetConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.ResetConfig(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebsiteUpdateStatus](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.UpdateStatus(req.ID, req.Status); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebsiteService) ObtainCert(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.websiteRepo.ObtainCert(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
