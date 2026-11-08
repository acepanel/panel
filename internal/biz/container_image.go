package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/docker"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerImageRepo interface {
	List(ctx context.Context, sock string) ([]types.ContainerImage, error)
	Exist(ctx context.Context, sock string, name string) (bool, error)
	Pull(ctx context.Context, sock string, req *request.ContainerImagePull) error
	Remove(ctx context.Context, sock string, id string) error
	Prune(ctx context.Context, sock string) error
}

type ContainerImageUsecase struct {
	repo    ContainerImageRepo
	setting SettingRepo
	task    TaskRepo
	t       *gotext.Locale
	log     *slog.Logger
}

func NewContainerImageUsecase(t *gotext.Locale, log *slog.Logger, containerImageRepo ContainerImageRepo, settingRepo SettingRepo, taskRepo TaskRepo) *ContainerImageUsecase {
	return &ContainerImageUsecase{
		repo:    containerImageRepo,
		setting: settingRepo,
		task:    taskRepo,
		t:       t,
		log:     log,
	}
}

func (uc *ContainerImageUsecase) List(ctx context.Context) ([]types.ContainerImage, error) {
	sock := containerSock(uc.setting)
	return uc.repo.List(ctx, sock)
}

func (uc *ContainerImageUsecase) Exist(ctx context.Context, name string) (bool, error) {
	sock := containerSock(uc.setting)
	return uc.repo.Exist(ctx, sock, name)
}

func (uc *ContainerImageUsecase) Pull(ctx context.Context, req *request.ContainerImagePull) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Pull(ctx, sock, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("image pulled"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name))
	return nil
}

func (uc *ContainerImageUsecase) PullBackground(ctx context.Context, req *request.ContainerImagePull) error {
	shell, cancelShell, err := docker.ImagePullShell(containerSock(uc.setting), req)
	if err != nil {
		return err
	}

	task := new(Task)
	task.Key = "container:image:pull:" + req.Name
	task.Name = uc.t.Get("Pull image %s", req.Name)
	task.Status = TaskStatusWaiting
	task.Shell = shell
	task.CancelShell = cancelShell

	if err = uc.task.Push(task); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("image pulled"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.Bool("background", true))
	return nil
}

func (uc *ContainerImageUsecase) Remove(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Remove(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("image deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id))
	return nil
}

func (uc *ContainerImageUsecase) Prune(ctx context.Context) error {
	sock := containerSock(uc.setting)
	return uc.repo.Prune(ctx, sock)
}
