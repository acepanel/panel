package service

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type ContainerService struct {
	containerRepo *biz.ContainerUsecase
}

func NewContainerService(containerUsecase *biz.ContainerUsecase) *ContainerService {
	return &ContainerService{
		containerRepo: containerUsecase,
	}
}

func (s *ContainerService) List(w http.ResponseWriter, r *http.Request) {
	containers, err := s.containerRepo.List(r.Context(), r.URL.Query().Get("keyword"))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, containers)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerService) Inspect(w http.ResponseWriter, r *http.Request) {
	data, err := s.containerRepo.Inspect(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, data)
}

func (s *ContainerService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	idParam := chi.URLParam(r, "id")
	if req.Background {
		if err = s.containerRepo.UpdateBackground(r.Context(), idParam, req); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		Success(w, nil)
		return
	}

	id, err := s.containerRepo.Update(r.Context(), idParam, req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, id)
}

func (s *ContainerService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if req.Background {
		if err = s.containerRepo.CreateBackground(r.Context(), req); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		Success(w, nil)
		return
	}

	id, err := s.containerRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, id)
}

func (s *ContainerService) Remove(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Remove(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Start(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Start(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Stop(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Stop(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Restart(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Restart(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Pause(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Pause(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Unpause(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Unpause(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Kill(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Kill(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Rename(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerRename](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.containerRepo.Rename(r.Context(), req.ID, req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Prune(w http.ResponseWriter, r *http.Request) {
	if err := s.containerRepo.Prune(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
