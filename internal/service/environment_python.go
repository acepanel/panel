package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type EnvironmentPythonService struct {
	environmentBase
}

func NewEnvironmentPythonService(
	environmentUsecase *biz.EnvironmentUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentPythonService {
	return &EnvironmentPythonService{environmentBase{t: t, log: log, environmentRepo: environmentUsecase}}
}

func (s *EnvironmentPythonService) SetCli(w http.ResponseWriter, r *http.Request) {
	s.setCli(w, r, "python")
}

func (s *EnvironmentPythonService) GetMirror(w http.ResponseWriter, r *http.Request) {
	s.getMirror(w, r, "python", s.t.Get("python mirror viewed"))
}

func (s *EnvironmentPythonService) SetMirror(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentMirror](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	s.setMirror(w, r, "python", req.Slug, req.Mirror, s.t.Get("python mirror set"))
}
