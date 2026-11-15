package biz

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/moby/moby/api/types/container"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerRepo interface {
	ListAll(ctx context.Context) ([]types.Container, error)
	Inspect(ctx context.Context, id string) (any, error)
	Create(ctx context.Context, req *request.ContainerCreate) (string, error)
	// CreateShell 生成后台创建容器的命令
	CreateShell(req *request.ContainerCreate) (string, error)
	// UpdateShell 生成后台重建容器的命令：删除旧容器后按新配置创建，本地没有镜像时先拉取
	UpdateShell(id string, req *request.ContainerCreate) (string, error)
	Remove(ctx context.Context, id string) error
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Pause(ctx context.Context, id string) error
	Unpause(ctx context.Context, id string) error
	Kill(ctx context.Context, id string) error
	Rename(ctx context.Context, id string, newName string) error
	Logs(ctx context.Context, id string, tail int) (string, error)
	// FollowLogs 把容器新增的日志持续写入 w，直到日志流结束或 ctx 取消，只在无法开始跟随时返回错误
	FollowLogs(ctx context.Context, id string, w io.Writer) error
	Prune(ctx context.Context) error
}

type ContainerUsecase struct {
	repo ContainerRepo
	task TaskRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewContainerUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	containerRepo ContainerRepo,
	taskRepo TaskRepo,
) *ContainerUsecase {
	return &ContainerUsecase{
		repo: containerRepo,
		task: taskRepo,
		t:    t,
		log:  log,
	}
}

func (uc *ContainerUsecase) ListAll(ctx context.Context) ([]types.Container, error) {
	return uc.repo.ListAll(ctx)
}

// List 按名称或镜像关键字筛选容器，关键字为空时返回全部
func (uc *ContainerUsecase) List(ctx context.Context, keyword string) ([]types.Container, error) {
	containers, err := uc.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("container list viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("keyword", keyword))

	keyword = strings.ToLower(keyword)
	return slices.DeleteFunc(containers, func(item types.Container) bool {
		return !strings.Contains(strings.ToLower(item.Name), keyword) && !strings.Contains(strings.ToLower(item.Image), keyword)
	}), nil
}

func (uc *ContainerUsecase) Inspect(ctx context.Context, id string) (any, error) {
	data, err := uc.repo.Inspect(ctx, id)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("container viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", uc.containerName(data)))
	return data, nil
}

// Create 创建并启动容器
func (uc *ContainerUsecase) Create(ctx context.Context, req *request.ContainerCreate) (string, error) {
	id, err := uc.create(ctx, req)
	if err != nil {
		return "", err
	}

	uc.log.Info(uc.t.Get("container created"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", req.Name), slog.String("image", req.Image))
	return id, nil
}

func (uc *ContainerUsecase) CreateBackground(ctx context.Context, req *request.ContainerCreate) error {
	shell, err := uc.repo.CreateShell(req)
	if err != nil {
		return err
	}

	task := new(Task)
	key := ""
	if req.Name != "" {
		key = "container:create:" + req.Name
	}
	task.Key = key
	target := req.Name
	if target == "" {
		target = req.Image
	}
	task.Name = uc.t.Get("Create container %s", target)
	task.Status = TaskStatusWaiting
	task.Shell = shell

	if err = uc.task.Push(task); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container created"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.String("image", req.Image), slog.Bool("background", true))
	return nil
}

// Update 删除旧容器后按新配置重建同名容器
func (uc *ContainerUsecase) Update(ctx context.Context, id string, req *request.ContainerCreate) (string, error) {
	name := uc.name(ctx, id)
	if err := uc.repo.Remove(ctx, id); err != nil {
		return "", err
	}
	newID, err := uc.create(ctx, req)
	if err != nil {
		uc.log.Warn(uc.t.Get("container deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
		return "", err
	}

	uc.log.Info(uc.t.Get("container updated"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("new_id", newID), slog.String("name", req.Name), slog.String("image", req.Image))
	return newID, nil
}

func (uc *ContainerUsecase) UpdateBackground(ctx context.Context, id string, req *request.ContainerCreate) error {
	shell, err := uc.repo.UpdateShell(id, req)
	if err != nil {
		return err
	}

	target := req.Name
	if target == "" {
		target = req.Image
	}

	task := new(Task)
	task.Key = "container:update:" + id
	task.Name = uc.t.Get("Update container %s", target)
	task.Status = TaskStatusWaiting
	task.Shell = shell

	if err = uc.task.Push(task); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container updated"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", req.Name), slog.String("image", req.Image), slog.Bool("background", true))
	return nil
}

func (uc *ContainerUsecase) Remove(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Remove(ctx, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("container deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Start(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Start(ctx, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container started"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Stop(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Stop(ctx, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container stopped"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Restart(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Restart(ctx, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container restarted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Pause(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Pause(ctx, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container paused"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Unpause(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Unpause(ctx, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container unpaused"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Kill(ctx context.Context, id string) error {
	name := uc.name(ctx, id)
	if err := uc.repo.Kill(ctx, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("container killed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Rename(ctx context.Context, id string, newName string) error {
	oldName := uc.name(ctx, id)
	if err := uc.repo.Rename(ctx, id, newName); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container renamed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("old_name", oldName), slog.String("name", newName))
	return nil
}

func (uc *ContainerUsecase) Logs(ctx context.Context, id string, tail int) (string, error) {
	return uc.repo.Logs(ctx, id, tail)
}

func (uc *ContainerUsecase) FollowLogs(ctx context.Context, id string, w io.Writer) error {
	return uc.repo.FollowLogs(ctx, id, w)
}

func (uc *ContainerUsecase) Prune(ctx context.Context) error {
	if err := uc.repo.Prune(ctx); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("containers pruned"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}

func (uc *ContainerUsecase) create(ctx context.Context, req *request.ContainerCreate) (string, error) {
	// 中途取消会留下已创建未启动的容器
	ctx = context.WithoutCancel(ctx)
	id, err := uc.repo.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if err = uc.repo.Start(ctx, id); err != nil {
		return "", errors.New(uc.t.Get("Container created but failed to start: %v", err))
	}
	return id, nil
}

func (uc *ContainerUsecase) name(ctx context.Context, id string) string {
	info, _ := uc.repo.Inspect(ctx, id)
	return uc.containerName(info)
}

func (uc *ContainerUsecase) containerName(info any) string {
	if resp, ok := info.(container.InspectResponse); ok {
		return strings.TrimPrefix(resp.Name, "/")
	}
	return ""
}
