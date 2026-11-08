package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type ContainerComposeService struct {
	containerComposeRepo *biz.ContainerComposeUsecase
}

func NewContainerComposeService(containerComposeUsecase *biz.ContainerComposeUsecase) *ContainerComposeService {
	return &ContainerComposeService{
		containerComposeRepo: containerComposeUsecase,
	}
}

func (s *ContainerComposeService) List(w http.ResponseWriter, r *http.Request) {
	composes, err := s.containerComposeRepo.List(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, composes)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerComposeService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeGet](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	compose, envs, err := s.containerComposeRepo.Get(r.Context(), req.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"compose": compose,
		"envs":    envs,
	})
}

func (s *ContainerComposeService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Create(r.Context(), req.Name, req.Compose, req.Envs); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Update(r.Context(), req.Name, req.Compose, req.Envs); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Up(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeUp](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Up(r.Context(), req.Name, req.Force); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Down(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeDown](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Down(r.Context(), req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerComposeService) Remove(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerComposeRemove](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerComposeRepo.Remove(r.Context(), req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
