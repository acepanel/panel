package biz

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type DatabaseRedisRepo interface {
	Databases(ctx context.Context, req *request.DatabaseRedisDatabases) (int, error)
	Data(ctx context.Context, req *request.DatabaseRedisData) ([]db.RedisKV, int, error)
	KeyGet(ctx context.Context, req *request.DatabaseRedisKeyGet) (*db.RedisKV, error)
	KeySet(ctx context.Context, req *request.DatabaseRedisKeySet) error
	KeyDelete(ctx context.Context, req *request.DatabaseRedisKeyDelete) error
	KeyTTL(ctx context.Context, req *request.DatabaseRedisKeyTTL) error
	KeyRename(ctx context.Context, req *request.DatabaseRedisKeyRename) error
	Clear(ctx context.Context, req *request.DatabaseRedisClear) error
}

// DatabaseRedisUsecase Redis 业务用例
type DatabaseRedisUsecase struct {
	t    *gotext.Locale
	log  *slog.Logger
	repo DatabaseRedisRepo
}

func NewDatabaseRedisUsecase(t *gotext.Locale, log *slog.Logger, repo DatabaseRedisRepo) *DatabaseRedisUsecase {
	return &DatabaseRedisUsecase{t: t, log: log, repo: repo}
}

func (uc *DatabaseRedisUsecase) Databases(ctx context.Context, req *request.DatabaseRedisDatabases) (int, error) {
	return uc.repo.Databases(ctx, req)
}

func (uc *DatabaseRedisUsecase) Data(ctx context.Context, req *request.DatabaseRedisData) ([]db.RedisKV, int, error) {
	items, total, err := uc.repo.Data(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("redis key list viewed"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB), slog.String("search", req.Search), slog.Uint64("page", uint64(req.Page)))
	return items, total, nil
}

func (uc *DatabaseRedisUsecase) KeyGet(ctx context.Context, req *request.DatabaseRedisKeyGet) (*db.RedisKV, error) {
	kv, err := uc.repo.KeyGet(ctx, req)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("redis key viewed"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB), slog.String("key", req.Key))
	return kv, nil
}

func (uc *DatabaseRedisUsecase) KeySet(ctx context.Context, req *request.DatabaseRedisKeySet) error {
	if err := uc.repo.KeySet(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("redis key saved"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB), slog.String("key", req.Key), slog.String("key_type", req.Type), slog.Int64("ttl", req.TTL))
	return nil
}

func (uc *DatabaseRedisUsecase) KeyDelete(ctx context.Context, req *request.DatabaseRedisKeyDelete) error {
	if err := uc.repo.KeyDelete(ctx, req); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("redis key deleted"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB), slog.String("key", req.Key))
	return nil
}

func (uc *DatabaseRedisUsecase) KeyTTL(ctx context.Context, req *request.DatabaseRedisKeyTTL) error {
	if err := uc.repo.KeyTTL(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("redis key ttl updated"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB), slog.String("key", req.Key), slog.Int64("ttl", req.TTL))
	return nil
}

func (uc *DatabaseRedisUsecase) KeyRename(ctx context.Context, req *request.DatabaseRedisKeyRename) error {
	if err := uc.repo.KeyRename(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("redis key renamed"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB), slog.String("old_key", req.OldKey), slog.String("new_key", req.NewKey))
	return nil
}

func (uc *DatabaseRedisUsecase) Clear(ctx context.Context, req *request.DatabaseRedisClear) error {
	if err := uc.repo.Clear(ctx, req); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("redis database cleared"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("server_id", uint64(req.ServerID)), slog.Int("db", req.DB))
	return nil
}
