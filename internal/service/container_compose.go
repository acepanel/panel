package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type ContainerComposeService struct {
	containerComposeRepo biz.ContainerComposeRepo
}

func NewContainerComposeService(containerCompose biz.ContainerComposeRepo) *ContainerComposeService {
	return &ContainerComposeService{
		containerComposeRepo: containerCompose,
	}
}

func (s *ContainerComposeService) List(c fiber.Ctx) error {
	composes, err := s.containerComposeRepo.List()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(r, composes)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerComposeService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ContainerComposeGet](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	compose, envs, err := s.containerComposeRepo.Get(req.Name)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"compose": compose,
		"envs":    envs,
	})
}

func (s *ContainerComposeService) Create(c fiber.Ctx) error {
	req, err := Bind[request.ContainerComposeCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerComposeRepo.Create(req.Name, req.Compose, req.Envs); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerComposeService) Update(c fiber.Ctx) error {
	req, err := Bind[request.ContainerComposeUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerComposeRepo.Update(req.Name, req.Compose, req.Envs); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerComposeService) Up(c fiber.Ctx) error {
	req, err := Bind[request.ContainerComposeUp](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerComposeRepo.Up(req.Name, req.Force); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerComposeService) Down(c fiber.Ctx) error {
	req, err := Bind[request.ContainerComposeDown](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerComposeRepo.Down(req.Name); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerComposeService) Remove(c fiber.Ctx) error {
	req, err := Bind[request.ContainerComposeRemove](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerComposeRepo.Remove(req.Name); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
