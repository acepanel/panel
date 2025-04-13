package data

import (
	"encoding/json"
	"errors"
	"slices"

	"gorm.io/gorm"

	"github.com/tnb-labs/panel/internal/app"
	"github.com/tnb-labs/panel/internal/biz"
	"github.com/tnb-labs/panel/pkg/api"
	"github.com/tnb-labs/panel/pkg/apploader"
)

type cacheRepo struct {
	api *api.API
	db  *gorm.DB
}

func NewCacheRepo(db *gorm.DB) biz.CacheRepo {
	return &cacheRepo{
		api: api.NewAPI(app.Version, app.Locale),
		db:  db,
	}
}

func (r *cacheRepo) Get(key biz.CacheKey, defaultValue ...string) (string, error) {
	cache := new(biz.Cache)
	if err := r.db.Where("key = ?", key).First(cache).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}

	if cache.Value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return cache.Value, nil
}

func (r *cacheRepo) Set(key biz.CacheKey, value string) error {
	cache := new(biz.Cache)
	if err := r.db.Where(biz.Cache{Key: key}).FirstOrInit(cache).Error; err != nil {
		return err
	}

	cache.Value = value
	return r.db.Save(cache).Error
}

func (r *cacheRepo) UpdateApps() error {
	remote, err := r.api.Apps()
	if err != nil {
		return err
	}

	// 去除本地不存在的应用
	*remote = slices.Clip(slices.DeleteFunc(*remote, func(item *api.App) bool {
		return !slices.Contains(apploader.Slugs(), item.Slug)
	}))

	encoded, err := json.Marshal(remote)
	if err != nil {
		return err
	}

	return r.Set(biz.CacheKeyApps, string(encoded))
}

func (r *cacheRepo) UpdateRewrites() error {
	rewrites, err := r.api.RewritesByType("nginx")
	if err != nil {
		return err
	}

	encoded, err := json.Marshal(rewrites)
	if err != nil {
		return err
	}

	return r.Set(biz.CacheKeyRewrites, string(encoded))
}
