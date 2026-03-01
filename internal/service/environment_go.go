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

type EnvironmentGoService struct {
	t               *gotext.Locale
	environmentRepo biz.EnvironmentRepo
}

func NewEnvironmentGoService(t *gotext.Locale, environmentRepo biz.EnvironmentRepo) *EnvironmentGoService {
	return &EnvironmentGoService{
		t:               t,
		environmentRepo: environmentRepo,
	}
}

func (s *EnvironmentGoService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("go", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Go-%s is not installed", req.Slug))
		return
	}

	binPath := fmt.Sprintf("%s/server/go/%s/bin", app.Root, req.Slug)
	if err = io.LinkCLIBinaries(binPath, []string{"go", "gofmt"}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentGoService) GetProxy(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("go", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Go-%s is not installed", req.Slug))
		return
	}

	goBin := fmt.Sprintf("%s/server/go/%s/bin/go", app.Root, req.Slug)
	proxy, err := shell.Execf("%s env GOPROXY", goBin)
	if err != nil {
		proxy = "https://proxy.golang.org,direct"
	}

	Success(w, strings.TrimSpace(proxy))
}

func (s *EnvironmentGoService) SetProxy(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentProxy](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("go", req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Go-%s is not installed", req.Slug))
		return
	}

	goBin := fmt.Sprintf("%s/server/go/%s/bin/go", app.Root, req.Slug)
	if _, err = shell.Execf("%s env -w GOPROXY=%s", goBin, req.Proxy); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
