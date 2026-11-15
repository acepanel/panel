package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

type EnvironmentJavaService struct {
	environmentBase
}

func NewEnvironmentJavaService(
	environmentUsecase *biz.EnvironmentUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentJavaService {
	return &EnvironmentJavaService{environmentBase{t: t, log: log, environmentRepo: environmentUsecase}}
}

func (s *EnvironmentJavaService) SetCli(w http.ResponseWriter, r *http.Request) {
	s.setCli(w, r, "java")
}
