package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type CertDNSService struct {
	t           *gotext.Locale
	log         *slog.Logger
	certDNSRepo *biz.CertDNSUsecase
}

func NewCertDNSService(certDNSUsecase *biz.CertDNSUsecase, t *gotext.Locale, log *slog.Logger) *CertDNSService {
	return &CertDNSService{
		t:           t,
		log:         log,
		certDNSRepo: certDNSUsecase,
	}
}

func (s *CertDNSService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	certDNS, total, err := s.certDNSRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cert dns list viewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": certDNS,
	})
}

func (s *CertDNSService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertDNSCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	certDNS, err := s.certDNSRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, certDNS)
}

func (s *CertDNSService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertDNSUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.certDNSRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertDNSService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	certDNS, err := s.certDNSRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cert dns viewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(certDNS.ID)), slog.String("name", certDNS.Name), slog.String("dns_type", string(certDNS.Type)))

	Success(w, certDNS)
}

func (s *CertDNSService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.certDNSRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
