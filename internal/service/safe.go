package service

import (
	"github.com/gofiber/fiber/v2"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type SafeService struct {
	safeRepo biz.SafeRepo
}

func NewSafeService(safe biz.SafeRepo) *SafeService {
	return &SafeService{
		safeRepo: safe,
	}
}

func (s *SafeService) GetSSH(c fiber.Ctx) error {
	port, status, err := s.safeRepo.GetSSH()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	return Success(c, chix.M{
		"port":   port,
		"status": status,
	})
}

func (s *SafeService) UpdateSSH(c fiber.Ctx) error {
	req, err := Bind[request.SafeUpdateSSH](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.safeRepo.UpdateSSH(req.Port, req.Status); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *SafeService) GetPingStatus(c fiber.Ctx) error {
	status, err := s.safeRepo.GetPingStatus()
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, status)
}

func (s *SafeService) UpdatePingStatus(c fiber.Ctx) error {
	req, err := Bind[request.SafeUpdatePingStatus](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.safeRepo.UpdatePingStatus(req.Status); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
