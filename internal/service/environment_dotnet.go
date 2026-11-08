package service

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type EnvironmentDotnetService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func NewEnvironmentDotnetService(environmentUsecase *biz.EnvironmentUsecase, t *gotext.Locale, log *slog.Logger) *EnvironmentDotnetService {
	return &EnvironmentDotnetService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
	}
}

func (s *EnvironmentDotnetService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("dotnet", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get(".NET-%s is not installed", req.Slug))
		return
	}

	binPath := fmt.Sprintf("%s/server/dotnet/%s", app.Root, req.Slug)
	if err = linkCLIBinaries(binPath, "dotnet"); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", "dotnet"), slog.String("slug", req.Slug), slog.String("bin_path", binPath))

	Success(w, nil)
}
