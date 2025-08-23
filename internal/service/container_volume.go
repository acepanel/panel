package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type ContainerVolumeService struct {
	containerVolumeRepo biz.ContainerVolumeRepo
}

func NewContainerVolumeService(containerVolume biz.ContainerVolumeRepo) *ContainerVolumeService {
	return &ContainerVolumeService{
		containerVolumeRepo: containerVolume,
	}
}

func (s *ContainerVolumeService) List(c fiber.Ctx) error {
	volumes, err := s.containerVolumeRepo.List()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(c, volumes)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerVolumeService) Create(c fiber.Ctx) error {
	req, err := Bind[request.ContainerVolumeCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	name, err := s.containerVolumeRepo.Create(req)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, name)

}

func (s *ContainerVolumeService) Remove(c fiber.Ctx) error {
	req, err := Bind[request.ContainerVolumeID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerVolumeRepo.Remove(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerVolumeService) Prune(c fiber.Ctx) error {
	if err := s.containerVolumeRepo.Prune(); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
