package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerVolumeRepo interface {
	List(ctx context.Context) ([]types.ContainerVolume, error)
	Create(ctx context.Context, req *request.ContainerVolumeCreate) (string, error)
	Remove(ctx context.Context, id string) error
	Prune(ctx context.Context) error
}

type ContainerVolumeUsecase struct {
	repo ContainerVolumeRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewContainerVolumeUsecase(t *gotext.Locale, log *slog.Logger, repo ContainerVolumeRepo) *ContainerVolumeUsecase {
	return &ContainerVolumeUsecase{repo: repo, t: t, log: log}
}

func (uc *ContainerVolumeUsecase) List(ctx context.Context) ([]types.ContainerVolume, error) {
	volumes, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("volume list viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return volumes, nil
}

func (uc *ContainerVolumeUsecase) Create(ctx context.Context, req *request.ContainerVolumeCreate) (string, error) {
	name, err := uc.repo.Create(ctx, req)
	if err != nil {
		return "", err
	}

	uc.log.Info(uc.t.Get("volume created"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name), slog.String("driver", req.Driver))
	return name, nil
}

func (uc *ContainerVolumeUsecase) Remove(ctx context.Context, id string) error {
	if err := uc.repo.Remove(ctx, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("volume deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", id))
	return nil
}

func (uc *ContainerVolumeUsecase) Prune(ctx context.Context) error {
	if err := uc.repo.Prune(ctx); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("volumes pruned"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}
