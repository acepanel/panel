package service

import (
	"github.com/gofiber/fiber/v3"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type DatabaseService struct {
	databaseRepo biz.DatabaseRepo
}

func NewDatabaseService(database biz.DatabaseRepo) *DatabaseService {
	return &DatabaseService{
		databaseRepo: database,
	}
}

func (s *DatabaseService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	databases, total, err := s.databaseRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": databases,
	})
}

func (s *DatabaseService) Create(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseRepo.Create(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseDelete](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseRepo.Delete(req.ServerID, req.Name); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseService) Comment(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseComment](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseRepo.Comment(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
