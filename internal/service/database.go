package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseService struct {
	databaseRepo *biz.DatabaseUsecase
}

func NewDatabaseService(i do.Injector) (*DatabaseService, error) {
	return &DatabaseService{
		databaseRepo: do.MustInvoke[*biz.DatabaseUsecase](i),
	}, nil
}

func (s *DatabaseService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	databases, total, err := s.databaseRepo.List(req.Page, req.Limit, req.Type)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	if err = s.databaseRepo.Comment(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
