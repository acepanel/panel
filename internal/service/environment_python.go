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

type EnvironmentPythonService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func NewEnvironmentPythonService(environmentUsecase *biz.EnvironmentUsecase, t *gotext.Locale, log *slog.Logger) *EnvironmentPythonService {
	return &EnvironmentPythonService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
	}
}

func (s *EnvironmentPythonService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("python", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Python-%s is not installed", req.Slug))
		return
	}

	binPath := fmt.Sprintf("%s/server/python/%s/bin", app.Root, req.Slug)
	if err = linkCLIBinaries(binPath, "python3", "pip3"); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", "python"), slog.String("slug", req.Slug), slog.String("bin_path", binPath))

	Success(w, nil)
}

func (s *EnvironmentPythonService) GetMirror(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("python", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Python-%s is not installed", req.Slug))
		return
	}

	pipBin := fmt.Sprintf("%s/server/python/%s/bin/pip3", app.Root, req.Slug)
	mirror, err := shell.Execf(r.Context(), "%s config --global get global.index-url", pipBin)
	if err != nil {
		mirror = "https://pypi.org/simple"
	}

	s.log.Info(s.t.Get("python mirror viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug))

	Success(w, strings.TrimSpace(mirror))
}

func (s *EnvironmentPythonService) SetMirror(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentMirror](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("python", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Python-%s is not installed", req.Slug))
		return
	}

	pipBin := fmt.Sprintf("%s/server/python/%s/bin/pip3", app.Root, req.Slug)
	if _, err = shell.Execf(r.Context(), "%s config --global set global.index-url %s", pipBin, shell.Quote(req.Mirror)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("python mirror set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("slug", req.Slug), slog.String("mirror", redactURL(req.Mirror)))

	Success(w, nil)
}
