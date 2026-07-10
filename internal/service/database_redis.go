package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type DatabaseRedisService struct {
	repo *biz.DatabaseRedisUsecase
}

func NewDatabaseRedisService(i do.Injector) (*DatabaseRedisService, error) {
	return &DatabaseRedisService{repo: do.MustInvoke[*biz.DatabaseRedisUsecase](i)}, nil
}

func (s *DatabaseRedisService) Databases(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisDatabases](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	count, err := s.repo.Databases(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, count)
}

func (s *DatabaseRedisService) Data(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisData](r)
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

func (s *DatabaseRedisService) KeyGet(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisKeyGet](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	kv, err := s.repo.KeyGet(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, kv)
}

func (s *DatabaseRedisService) KeySet(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisKeySet](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.KeySet(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseRedisService) KeyDelete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisKeyDelete](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.KeyDelete(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseRedisService) KeyTTL(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisKeyTTL](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.KeyTTL(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseRedisService) KeyRename(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisKeyRename](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.KeyRename(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *DatabaseRedisService) Clear(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.DatabaseRedisClear](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.repo.Clear(req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
