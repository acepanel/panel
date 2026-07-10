package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseServerService struct {
	databaseServerRepo *biz.DatabaseServerUsecase
}

func NewDatabaseServerService(i do.Injector) (*DatabaseServerService, error) {
	return &DatabaseServerService{
		databaseServerRepo: do.MustInvoke[*biz.DatabaseServerUsecase](i),
	}, nil
}

func (s *DatabaseServerService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	servers, total, err := s.databaseServerRepo.List(req.Page, req.Limit, req.Type)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	if err = s.databaseServerRepo.Create(req); err != nil {
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

	server, err := s.databaseServerRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, server)
}

func (s *DatabaseServerService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseServerUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.databaseServerRepo.Update(req); err != nil {
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

	if err = s.databaseServerRepo.UpdateRemark(req); err != nil {
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

	if err = s.databaseServerRepo.Delete(req.ID); err != nil {
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

	if err = s.databaseServerRepo.Sync(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
