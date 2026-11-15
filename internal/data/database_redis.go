package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type databaseRedisRepo struct {
	t   *gotext.Locale
	orm *gorm.DB
}

func NewDatabaseRedisRepo(db *gorm.DB, t *gotext.Locale) biz.DatabaseRedisRepo {
	return &databaseRedisRepo{
		t:   t,
		orm: db,
	}
}

func (r *databaseRedisRepo) Databases(ctx context.Context, req *request.DatabaseRedisDatabases) (int, error) {
	client, err := r.getClient(ctx, req.ServerID, 0)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	return client.Database()
}

func (r *databaseRedisRepo) Data(ctx context.Context, req *request.DatabaseRedisData) ([]db.RedisKV, int, error) {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return nil, 0, err
	}
	defer client.Close()

	pattern := req.Search
	if pattern == "" {
		pattern = "*"
	}

	return client.Search(pattern, int(req.Page), int(req.Limit))
}

func (r *databaseRedisRepo) KeyGet(ctx context.Context, req *request.DatabaseRedisKeyGet) (*db.RedisKV, error) {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	return client.Get(req.Key)
}

func (r *databaseRedisRepo) KeySet(ctx context.Context, req *request.DatabaseRedisKeySet) error {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.SetKey(req.Key, req.Value, req.Type, req.TTL)
}

func (r *databaseRedisRepo) KeyDelete(ctx context.Context, req *request.DatabaseRedisKeyDelete) error {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Del(req.Key)
}

func (r *databaseRedisRepo) KeyTTL(ctx context.Context, req *request.DatabaseRedisKeyTTL) error {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Expire(req.Key, req.TTL)
}

func (r *databaseRedisRepo) KeyRename(ctx context.Context, req *request.DatabaseRedisKeyRename) error {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Rename(req.OldKey, req.NewKey)
}

func (r *databaseRedisRepo) Clear(ctx context.Context, req *request.DatabaseRedisClear) error {
	client, err := r.getClient(ctx, req.ServerID, req.DB)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Clear()
}

// getClient 根据服务器 ID 创建 Redis 客户端并选择指定数据库
func (r *databaseRedisRepo) getClient(ctx context.Context, serverID uint, dbIndex int) (*db.Redis, error) {
	server := new(biz.DatabaseServer)
	if err := r.orm.Where("id = ?", serverID).First(server).Error; err != nil {
		return nil, errors.New(r.t.Get("server not found"))
	}
	if server.Type != biz.DatabaseTypeRedis {
		return nil, errors.New(r.t.Get("server is not Redis type"))
	}

	client, err := db.NewRedis(ctx, server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
	if err != nil {
		return nil, errors.New(r.t.Get("failed to connect to Redis: %v", err))
	}

	if dbIndex > 0 {
		if err = client.Select(dbIndex); err != nil {
			client.Close()
			return nil, errors.New(r.t.Get("failed to select database: %v", err))
		}
	}

	return client, nil
}
