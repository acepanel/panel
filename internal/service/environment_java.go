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

type EnvironmentJavaService struct {
	t               *gotext.Locale
	environmentRepo biz.EnvironmentRepo
}

func NewEnvironmentJavaService(t *gotext.Locale, environmentRepo biz.EnvironmentRepo) *EnvironmentJavaService {
	return &EnvironmentJavaService{
		t:               t,
		environmentRepo: environmentRepo,
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
	if err = io.LinkCLIBinaries(binPath, []string{"java", "javac", "jar", "jshell"}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
