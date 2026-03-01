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

type EnvironmentNodejsService struct {
	t               *gotext.Locale
	environmentRepo biz.EnvironmentRepo
}

func NewEnvironmentNodejsService(t *gotext.Locale, environmentRepo biz.EnvironmentRepo) *EnvironmentNodejsService {
	return &EnvironmentNodejsService{
		t:               t,
		environmentRepo: environmentRepo,
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
	if err = io.LinkCLIBinaries(binPath, []string{"node", "npm", "npx", "corepack"}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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
	registry, err := shell.Execf("%s config get --global registry", npmBin)
	if err != nil {
		registry = "https://registry.npmjs.org/"
	}

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
	if _, err = shell.Execf("%s config set --global registry %s", npmBin, req.Registry); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
