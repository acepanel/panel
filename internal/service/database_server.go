package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type DatabaseServerService struct {
	databaseServerRepo biz.DatabaseServerRepo
}

func NewDatabaseServerService(databaseServer biz.DatabaseServerRepo) *DatabaseServerService {
	return &DatabaseServerService{
		databaseServerRepo: databaseServer,
	}
}

func (s *DatabaseServerService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	servers, total, err := s.databaseServerRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": servers,
	})
}

func (s *DatabaseServerService) Create(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseServerCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseServerRepo.Create(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseServerService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	server, err := s.databaseServerRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, server)
}

func (s *DatabaseServerService) Update(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseServerUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseServerRepo.Update(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseServerService) UpdateRemark(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseServerUpdateRemark](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseServerRepo.UpdateRemark(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseServerService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseServerRepo.Delete(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseServerService) Sync(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseServerRepo.Sync(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
