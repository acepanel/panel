package service

import (
	"net/http"
	"time"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type MonitorService struct {
	monitorRepo *biz.MonitorUsecase
}

func NewMonitorService(monitorUsecase *biz.MonitorUsecase) *MonitorService {
	return &MonitorService{
		monitorRepo: monitorUsecase,
	}
}

func (s *MonitorService) GetSetting(w http.ResponseWriter, r *http.Request) {
	setting, err := s.monitorRepo.GetSetting(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, setting)
}

func (s *MonitorService) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.MonitorSetting](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.monitorRepo.UpdateSetting(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *MonitorService) Clear(w http.ResponseWriter, r *http.Request) {
	if err := s.monitorRepo.Clear(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *MonitorService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.MonitorList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	detail, err := s.monitorRepo.Detail(time.UnixMilli(req.Start), time.UnixMilli(req.End))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, detail)
}
