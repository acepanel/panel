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

type EnvironmentJavaService struct {
	t               *gotext.Locale
	log             *slog.Logger
	environmentRepo *biz.EnvironmentUsecase
}

func NewEnvironmentJavaService(environmentUsecase *biz.EnvironmentUsecase, t *gotext.Locale, log *slog.Logger) *EnvironmentJavaService {
	return &EnvironmentJavaService{
		t:               t,
		log:             log,
		environmentRepo: environmentUsecase,
	}
}

func (s *EnvironmentJavaService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("java", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Java-%s is not installed", req.Slug))
		return
	}

	binPath := fmt.Sprintf("%s/server/java/%s/bin", app.Root, req.Slug)
	if err = linkCLIBinaries(binPath, "java", "javac", "jar", "jshell"); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", "java"), slog.String("slug", req.Slug), slog.String("bin_path", binPath))

	Success(w, nil)
}
