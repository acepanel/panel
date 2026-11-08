package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type TaskService struct {
	taskRepo *biz.TaskUsecase
}

func NewTaskService(taskUsecase *biz.TaskUsecase) *TaskService {
	return &TaskService{
		taskRepo: taskUsecase,
	}
}

func (s *TaskService) Status(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.taskRepo.ListActive()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"task":  len(tasks) > 0,
		"tasks": tasks,
	})
}

func (s *TaskService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	tasks, total, err := s.taskRepo.List(r.Context(), req.Page, req.Limit)
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

	task, err := s.taskRepo.Get(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, task)
}

// Query 批量获取任务，已删除的任务不在结果里
func (s *TaskService) Query(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.TaskIDs](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	tasks, err := s.taskRepo.GetByIDs(req.IDs)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, tasks)
}

func (s *TaskService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	err = s.taskRepo.Delete(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *TaskService) Cancel(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.taskRepo.Cancel(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
