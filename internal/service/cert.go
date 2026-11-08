package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/acme"
	"github.com/acepanel/panel/v3/pkg/types"
)

type CertService struct {
	t        *gotext.Locale
	log      *slog.Logger
	certRepo *biz.CertUsecase
}

func NewCertService(certUsecase *biz.CertUsecase, t *gotext.Locale, log *slog.Logger) *CertService {
	return &CertService{
		t:        t,
		log:      log,
		certRepo: certUsecase,
	}
}

func (s *CertService) CAProviders(w http.ResponseWriter, r *http.Request) {
	Success(w, []types.LV{
		{
			Label: "Let's Encrypt",
			Value: "letsencrypt",
		},
		{
			Label: "GoogleCN",
			Value: "googlecn",
		},
		{
			Label: "LiteSSL",
			Value: "litessl",
		},
		{
			Label: "ZeroSSL",
			Value: "zerossl",
		},
		{
			Label: "SSL.com",
			Value: "sslcom",
		},
		{
			Label: "Google",
			Value: "google",
		},
	})
}

func (s *CertService) DNSProviders(w http.ResponseWriter, r *http.Request) {
	Success(w, []types.LV{
		{
			Label: s.t.Get("Aliyun"),
			Value: string(acme.AliYun),
		},
		{
			Label: s.t.Get("Tencent Cloud"),
			Value: string(acme.Tencent),
		},
		{
			Label: s.t.Get("Huawei Cloud"),
			Value: string(acme.Huawei),
		},
		{
			Label: s.t.Get("West.cn"),
			Value: string(acme.Westcn),
		},
		{
			Label: s.t.Get("CloudFlare"),
			Value: string(acme.CloudFlare),
		},
		{
			Label: s.t.Get("Gcore"),
			Value: string(acme.Gcore),
		},
		{
			Label: s.t.Get("Porkbun"),
			Value: string(acme.Porkbun),
		},
		{
			Label: s.t.Get("NameSilo"),
			Value: string(acme.NameSilo),
		},
		{
			Label: s.t.Get("ClouDNS"),
			Value: string(acme.ClouDNS),
		},
	})
}

func (s *CertService) Algorithms(w http.ResponseWriter, r *http.Request) {
	Success(w, []types.LV{
		{
			Label: "EC256",
			Value: string(acme.KeyEC256),
		},
		{
			Label: "EC384",
			Value: string(acme.KeyEC384),
		},
		{
			Label: "RSA2048",
			Value: string(acme.KeyRSA2048),
		},
		{
			Label: "RSA4096",
			Value: string(acme.KeyRSA4096),
		},
	})
}

func (s *CertService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	certs, total, err := s.certRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cert list viewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": certs,
	})
}

func (s *CertService) Upload(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertUpload](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cert, err := s.certRepo.Upload(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, cert)
}

func (s *CertService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cert, err := s.certRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, cert)
}

func (s *CertService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.certRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cert, err := s.certRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cert viewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(cert.ID)), slog.Any("domains", cert.Domains))

	Success(w, cert)
}

func (s *CertService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	err = s.certRepo.Delete(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertService) ObtainAuto(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if _, err = s.certRepo.ObtainAutoWithProgressCallback(r.Context(), req.ID, nil); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertService) ObtainSelfSigned(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.certRepo.ObtainSelfSigned(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertService) Renew(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	_, err = s.certRepo.RenewWithProgressCallback(r.Context(), req.ID, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertService) Deploy(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertDeploy](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	err = s.certRepo.Deploy(r.Context(), req.ID, req.WebsiteIDs, req.EnableHTTPS)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
