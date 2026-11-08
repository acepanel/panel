package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerVolumeRepo interface {
	List(ctx context.Context, sock string) ([]types.ContainerVolume, error)
	Create(ctx context.Context, sock string, req *request.ContainerVolumeCreate) (string, error)
	Remove(ctx context.Context, sock string, id string) error
	Prune(ctx context.Context, sock string) error
}

type ContainerVolumeUsecase struct {
	repo    ContainerVolumeRepo
	setting SettingRepo
	t       *gotext.Locale
	log     *slog.Logger
}

func NewContainerVolumeUsecase(t *gotext.Locale, log *slog.Logger, repo ContainerVolumeRepo, setting SettingRepo) *ContainerVolumeUsecase {
	return &ContainerVolumeUsecase{repo: repo, setting: setting, t: t, log: log}
}

func (uc *ContainerVolumeUsecase) List(ctx context.Context) ([]types.ContainerVolume, error) {
	sock := containerSock(uc.setting)
	volumes, err := uc.repo.List(ctx, sock)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("volume list viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return volumes, nil
}

func (uc *ContainerVolumeUsecase) Create(ctx context.Context, req *request.ContainerVolumeCreate) (string, error) {
	sock := containerSock(uc.setting)
	name, err := uc.repo.Create(ctx, sock, req)
	if err != nil {
		return "", err
	}

	uc.log.Info(uc.t.Get("volume created"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name), slog.String("driver", req.Driver))
	return name, nil
}

func (uc *ContainerVolumeUsecase) Remove(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Remove(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("volume deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", id))
	return nil
}

func (uc *ContainerVolumeUsecase) Prune(ctx context.Context) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Prune(ctx, sock); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("volumes pruned"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}
