package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type EnvironmentNodejsService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func NewEnvironmentNodejsService(environmentUsecase *biz.EnvironmentUsecase, t *gotext.Locale, log *slog.Logger) *EnvironmentNodejsService {
	return &EnvironmentNodejsService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
	}
}

func (s *EnvironmentNodejsService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("nodejs", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Node.js-%s is not installed", req.Slug))
		return
	}

	binPath := fmt.Sprintf("%s/server/nodejs/%s/bin", app.Root, req.Slug)
	if err = linkCLIBinaries(binPath, "node", "npm", "npx", "corepack"); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", "nodejs"), slog.String("slug", req.Slug), slog.String("bin_path", binPath))

	Success(w, nil)
}

func (s *EnvironmentNodejsService) GetRegistry(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("nodejs", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Node.js-%s is not installed", req.Slug))
		return
	}

	npmBin := fmt.Sprintf("%s/server/nodejs/%s/bin/npm", app.Root, req.Slug)
	registry, err := shell.Execf(r.Context(), "%s config get --global registry", npmBin)
	if err != nil {
		registry = "https://registry.npmjs.org/"
	}

	s.log.Info(s.t.Get("nodejs registry viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug))

	Success(w, strings.TrimSpace(registry))
}

func (s *EnvironmentNodejsService) SetRegistry(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentRegistry](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("nodejs", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Node.js-%s is not installed", req.Slug))
		return
	}

	npmBin := fmt.Sprintf("%s/server/nodejs/%s/bin/npm", app.Root, req.Slug)
	if _, err = shell.Execf(r.Context(), "%s config set --global registry %s", npmBin, shell.Quote(req.Registry)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("nodejs registry set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug), slog.String("registry", redactURL(req.Registry)))

	Success(w, nil)
}
