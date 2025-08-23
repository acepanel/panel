package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type ContainerImageService struct {
	containerImageRepo biz.ContainerImageRepo
}

func NewContainerImageService(containerImage biz.ContainerImageRepo) *ContainerImageService {
	return &ContainerImageService{
		containerImageRepo: containerImage,
	}
}

func (s *ContainerImageService) List(c fiber.Ctx) error {
	images, err := s.containerImageRepo.List()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(r, images)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *ContainerImageService) Pull(c fiber.Ctx) error {
	req, err := Bind[request.ContainerImagePull](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerImageRepo.Pull(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerImageService) Remove(c fiber.Ctx) error {
	req, err := Bind[request.ContainerImageID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.containerImageRepo.Remove(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *ContainerImageService) Prune(c fiber.Ctx) error {
	if err := s.containerImageRepo.Prune(); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
