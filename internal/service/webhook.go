package service

import (
	"net/http"

	"github.com/libtnb/chix"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
)

type WebHookService struct {
	webhookRepo biz.WebHookRepo
}

func NewWebHookService(webhook biz.WebHookRepo) *WebHookService {
	return &WebHookService{
		webhookRepo: webhook,
	}
}

func (s *WebHookService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	webhooks, total, err := s.webhookRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"total": total,
		"items": webhooks,
	})
}

func (s *WebHookService) Get(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	webhook, err := s.webhookRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, webhook)
}

func (s *WebHookService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebHookCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	webhook, err := s.webhookRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, webhook)
}

func (s *WebHookService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebHookUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.webhookRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *WebHookService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.webhookRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// Call 处理 webhook 调用请求
func (s *WebHookService) Call(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WebHookKey](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 获取 webhook 信息以判断返回格式
	webhook, err := s.webhookRepo.GetByKey(req.Key)
	if err != nil {
		Error(w, http.StatusNotFound, "webhook not found")
		return
	}

	output, err := s.webhookRepo.Call(req.Key)
	if err != nil {
		if webhook.Raw {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(output))
			return
		}
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if webhook.Raw {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(output))
		return
	}

	Success(w, chix.M{
		"output": output,
	})
}
