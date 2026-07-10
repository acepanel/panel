package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type ContainerVolumeService struct {
	containerVolumeRepo *biz.ContainerVolumeUsecase
}

func NewContainerVolumeService(i do.Injector) (*ContainerVolumeService, error) {
	return &ContainerVolumeService{
		containerVolumeRepo: do.MustInvoke[*biz.ContainerVolumeUsecase](i),
	}, nil
}

func (s *ContainerVolumeService) List(w http.ResponseWriter, r *http.Request) {
	volumes, err := s.containerVolumeRepo.List()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, volumes)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerVolumeService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerVolumeCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name, err := s.containerVolumeRepo.Create(req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, name)

}

func (s *ContainerVolumeService) Remove(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerVolumeID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerVolumeRepo.Remove(req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerVolumeService) Prune(w http.ResponseWriter, r *http.Request) {
	if err := s.containerVolumeRepo.Prune(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
