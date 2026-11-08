package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseServerService struct {
	t                  *gotext.Locale
	log                *slog.Logger
	databaseServerRepo *biz.DatabaseServerUsecase
}

func NewDatabaseServerService(databaseServerUsecase *biz.DatabaseServerUsecase, t *gotext.Locale, log *slog.Logger) *DatabaseServerService {
	return &DatabaseServerService{
		t:                  t,
		log:                log,
		databaseServerRepo: databaseServerUsecase,
	}
}

func (s *DatabaseServerService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	servers, total, err := s.databaseServerRepo.List(r.Context(), req.Page, req.Limit, req.Type)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("database server list viewed"), slog.String("type", biz.OperationTypeDatabaseServer), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("server_type", req.Type), slog.Uint64("page", uint64(req.Page)))

	Success(w, chix.M{
		"total": total,
		"items": servers,
	})
}

func (s *DatabaseServerService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseServerCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseServerRepo.Create(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseServerService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	server, err := s.databaseServerRepo.Get(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("database server viewed"), slog.String("type", biz.OperationTypeDatabaseServer), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(server.ID)), slog.String("name", server.Name))

	Success(w, server)
}

func (s *DatabaseServerService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseServerUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseServerRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseServerService) UpdateRemark(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseServerUpdateRemark](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseServerRepo.UpdateRemark(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseServerService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseServerRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseServerService) Sync(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseServerRepo.Sync(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
