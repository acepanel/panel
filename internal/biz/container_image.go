package biz

import (
	"context"
	"iter"
	"log/slog"

	"github.com/leonelquinteros/gotext"
	"github.com/moby/moby/api/types/jsonstream"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

// ImagePullProgress 拉取镜像时 Docker 推送的一条进度
type ImagePullProgress = jsonstream.Message

type ContainerImageRepo interface {
	List(ctx context.Context) ([]types.ContainerImage, error)
	Exist(ctx context.Context, name string) (bool, error)
	Pull(ctx context.Context, req *request.ContainerImagePull) error
	// PullStream 发起镜像拉取并返回进度流，发起失败直接返回错误，开始后的失败在流里给出
	PullStream(ctx context.Context, req *request.ContainerImagePull) (iter.Seq2[ImagePullProgress, error], error)
	// PullShell 生成后台拉取镜像的命令，需要认证时一并返回清理凭据的命令
	PullShell(req *request.ContainerImagePull) (shell, cancelShell string, err error)
	// ExportShell 生成把镜像导出到 dir 的命令和取消时的清理命令，dir 留空用备份目录，一并返回归档路径
	ExportShell(ctx context.Context, id, dir string) (path, shell, cancelShell string, err error)
	// ImportShell 生成从归档导入镜像的命令
	ImportShell(path string) string
	Remove(ctx context.Context, id string) error
	Prune(ctx context.Context) error
}

type ContainerImageUsecase struct {
	repo ContainerImageRepo
	task TaskRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewContainerImageUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	containerImageRepo ContainerImageRepo,
	taskRepo TaskRepo,
) *ContainerImageUsecase {
	return &ContainerImageUsecase{
		repo: containerImageRepo,
		task: taskRepo,
		t:    t,
		log:  log,
	}
}

func (uc *ContainerImageUsecase) List(ctx context.Context) ([]types.ContainerImage, error) {
	return uc.repo.List(ctx)
}

func (uc *ContainerImageUsecase) Exist(ctx context.Context, name string) (bool, error) {
	return uc.repo.Exist(ctx, name)
}

func (uc *ContainerImageUsecase) Pull(ctx context.Context, req *request.ContainerImagePull) error {
	if err := uc.repo.Pull(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("image pulled"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name))
	return nil
}

func (uc *ContainerImageUsecase) PullStream(
	ctx context.Context,
	req *request.ContainerImagePull,
) (iter.Seq2[ImagePullProgress, error], error) {
	return uc.repo.PullStream(ctx, req)
}

func (uc *ContainerImageUsecase) PullBackground(ctx context.Context, req *request.ContainerImagePull) error {
	shell, cancelShell, err := uc.repo.PullShell(req)
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

// Export 提交导出镜像的后台任务，返回归档路径
func (uc *ContainerImageUsecase) Export(ctx context.Context, req *request.ContainerImageExport) (string, error) {
	path, shell, cancelShell, err := uc.repo.ExportShell(ctx, req.ID, req.Path)
	if err != nil {
		return "", err
	}

	task := new(Task)
	task.Key = "container:image:export:" + req.ID
	task.Name = uc.t.Get("Export image to %s", path)
	task.Status = TaskStatusWaiting
	task.Shell = shell
	task.CancelShell = cancelShell

	if err = uc.task.Push(task); err != nil {
		return "", err
	}

	uc.log.Info(uc.t.Get("image exported"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", req.ID), slog.String("path", path))
	return path, nil
}

// Import 提交导入镜像的后台任务
func (uc *ContainerImageUsecase) Import(ctx context.Context, req *request.ContainerImageImport) error {
	task := new(Task)
	task.Key = "container:image:import:" + req.Path
	task.Name = uc.t.Get("Import image from %s", req.Path)
	task.Status = TaskStatusWaiting
	task.Shell = uc.repo.ImportShell(req.Path)

	if err := uc.task.Push(task); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("image imported"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("path", req.Path))
	return nil
}

func (uc *ContainerImageUsecase) Remove(ctx context.Context, id string) error {
	if err := uc.repo.Remove(ctx, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("image deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id))
	return nil
}

func (uc *ContainerImageUsecase) Prune(ctx context.Context) error {
	return uc.repo.Prune(ctx)
}
