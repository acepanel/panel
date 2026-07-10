package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type TaskService struct {
	taskRepo *biz.TaskUsecase
}

func NewTaskService(i do.Injector) (*TaskService, error) {
	return &TaskService{
		taskRepo: do.MustInvoke[*biz.TaskUsecase](i),
	}, nil
}

func (s *TaskService) Status(w http.ResponseWriter, r *http.Request) {
	Success(w, chix.M{
		"task": s.taskRepo.HasRunningTask(),
	})
}

func (s *TaskService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	tasks, total, err := s.taskRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": tasks,
	})
}

func (s *TaskService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	task, err := s.taskRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, task)
}

func (s *TaskService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	err = s.taskRepo.Delete(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
