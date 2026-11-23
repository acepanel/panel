package service

import (
	"log/slog"
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type FirewallSubscriptionService struct {
	t                *gotext.Locale
	log              *slog.Logger
	subscriptionRepo *biz.FirewallSubscriptionUsecase
}

func NewFirewallSubscriptionService(
	t *gotext.Locale,
	log *slog.Logger,
	subscriptionUsecase *biz.FirewallSubscriptionUsecase,
) *FirewallSubscriptionService {
	return &FirewallSubscriptionService{
		t:                t,
		log:              log,
		subscriptionRepo: subscriptionUsecase,
	}
}

func (s *FirewallSubscriptionService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	subs, total, err := s.subscriptionRepo.List(req.Page, req.Limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("firewall subscription list viewed"), slog.String("type", biz.OperationTypeFirewall), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("page", uint64(req.Page)), slog.Uint64("limit", uint64(req.Limit)))

	Success(w, chix.M{
		"total": total,
		"items": subs,
	})
}

func (s *FirewallSubscriptionService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallSubscriptionCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	sub, err := s.subscriptionRepo.Create(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, sub)
}

func (s *FirewallSubscriptionService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FirewallSubscriptionUpdate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.subscriptionRepo.Update(r.Context(), req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FirewallSubscriptionService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.subscriptionRepo.Delete(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// Refresh 立即更新订阅
func (s *FirewallSubscriptionService) Refresh(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.subscriptionRepo.Refresh(r.Context(), req.ID); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
