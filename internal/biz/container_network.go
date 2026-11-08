package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerNetworkRepo interface {
	List(ctx context.Context, sock string) ([]types.ContainerNetwork, error)
	Create(ctx context.Context, sock string, req *request.ContainerNetworkCreate) (string, error)
	Remove(ctx context.Context, sock string, id string) error
	Prune(ctx context.Context, sock string) error
}

type ContainerNetworkUsecase struct {
	repo    ContainerNetworkRepo
	setting SettingRepo
	t       *gotext.Locale
	log     *slog.Logger
}

func NewContainerNetworkUsecase(t *gotext.Locale, log *slog.Logger, repo ContainerNetworkRepo, setting SettingRepo) *ContainerNetworkUsecase {
	return &ContainerNetworkUsecase{repo: repo, setting: setting, t: t, log: log}
}

func (uc *ContainerNetworkUsecase) List(ctx context.Context) ([]types.ContainerNetwork, error) {
	sock := containerSock(uc.setting)
	networks, err := uc.repo.List(ctx, sock)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("network list viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return networks, nil
}

func (uc *ContainerNetworkUsecase) Create(ctx context.Context, req *request.ContainerNetworkCreate) (string, error) {
	sock := containerSock(uc.setting)
	id, err := uc.repo.Create(ctx, sock, req)
	if err != nil {
		return "", err
	}

	uc.log.Info(uc.t.Get("network created"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id), slog.String("name", req.Name), slog.String("driver", req.Driver))
	return id, nil
}

func (uc *ContainerNetworkUsecase) Remove(ctx context.Context, id string) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Remove(ctx, sock, id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("network deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("id", id))
	return nil
}

func (uc *ContainerNetworkUsecase) Prune(ctx context.Context) error {
	sock := containerSock(uc.setting)
	if err := uc.repo.Prune(ctx, sock); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("networks pruned"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return nil
}
