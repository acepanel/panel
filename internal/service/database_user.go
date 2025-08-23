package service

import (
	"github.com/gofiber/fiber/v2"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type DatabaseUserService struct {
	databaseUserRepo biz.DatabaseUserRepo
}

func NewDatabaseUserService(databaseUser biz.DatabaseUserRepo) *DatabaseUserService {
	return &DatabaseUserService{
		databaseUserRepo: databaseUser,
	}
}

func (s *DatabaseUserService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	users, total, err := s.databaseUserRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": users,
	})
}

func (s *DatabaseUserService) Create(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseUserCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseUserRepo.Create(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseUserService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	user, err := s.databaseUserRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, user)
}

func (s *DatabaseUserService) Update(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseUserUpdate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseUserRepo.Update(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseUserService) UpdateRemark(c fiber.Ctx) error {
	req, err := Bind[request.DatabaseUserUpdateRemark](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseUserRepo.UpdateRemark(req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *DatabaseUserService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.databaseUserRepo.Delete(req.ID); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
