package service

import (
	"net/http"

	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type NotifyService struct {
	notifyRepo *biz.NotifyUsecase
}

func NewNotifyService(notifyUsecase *biz.NotifyUsecase) *NotifyService {
	return &NotifyService{
		notifyRepo: notifyUsecase,
	}
}

func (s *NotifyService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	channels, total, err := s.notifyRepo.List(r.Context(), req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": channels,
	})
}

func (s *NotifyService) All(w http.ResponseWriter, r *http.Request) {
	channels, err := s.notifyRepo.All()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, channels)
}

func (s *NotifyService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	channel, err := s.notifyRepo.Get(r.Context(), req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, channel)
}

func (s *NotifyService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.NotifyChannelCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	channel, err := s.notifyRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, channel)
}

func (s *NotifyService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.NotifyChannelUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.notifyRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *NotifyService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.notifyRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *NotifyService) Test(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.notifyRepo.Test(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *NotifyService) GetSetting(w http.ResponseWriter, r *http.Request) {
	setting, err := s.notifyRepo.GetSetting(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, setting)
}

func (s *NotifyService) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.NotifySetting](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.notifyRepo.UpdateSetting(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
