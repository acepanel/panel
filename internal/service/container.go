package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type ContainerService struct {
	containerRepo *biz.ContainerUsecase
}

func NewContainerService(i do.Injector) (*ContainerService, error) {
	return &ContainerService{
		containerRepo: do.MustInvoke[*biz.ContainerUsecase](i),
	}, nil
}

func (s *ContainerService) List(w http.ResponseWriter, r *http.Request) {
	containers, err := s.containerRepo.ListAll()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(r, containers)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerService) Search(w http.ResponseWriter, r *http.Request) {
	containers, err := s.containerRepo.ListByName(r.FormValue("name"))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": len(containers),
		"items": containers,
	})
}

func (s *ContainerService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	id, err := s.containerRepo.Create(req)
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

	if err = s.containerRepo.Remove(req.ID); err != nil {
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

	if err = s.containerRepo.Start(req.ID); err != nil {
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

	if err = s.containerRepo.Stop(req.ID); err != nil {
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

	if err = s.containerRepo.Restart(req.ID); err != nil {
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

	if err = s.containerRepo.Pause(req.ID); err != nil {
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

	if err = s.containerRepo.Unpause(req.ID); err != nil {
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

	if err = s.containerRepo.Kill(req.ID); err != nil {
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

	if err = s.containerRepo.Rename(req.ID, req.Name); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *ContainerService) Prune(w http.ResponseWriter, r *http.Request) {
	if err := s.containerRepo.Prune(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
