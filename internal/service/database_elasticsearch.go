package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseElasticsearchService struct {
	repo *biz.DatabaseElasticsearchUsecase
}

func NewDatabaseElasticsearchService(i do.Injector) (*DatabaseElasticsearchService, error) {
	return &DatabaseElasticsearchService{repo: do.MustInvoke[*biz.DatabaseElasticsearchUsecase](i)}, nil
}

func (s *DatabaseElasticsearchService) Indices(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESIndices](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	indices, err := s.repo.Indices(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, indices)
}

func (s *DatabaseElasticsearchService) IndexCreate(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESIndexCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.IndexCreate(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseElasticsearchService) IndexDelete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESIndexDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.IndexDelete(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseElasticsearchService) Data(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESData](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	items, total, err := s.repo.Data(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": items,
	})
}

func (s *DatabaseElasticsearchService) DocumentGet(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESDocumentGet](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	doc, err := s.repo.DocumentGet(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, doc)
}

func (s *DatabaseElasticsearchService) DocumentSet(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESDocumentSet](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.DocumentSet(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseElasticsearchService) DocumentDelete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseESDocumentDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.DocumentDelete(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
