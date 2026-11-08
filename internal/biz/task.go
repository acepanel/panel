package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
)

type TaskStatus string

const (
	TaskStatusWaiting  TaskStatus = "waiting"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusSuccess  TaskStatus = "finished"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusCanceled TaskStatus = "canceled"
)

type Task struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Key         string     `gorm:"not null;default:'';index" json:"-"` // 任务标识（域:动作:标的），用于防重
	Name        string     `gorm:"not null;default:'';index" json:"name"`
	Status      TaskStatus `gorm:"not null;default:'waiting'" json:"status"`
	Shell       string     `gorm:"not null;default:''" json:"-"`
	CancelShell string     `gorm:"not null;default:''" json:"-"` // 运行中被取消后执行的清理命令（可选）
	Log         string     `gorm:"not null;default:''" json:"log"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TaskRepo interface {
	HasRunningTask() bool
	List(page, limit uint) ([]*Task, int64, error)
	ListActive() ([]*Task, error)
	Get(id uint) (*Task, error)
	GetByIDs(ids []uint) ([]*Task, error)
	Delete(id uint) error
	Cancel(id uint) error
	UpdateStatus(id uint, status TaskStatus) error
	UpdateLog(id uint, log string) error
	Push(task *Task) error
}

type TaskUsecase struct {
	t    *gotext.Locale
	log  *slog.Logger
	repo TaskRepo
}

func NewTaskUsecase(t *gotext.Locale, log *slog.Logger, repo TaskRepo) *TaskUsecase {
	return &TaskUsecase{t: t, log: log, repo: repo}
}

func (uc *TaskUsecase) HasRunningTask() bool {
	return uc.repo.HasRunningTask()
}

func (uc *TaskUsecase) List(ctx context.Context, page, limit uint) ([]*Task, int64, error) {
	tasks, total, err := uc.repo.List(page, limit)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("task list viewed"), slog.String("type", OperationTypePanel), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("page", uint64(page)), slog.Uint64("limit", uint64(limit)))
	return tasks, total, nil
}

func (uc *TaskUsecase) ListActive() ([]*Task, error) {
	return uc.repo.ListActive()
}

func (uc *TaskUsecase) Get(ctx context.Context, id uint) (*Task, error) {
	task, err := uc.repo.Get(id)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("task viewed"), slog.String("type", OperationTypePanel), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", task.Name))
	return task, nil
}

func (uc *TaskUsecase) GetByIDs(ids []uint) ([]*Task, error) {
	return uc.repo.GetByIDs(ids)
}

func (uc *TaskUsecase) Delete(ctx context.Context, id uint) error {
	task, err := uc.repo.Get(id)
	if err != nil {
		return err
	}
	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("task deleted"), slog.String("type", OperationTypePanel), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", task.Name), slog.String("status", string(task.Status)))
	return nil
}

func (uc *TaskUsecase) Cancel(ctx context.Context, id uint) error {
	task, err := uc.repo.Get(id)
	if err != nil {
		return err
	}
	if err = uc.repo.Cancel(id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("task canceled"), slog.String("type", OperationTypePanel), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", task.Name), slog.String("status", string(task.Status)))
	return nil
}

func (uc *TaskUsecase) UpdateStatus(id uint, status TaskStatus) error {
	return uc.repo.UpdateStatus(id, status)
}

func (uc *TaskUsecase) UpdateLog(id uint, log string) error {
	return uc.repo.UpdateLog(id, log)
}

func (uc *TaskUsecase) Push(task *Task) error {
	return uc.repo.Push(task)
}
