package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

type EnvironmentDotnetService struct {
	environmentBase
}

func NewEnvironmentDotnetService(
	environmentUsecase *biz.EnvironmentUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentDotnetService {
	return &EnvironmentDotnetService{environmentBase{t: t, log: log, environmentRepo: environmentUsecase}}
}

func (s *EnvironmentDotnetService) SetCli(w http.ResponseWriter, r *http.Request) {
	s.setCli(w, r, "dotnet")
}
