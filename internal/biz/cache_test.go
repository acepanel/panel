package biz

import (
	"errors"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/api"
)

type stubCacheRepo struct {
	CacheRepo
	channel string
	latest  *api.Version
	all     api.Versions
	err     error
	failed  CacheKey
	sets    []CacheKey
}

func (s *stubCacheRepo) FetchLatestVersion(channel string) (*api.Version, error) {
	s.channel = channel
	return s.latest, s.err
}

func (s *stubCacheRepo) FetchVersions(channel string) (*api.Versions, error) {
	s.channel = channel
	return &s.all, s.err
}

func (s *stubCacheRepo) FetchCategories() (*api.Categories, error) {
	return &api.Categories{}, s.fail(CacheKeyCategories)
}

func (s *stubCacheRepo) FetchApps() (*api.Apps, error) {
	return &api.Apps{}, s.fail(CacheKeyApps)
}

func (s *stubCacheRepo) FetchEnvironments() (*api.Environments, error) {
	return &api.Environments{}, s.fail(CacheKeyEnvironment)
}

func (s *stubCacheRepo) FetchTemplates() (*api.Templates, error) {
	return &api.Templates{}, s.fail(CacheKeyTemplates)
}

func (s *stubCacheRepo) Set(key CacheKey, _ string) error {
	s.sets = append(s.sets, key)
	return nil
}

func (s *stubCacheRepo) fail(key CacheKey) error {
	if key == s.failed {
		return errors.New("boom")
	}
	return nil
}

func TestCacheUsecaseUpdateAll(t *testing.T) {
	all := []CacheKey{CacheKeyCategories, CacheKeyApps, CacheKeyEnvironment, CacheKeyTemplates}

	repo := new(stubCacheRepo)
	key, err := NewCacheUsecase(repo).UpdateAll()
	must.NoError(t, err)
	check.Equal(t, key, CacheKey(""))
	check.DeepEqual(t, repo.sets, all)

	// 出错即停，返回出错的那一项
	for i, failed := range all {
		repo = &stubCacheRepo{failed: failed}
		key, err = NewCacheUsecase(repo).UpdateAll()
		check.Error(t, err)
		check.Equal(t, key, failed)
		check.DeepEqual(t, append([]CacheKey{}, repo.sets...), all[:i])
	}
}
