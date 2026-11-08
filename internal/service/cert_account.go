package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type CertAccountService struct {
	t               *gotext.Locale
	log             *slog.Logger
	certAccountRepo *biz.CertAccountUsecase
}

func NewCertAccountService(certAccountUsecase *biz.CertAccountUsecase, t *gotext.Locale, log *slog.Logger) *CertAccountService {
	return &CertAccountService{
		t:               t,
		log:             log,
		certAccountRepo: certAccountUsecase,
	}
}

func (s *CertAccountService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	certDNS, total, err := s.certAccountRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cert account list viewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": certDNS,
	})
}

func (s *CertAccountService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertAccountCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := s.certAccountRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, account)
}

func (s *CertAccountService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.CertAccountUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.certAccountRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *CertAccountService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	account, err := s.certAccountRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("cert account viewed"), slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(account.ID)), slog.String("email", account.Email), slog.String("ca", account.CA))

	Success(w, account)
}

func (s *CertAccountService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.certAccountRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
