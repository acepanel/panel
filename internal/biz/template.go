package biz

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/types"
)

type TemplateRepo interface {
	LoadLocalTemplates() api.Templates
	Callback(slug string) error
	WriteCompose(name, compose string, envs []types.KV) (string, error)
}

type TemplateUsecase struct {
	repo  TemplateRepo
	cache CacheRepo
	t     *gotext.Locale
	log   *slog.Logger
}

func NewTemplateUsecase(t *gotext.Locale, log *slog.Logger, cacheRepo CacheRepo, templateRepo TemplateRepo) *TemplateUsecase {
	return &TemplateUsecase{
		repo:  templateRepo,
		cache: cacheRepo,
		t:     t,
		log:   log,
	}
}

// List 获取所有模版，包括本地模板
func (uc *TemplateUsecase) List() api.Templates {
	templates := make(api.Templates, 0)
	cached, err := uc.cache.Get(CacheKeyTemplates)
	if err == nil {
		_ = json.Unmarshal([]byte(cached), &templates)
	}

	// 加载本地模板并合并，本地模板覆盖同 slug 的远端模板
	localTemplates := uc.repo.LoadLocalTemplates()
	if len(localTemplates) > 0 {
		slugMap := make(map[string]int, len(templates))
		for i, t := range templates {
			slugMap[t.Slug] = i
		}
		for _, lt := range localTemplates {
			if i, ok := slugMap[lt.Slug]; ok {
				templates[i] = lt
			} else {
				templates = append(templates, lt)
			}
		}
	}

	return templates
}

// Get 获取模版详情
func (uc *TemplateUsecase) Get(slug string) (*api.Template, error) {
	templates := uc.List()

	for _, t := range templates {
		if t.Slug == slug {
			return t, nil
		}
	}

	return nil, errors.New(uc.t.Get("template %s not found", slug))
}

// Callback 模版下载回调
func (uc *TemplateUsecase) Callback(slug string) error {
	return uc.repo.Callback(slug)
}

// CreateCompose 创建编排
func (uc *TemplateUsecase) CreateCompose(ctx context.Context, slug, name, compose string, envs []types.KV) (string, error) {
	dir := filepath.Join(app.Root, "compose", name)

	// 检查编排是否已存在
	if _, err := os.Stat(dir); err == nil {
		return "", errors.New(uc.t.Get("compose %s already exists", name))
	}

	dir, err := uc.repo.WriteCompose(name, compose, envs)
	if err != nil {
		return "", err
	}

	uc.log.Info(uc.t.Get("compose created from template"), slog.String("type", OperationTypeContainer), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name), slog.String("template", slug))
	return dir, nil
}
