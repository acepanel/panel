package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type EnvironmentNodejsService struct {
	environmentBase
}

func NewEnvironmentNodejsService(
	environmentUsecase *biz.EnvironmentUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentNodejsService {
	return &EnvironmentNodejsService{environmentBase{t: t, log: log, environmentRepo: environmentUsecase}}
}

func (s *EnvironmentNodejsService) SetCli(w http.ResponseWriter, r *http.Request) {
	s.setCli(w, r, "nodejs")
}

func (s *EnvironmentNodejsService) GetRegistry(w http.ResponseWriter, r *http.Request) {
	s.getMirror(w, r, "nodejs", s.t.Get("nodejs registry viewed"))
}

func (s *EnvironmentNodejsService) SetRegistry(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentRegistry](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	s.setMirror(w, r, "nodejs", req.Slug, req.Registry, s.t.Get("nodejs registry set"))
}
