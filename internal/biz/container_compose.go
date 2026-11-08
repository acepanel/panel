package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/pkg/types"
)

type ContainerComposeRepo interface {
	List(ctx context.Context) ([]types.ContainerCompose, error)
	Get(name string) (string, []types.KV, error)
	Create(name, compose string, envs []types.KV) error
	Update(name, compose string, envs []types.KV) error
	Up(ctx context.Context, name string, force bool) error
	Down(ctx context.Context, name string) error
	RemoveDir(name string) error
}

type ContainerComposeUsecase struct {
	repo ContainerComposeRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewContainerComposeUsecase(t *gotext.Locale, log *slog.Logger, repo ContainerComposeRepo) *ContainerComposeUsecase {
	return &ContainerComposeUsecase{repo: repo, t: t, log: log}
}

func (uc *ContainerComposeUsecase) List(ctx context.Context) ([]types.ContainerCompose, error) {
	composes, err := uc.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("compose list viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)))
	return composes, nil
}

func (uc *ContainerComposeUsecase) Get(ctx context.Context, name string) (string, []types.KV, error) {
	compose, envs, err := uc.repo.Get(name)
	if err != nil {
		return "", nil, err
	}

	uc.log.Info(uc.t.Get("compose viewed"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name))
	return compose, envs, nil
}

func (uc *ContainerComposeUsecase) Create(ctx context.Context, name, compose string, envs []types.KV) error {
	if err := uc.repo.Create(name, compose, envs); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("compose created"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name))
	return nil
}

func (uc *ContainerComposeUsecase) Update(ctx context.Context, name, compose string, envs []types.KV) error {
	if err := uc.repo.Update(name, compose, envs); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("compose updated"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name))
	return nil
}

func (uc *ContainerComposeUsecase) Up(ctx context.Context, name string, force bool) error {
	if err := uc.repo.Up(ctx, name, force); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("compose started"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name), slog.Bool("force", force))
	return nil
}

func (uc *ContainerComposeUsecase) Down(ctx context.Context, name string) error {
	if err := uc.repo.Down(ctx, name); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("compose stopped"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name))
	return nil
}

func (uc *ContainerComposeUsecase) Remove(ctx context.Context, name string) error {
	if err := uc.repo.Down(ctx, name); err != nil {
		return err
	}
	if err := uc.repo.RemoveDir(name); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("compose deleted"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name))
	return nil
}
