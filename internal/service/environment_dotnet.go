package service

import (
	"fmt"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/io"
)

type EnvironmentDotnetService struct {
	t               *gotext.Locale
	environmentRepo biz.EnvironmentRepo
}

func NewEnvironmentDotnetService(t *gotext.Locale, environmentRepo biz.EnvironmentRepo) *EnvironmentDotnetService {
	return &EnvironmentDotnetService{
		t:               t,
		environmentRepo: environmentRepo,
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
	if err = io.LinkCLIBinaries(binPath, []string{"dotnet"}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
