package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type ContainerNetworkService struct {
	containerNetworkRepo *biz.ContainerNetworkUsecase
}

func NewContainerNetworkService(i do.Injector) (*ContainerNetworkService, error) {
	return &ContainerNetworkService{
		containerNetworkRepo: do.MustInvoke[*biz.ContainerNetworkUsecase](i),
	}, nil
}

func (s *ContainerNetworkService) List(w http.ResponseWriter, r *http.Request) {
	networks, err := s.containerNetworkRepo.List()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, networks)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerNetworkService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerNetworkCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	id, err := s.containerNetworkRepo.Create(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, id)
}

func (s *ContainerNetworkService) Remove(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerNetworkID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerNetworkRepo.Remove(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerNetworkService) Prune(w http.ResponseWriter, r *http.Request) {
	if err := s.containerNetworkRepo.Prune(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
