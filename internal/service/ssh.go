package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type SSHService struct {
	sshRepo biz.SSHRepo
}

func NewSSHService(ssh biz.SSHRepo) *SSHService {
	return &SSHService{
		sshRepo: ssh,
	}
}

func (s *SSHService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	cron, total, err := s.sshRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": cron,
	})
}

func (s *SSHService) Create(c fiber.Ctx) error {
	req, err := Bind[request.SSHCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.sshRepo.Create(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *SSHService) Update(c fiber.Ctx) error {
	req, err := Bind[request.SSHUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.sshRepo.Update(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *SSHService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	cron, err := s.sshRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, cron)
}

func (s *SSHService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.sshRepo.Delete(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
