package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
)

type EnvironmentPythonService struct {
	t               *gotext.Locale
	environmentRepo biz.EnvironmentRepo
}

func NewEnvironmentPythonService(t *gotext.Locale, environmentRepo biz.EnvironmentRepo) *EnvironmentPythonService {
	return &EnvironmentPythonService{
		t:               t,
		environmentRepo: environmentRepo,
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
	if err = io.LinkCLIBinaries(binPath, []string{"python3", "pip3"}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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
	mirror, err := shell.Execf("%s config --global get global.index-url", pipBin)
	if err != nil {
		mirror = "https://pypi.org/simple"
	}

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
	if _, err = shell.Execf("%s config --global set global.index-url %s", pipBin, req.Mirror); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
