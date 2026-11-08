package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type AlertService struct {
	alertRepo *biz.AlertUsecase
}

func NewAlertService(alertUsecase *biz.AlertUsecase) *AlertService {
	return &AlertService{
		alertRepo: alertUsecase,
	}
}

func (s *AlertService) ListRules(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	rules, total, err := s.alertRepo.ListRules(r.Context(), req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": rules,
	})
}

func (s *AlertService) GetRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	rule, err := s.alertRepo.GetRule(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, rule)
}

func (s *AlertService) CreateRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AlertRuleCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	rule, err := s.alertRepo.CreateRule(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, rule)
}

func (s *AlertService) UpdateRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AlertRuleUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.alertRepo.UpdateRule(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AlertService) DeleteRule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.alertRepo.DeleteRule(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AlertService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	alerts, total, err := s.alertRepo.ListAlerts(r.Context(), req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": alerts,
	})
}

func (s *AlertService) Clear(w http.ResponseWriter, r *http.Request) {
	if err := s.alertRepo.ClearAlerts(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
