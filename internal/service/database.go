package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseService struct {
	t            *gotext.Locale
	log          *slog.Logger
	databaseRepo *biz.DatabaseUsecase
}

func NewDatabaseService(databaseUsecase *biz.DatabaseUsecase, t *gotext.Locale, log *slog.Logger) *DatabaseService {
	return &DatabaseService{
		t:            t,
		log:          log,
		databaseRepo: databaseUsecase,
	}
}

func (s *DatabaseService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	databases, total, err := s.databaseRepo.List(r.Context(), req.Page, req.Limit, req.Type, req.Keyword)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("database list viewed"), slog.String("type", biz.OperationTypeDatabase), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("server_type", req.Type), slog.String("keyword", req.Keyword), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": databases,
	})
}

func (s *DatabaseService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseRepo.Create(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseRepo.Delete(r.Context(), req.ServerID, req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseService) Comment(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseComment](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseRepo.Comment(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
