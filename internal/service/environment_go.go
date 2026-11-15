package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type EnvironmentGoService struct {
	environmentBase
}

func NewEnvironmentGoService(
	environmentUsecase *biz.EnvironmentUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentGoService {
	return &EnvironmentGoService{environmentBase{t: t, log: log, environmentRepo: environmentUsecase}}
}

func (s *EnvironmentGoService) SetCli(w http.ResponseWriter, r *http.Request) {
	s.setCli(w, r, "go")
}

func (s *EnvironmentGoService) GetProxy(w http.ResponseWriter, r *http.Request) {
	s.getMirror(w, r, "go", s.t.Get("go proxy viewed"))
}

func (s *EnvironmentGoService) SetProxy(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentProxy](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	s.setMirror(w, r, "go", req.Slug, req.Proxy, s.t.Get("go proxy set"))
}
