package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type ContainerService struct {
	containerRepo biz.ContainerRepo
}

func NewContainerService(container biz.ContainerRepo) *ContainerService {
	return &ContainerService{
		containerRepo: container,
	}
}

func (s *ContainerService) List(c fiber.Ctx) error {
	containers, err := s.containerRepo.ListAll()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(r, containers)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerService) Search(c fiber.Ctx) error {
	containers, err := s.containerRepo.ListByName(r.FormValue("name"))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": len(containers),
		"items": containers,
	})
}

func (s *ContainerService) Create(c fiber.Ctx) error {
	req, err := Bind[request.ContainerCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	id, err := s.containerRepo.Create(req)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, id)
}

func (s *ContainerService) Remove(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Remove(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Start(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Start(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Stop(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Stop(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Restart(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Restart(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Pause(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Pause(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Unpause(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Unpause(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Kill(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Kill(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Rename(c fiber.Ctx) error {
	req, err := Bind[request.ContainerRename](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerRepo.Rename(req.ID, req.Name); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerService) Logs(c fiber.Ctx) error {
	req, err := Bind[request.ContainerID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	logs, err := s.containerRepo.Logs(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, logs)
}

func (s *ContainerService) Prune(c fiber.Ctx) error {
	if err := s.containerRepo.Prune(); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
