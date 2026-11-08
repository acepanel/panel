package biz

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/moby/moby/api/types/container"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/docker"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerRepo interface {
	ListAll(ctx context.Context, sock string) ([]types.Container, error)
	Inspect(ctx context.Context, sock string, id string) (any, error)
	Create(ctx context.Context, sock string, req *request.ContainerCreate) (string, error)
	Remove(ctx context.Context, sock string, id string) error
	Start(ctx context.Context, sock string, id string) error
	Stop(ctx context.Context, sock string, id string) error
	Restart(ctx context.Context, sock string, id string) error
	Pause(ctx context.Context, sock string, id string) error
	Unpause(ctx context.Context, sock string, id string) error
	Kill(ctx context.Context, sock string, id string) error
	Rename(ctx context.Context, sock string, id string, newName string) error
	Logs(ctx context.Context, sock string, id string, tail int) (string, error)
	Prune(ctx context.Context, sock string) error
}

type ContainerUsecase struct {
	repo    ContainerRepo
	setting SettingRepo
	task    TaskRepo
	t       *gotext.Locale
	log     *slog.Logger
}

func NewContainerUsecase(t *gotext.Locale, log *slog.Logger, containerRepo ContainerRepo, settingRepo SettingRepo, taskRepo TaskRepo) *ContainerUsecase {
	return &ContainerUsecase{
		repo:    containerRepo,
		setting: settingRepo,
		task:    taskRepo,
		t:       t,
		log:     log,
	}
}

func (uc *ContainerUsecase) ListAll(ctx context.Context) ([]types.Container, error) {
	sock := containerSock(uc.setting)
	return uc.repo.ListAll(ctx, sock)
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
	sock := containerSock(uc.setting)
	data, err := uc.repo.Inspect(ctx, sock, id)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("container viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", containerName(data)))
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

func (uc *ContainerUsecase) create(ctx context.Context, req *request.ContainerCreate) (string, error) {
	sock := containerSock(uc.setting)
	// 中途取消会留下已创建未启动的容器
	ctx = context.WithoutCancel(ctx)
	id, err := uc.repo.Create(ctx, sock, req)
	if err != nil {
		return "", err
	}
	if err = uc.repo.Start(ctx, sock, id); err != nil {
		return "", errors.New(uc.t.Get("Container created but failed to start: %v", err))
	}
	return id, nil
}

func (uc *ContainerUsecase) CreateBackground(ctx context.Context, req *request.ContainerCreate) error {
	shell, err := docker.RunShell(containerSock(uc.setting), req)
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
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Remove(ctx, sock, id); err != nil {
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
	sock := containerSock(uc.setting)
	runShell, err := docker.RunShell(sock, req)
	if err != nil {
		return err
	}

	shell := strings.Join([]string{
		"set -e",
		docker.Command(sock, "image", "inspect", req.Image) + " >/dev/null 2>&1 || " + docker.Command(sock, "pull", req.Image),
		docker.Command(sock, "rm", "--force", id),
		runShell,
	}, "\n")
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
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Remove(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("container deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Start(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Start(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container started"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Stop(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Stop(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container stopped"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Restart(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Restart(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container restarted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Pause(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Pause(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container paused"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Unpause(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Unpause(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container unpaused"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Kill(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	name := uc.name(ctx, sock, id)
	if err := uc.repo.Kill(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("container killed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", name))
	return nil
}

func (uc *ContainerUsecase) Rename(ctx context.Context, id string, newName string) error {
	sock := containerSock(uc.setting)
	oldName := uc.name(ctx, sock, id)
	if err := uc.repo.Rename(ctx, sock, id, newName); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("container renamed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("old_name", oldName), slog.String("name", newName))
	return nil
}

func (uc *ContainerUsecase) Logs(ctx context.Context, id string, tail int) (string, error) {
	sock := containerSock(uc.setting)
	return uc.repo.Logs(ctx, sock, id, tail)
}

func (uc *ContainerUsecase) Prune(ctx context.Context) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Prune(ctx, sock); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("containers pruned"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}

func (uc *ContainerUsecase) name(ctx context.Context, sock, id string) string {
	info, _ := uc.repo.Inspect(ctx, sock, id)
	return containerName(info)
}

func containerName(info any) string {
	if resp, ok := info.(container.InspectResponse); ok {
		return strings.TrimPrefix(resp.Name, "/")
	}
	return ""
}
