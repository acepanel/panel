package biz

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/apploader"
)

type CacheKey string

const (
	CacheKeyCategories  CacheKey = "categories"
	CacheKeyApps        CacheKey = "apps"
	CacheKeyEnvironment CacheKey = "environment"
	CacheKeyTemplates   CacheKey = "templates"
)

type Cache struct {
	Key       CacheKey  `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"not null;default:''" json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CacheRepo interface {
	Get(key CacheKey, defaultValue ...string) (string, error)
	Set(key CacheKey, value string) error
	FetchCategories() (*api.Categories, error)
	FetchApps() (*api.Apps, error)
	FetchEnvironments() (*api.Environments, error)
	FetchTemplates() (*api.Templates, error)
	FetchLatestVersion(channel string) (*api.Version, error)
	FetchIntermediateVersions(channel string) (*api.Versions, error)
	FetchVersions(channel string) (*api.Versions, error)
}

// CacheUpdate 一项云端数据缓存的刷新动作
type CacheUpdate struct {
	Key CacheKey
	Run func() error
}

type CacheUsecase struct {
	repo CacheRepo
}

func NewCacheUsecase(repo CacheRepo) *CacheUsecase {
	return &CacheUsecase{repo: repo}
}

// Updates 按刷新顺序列出全部云端数据缓存
func (uc *CacheUsecase) Updates() []CacheUpdate {
	return []CacheUpdate{
		{Key: CacheKeyCategories, Run: func() error { return updateCache(uc.repo, CacheKeyCategories, uc.repo.FetchCategories) }},
		{Key: CacheKeyApps, Run: func() error { return updateCache(uc.repo, CacheKeyApps, uc.fetchApps) }},
		{Key: CacheKeyEnvironment, Run: func() error { return updateCache(uc.repo, CacheKeyEnvironment, uc.repo.FetchEnvironments) }},
		{Key: CacheKeyTemplates, Run: func() error { return updateCache(uc.repo, CacheKeyTemplates, uc.repo.FetchTemplates) }},
	}
}

// UpdateAll 依次刷新全部云端数据缓存，出错即停并返回出错的那一项
func (uc *CacheUsecase) UpdateAll() (CacheKey, error) {
	for _, update := range uc.Updates() {
		if err := update.Run(); err != nil {
			return update.Key, err
		}
	}

	return "", nil
}

func (uc *CacheUsecase) fetchApps() (*api.Apps, error) {
	remote, err := uc.repo.FetchApps()
	if err != nil {
		return nil, err
	}

	// 去除本地不存在的应用
	*remote = slices.Clip(slices.DeleteFunc(*remote, func(item *api.App) bool {
		return !slices.Contains(apploader.SupportedSlugs(), item.Slug)
	}))

	return remote, nil
}

func updateCache[T any](repo CacheRepo, key CacheKey, fetch func() (T, error)) error {
	data, err := fetch()
	if err != nil {
		return err
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return repo.Set(key, string(encoded))
}
