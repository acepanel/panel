package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type ContainerNetworkService struct {
	containerNetworkRepo biz.ContainerNetworkRepo
}

func NewContainerNetworkService(containerNetwork biz.ContainerNetworkRepo) *ContainerNetworkService {
	return &ContainerNetworkService{
		containerNetworkRepo: containerNetwork,
	}
}

func (s *ContainerNetworkService) List(c fiber.Ctx) error {
	networks, err := s.containerNetworkRepo.List()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(c, networks)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerNetworkService) Create(c fiber.Ctx) error {
	req, err := Bind[request.ContainerNetworkCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	id, err := s.containerNetworkRepo.Create(req)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, id)
}

func (s *ContainerNetworkService) Remove(c fiber.Ctx) error {
	req, err := Bind[request.ContainerNetworkID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerNetworkRepo.Remove(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerNetworkService) Prune(c fiber.Ctx) error {
	if err := s.containerNetworkRepo.Prune(); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
