package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseUserService struct {
	t                *gotext.Locale
	log              *slog.Logger
	databaseUserRepo *biz.DatabaseUserUsecase
}

func NewDatabaseUserService(databaseUserUsecase *biz.DatabaseUserUsecase, t *gotext.Locale, log *slog.Logger) *DatabaseUserService {
	return &DatabaseUserService{
		t:                t,
		log:              log,
		databaseUserRepo: databaseUserUsecase,
	}
}

func (s *DatabaseUserService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	users, total, err := s.databaseUserRepo.List(r.Context(), req.Page, req.Limit, req.Type)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("database user list viewed"), slog.String("type", biz.OperationTypeDatabaseUser), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("server_type", req.Type), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": users,
	})
}

func (s *DatabaseUserService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseUserCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseUserRepo.Create(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseUserService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	user, err := s.databaseUserRepo.Get(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("database user viewed"), slog.String("type", biz.OperationTypeDatabaseUser), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("id", uint64(req.ID)), slog.String("username", user.Username), slog.String("host", user.Host), slog.Uint64("server_id", uint64(user.ServerID)))

	Success(w, user)
}

func (s *DatabaseUserService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseUserUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseUserRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseUserService) UpdateRemark(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseUserUpdateRemark](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseUserRepo.UpdateRemark(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseUserService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseUserRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
