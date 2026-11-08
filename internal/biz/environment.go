package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/types"
)

type EnvironmentRepo interface {
	IsInstalled(typ, slug string) bool
	InstalledVersion(ctx context.Context, typ, slug string) string
	ScriptCommand(typ, action, slug, version string) string
	ExecScript(ctx context.Context, cmd string) error
}

type EnvironmentUsecase struct {
	repo  EnvironmentRepo
	cache CacheRepo
	task  TaskRepo
	t     *gotext.Locale
	log   *slog.Logger
}

func NewEnvironmentUsecase(t *gotext.Locale, log *slog.Logger, cacheRepo CacheRepo, environmentRepo EnvironmentRepo, taskRepo TaskRepo) *EnvironmentUsecase {
	return &EnvironmentUsecase{
		repo:  environmentRepo,
		cache: cacheRepo,
		task:  taskRepo,
		t:     t,
		log:   log,
	}
}

func (uc *EnvironmentUsecase) Types() []types.LV {
	return []types.LV{
		{Label: "Go", Value: "go"},
		{Label: "Java", Value: "java"},
		{Label: "Node.js", Value: "nodejs"},
		{Label: "PHP", Value: "php"},
		{Label: "Python", Value: "python"},
		{Label: ".NET", Value: "dotnet"},
	}
}

func (uc *EnvironmentUsecase) All(typ ...string) api.Environments {
	cached, err := uc.cache.Get(CacheKeyEnvironment)
	if err != nil {
		return nil
	}
	var environments api.Environments
	if err = json.Unmarshal([]byte(cached), &environments); err != nil {
		return nil
	}

	// 过滤
	environments = slices.DeleteFunc(environments, func(env *api.Environment) bool {
		return len(typ) > 0 && typ[0] != "" && env.Type != typ[0]
	})

	return environments
}

func (uc *EnvironmentUsecase) getByTypeAndSlug(typ, slug string) *api.Environment {
	all := uc.All()
	for _, env := range all {
		if env.Type == typ && env.Slug == slug {
			return env
		}
	}
	return nil
}

func (uc *EnvironmentUsecase) IsInstalled(typ, slug string) bool {
	return uc.repo.IsInstalled(typ, slug)
}

func (uc *EnvironmentUsecase) InstalledSlugs(typ string) []string {
	var slugs []string
	all := uc.All()
	for _, env := range all {
		if env.Type != typ {
			continue
		}
		if uc.repo.IsInstalled(typ, env.Slug) {
			slugs = append(slugs, env.Slug)
		}
	}
	return slugs
}

func (uc *EnvironmentUsecase) InstalledVersion(ctx context.Context, typ, slug string) string {
	return uc.repo.InstalledVersion(ctx, typ, slug)
}

func (uc *EnvironmentUsecase) HasUpdate(ctx context.Context, typ, slug string) bool {
	if !uc.repo.IsInstalled(typ, slug) {
		return false
	}
	env := uc.getByTypeAndSlug(typ, slug)
	if env == nil {
		return false
	}

	mainlineVersion := env.Version
	installedVersion := uc.repo.InstalledVersion(ctx, typ, slug)

	return mainlineVersion != installedVersion && mainlineVersion != "" && installedVersion != ""
}

func (uc *EnvironmentUsecase) Install(ctx context.Context, typ, slug string) error {
	if installed := uc.repo.IsInstalled(typ, slug); installed {
		return errors.New(uc.t.Get("environment %s-%s already installed", typ, slug))
	}
	return uc.do(ctx, typ, slug, "install")
}

func (uc *EnvironmentUsecase) Uninstall(ctx context.Context, typ, slug string) error {
	if installed := uc.repo.IsInstalled(typ, slug); !installed {
		return errors.New(uc.t.Get("environment %s-%s not installed", typ, slug))
	}
	return uc.do(ctx, typ, slug, "uninstall")
}

func (uc *EnvironmentUsecase) Update(ctx context.Context, typ, slug string) error {
	if installed := uc.repo.IsInstalled(typ, slug); !installed {
		return errors.New(uc.t.Get("environment %s-%s not installed", typ, slug))
	}
	return uc.do(ctx, typ, slug, "update")
}

func (uc *EnvironmentUsecase) do(ctx context.Context, typ, slug, action string) error {
	env := uc.getByTypeAndSlug(typ, slug)
	if env == nil {
		return fmt.Errorf("environment not found: %s-%s", typ, slug)
	}

	cmd := uc.repo.ScriptCommand(typ, action, env.Slug, env.Version)

	var name, msg string
	logf := uc.log.Info
	switch action {
	case "install":
		name = uc.t.Get("Install environment %s", env.Name)
		msg = uc.t.Get("environment installed")
	case "uninstall":
		name = uc.t.Get("Uninstall environment %s", env.Name)
		msg = uc.t.Get("environment uninstalled")
		logf = uc.log.Warn
	case "update":
		name = uc.t.Get("Update environment %s", env.Name)
		msg = uc.t.Get("environment updated")
	}

	if app.IsCli {
		if err := uc.repo.ExecScript(ctx, cmd); err != nil {
			return err
		}
	} else {
		task := new(Task)
		task.Key = fmt.Sprintf("environment:%s:%s", typ, slug)
		task.Name = name
		task.Status = TaskStatusWaiting
		task.Shell = cmd
		if err := uc.task.Push(task); err != nil {
			return err
		}
	}

	logf(msg, slog.String("type", OperationTypeEnvironment), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("environment_type", typ), slog.String("slug", slug), slog.String("name", env.Name), slog.String("version", env.Version))

	return nil
}
