package data

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/api"
)

type cacheRepo struct {
	api *api.API
	db  *gorm.DB
}

func NewCacheRepo(apiClient *api.API, db *gorm.DB) biz.CacheRepo {
	return &cacheRepo{
		api: apiClient,
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
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&biz.Cache{Key: key, Value: value}).Error
}

func (r *cacheRepo) FetchCategories() (*api.Categories, error) {
	return r.api.Categories()
}

func (r *cacheRepo) FetchApps() (*api.Apps, error) {
	return r.api.Apps()
}

func (r *cacheRepo) FetchEnvironments() (*api.Environments, error) {
	return r.api.Environments()
}

func (r *cacheRepo) FetchTemplates() (*api.Templates, error) {
	return r.api.Templates()
}

func (r *cacheRepo) FetchLatestVersion(channel string) (*api.Version, error) {
	return r.api.LatestVersion(channel)
}

func (r *cacheRepo) FetchIntermediateVersions(channel string) (*api.Versions, error) {
	return r.api.IntermediateVersions(channel)
}
