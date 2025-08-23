package service

import (
	"github.com/gofiber/fiber/v2"
	"net/http"

	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
)

type TaskService struct {
	taskRepo biz.TaskRepo
}

func NewTaskService(task biz.TaskRepo) *TaskService {
	return &TaskService{
		taskRepo: task,
	}
}

func (s *TaskService) Status(c fiber.Ctx) error {
	return Success(c, chix.M{
		"task": s.taskRepo.HasRunningTask(),
	})
}

func (s *TaskService) List(c fiber.Ctx) error {
	req, err := Bind[request.Paginate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	tasks, total, err := s.taskRepo.List(req.Page, req.Limit)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"total": total,
		"items": tasks,
	})
}

func (s *TaskService) Get(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	task, err := s.taskRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, task)
}

func (s *TaskService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	err = s.taskRepo.Delete(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
