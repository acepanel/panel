package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

type WafService struct {
	wafRepo *biz.WafUsecase
	t       *gotext.Locale
}

func NewWafService(wafUsecase *biz.WafUsecase, t *gotext.Locale) (*WafService, error) {
	return &WafService{
		wafRepo: wafUsecase,
		t:       t,
	}, nil
}

// ===================== 策略透传 =====================

func (s *WafService) ListPolicies(w http.ResponseWriter, _ *http.Request) {
	data, err := s.wafRepo.ListPolicies()
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) GetPolicy(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.GetPolicy(chi.URLParam(r, "policy_id"))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) GetPolicyStatus(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.GetPolicyStatus(chi.URLParam(r, "policy_id"))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	data, err := s.wafRepo.CreatePolicy(body)
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	data, err := s.wafRepo.UpdatePolicy(chi.URLParam(r, "policy_id"), body)
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	if err := s.wafRepo.DeletePolicy(chi.URLParam(r, "policy_id")); err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, nil)
}

// ===================== 决策透传 =====================

func (s *WafService) ListDecisions(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.ListDecisions(flattenQuery(r))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) CreateDecision(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WafDecisionCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	value, scope, ok := normalizeDecisionValue(req.Value)
	if !ok {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("decision value must be a valid IP address or CIDR"))
		return
	}
	if req.Scope != "" && req.Scope != scope {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("decision scope does not match value; expected %s", scope))
		return
	}
	req.Value = value
	req.Scope = scope

	data, err := s.wafRepo.CreateDecision(req)
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func normalizeDecisionValue(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil && addr.Zone() == "" {
		return addr.String(), "ip", true
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), "range", true
	}
	return "", "", false
}

func (s *WafService) DeleteDecision(w http.ResponseWriter, r *http.Request) {
	if err := s.wafRepo.DeleteDecision(flattenQuery(r)); err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, nil)
}

// ===================== 误报加白透传 =====================

func (s *WafService) ListExclusions(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.ListExclusions(chi.URLParam(r, "policy_id"))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) CreateExclusion(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	data, err := s.wafRepo.CreateExclusion(chi.URLParam(r, "policy_id"), body)
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) DeleteExclusion(w http.ResponseWriter, r *http.Request) {
	if err := s.wafRepo.DeleteExclusion(chi.URLParam(r, "policy_id"), flattenQuery(r)); err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, nil)
}

// ===================== 报表透传 =====================

func (s *WafService) Events(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.Events(flattenQuery(r))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) Stats(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.Stats(flattenQuery(r))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

func (s *WafService) AttackMap(w http.ResponseWriter, r *http.Request) {
	data, err := s.wafRepo.AttackMap(flattenQuery(r))
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, data)
}

// ===================== 网站绑定 + 启停 =====================

func (s *WafService) ListBindings(w http.ResponseWriter, _ *http.Request) {
	bindings, err := s.wafRepo.ListBindings()
	if err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, bindings)
}

// EnableWebsite 网站启用 WAF
func (s *WafService) EnableWebsite(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.WafWebsiteToggle](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.wafRepo.EnableWebsite(req); err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, nil)
}

// DisableWebsite 网站关闭 WAF
func (s *WafService) DisableWebsite(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.wafRepo.DisableWebsite(req.ID); err != nil {
		writeWafError(w, err)
		return
	}

	Success(w, nil)
}

// decodeBody 将请求体原样解析为 any（透传 JSON，对 agent 字段新增保持健壮）
func decodeBody(r *http.Request) (any, error) {
	if r.Body == nil || r.ContentLength == 0 {
		return nil, nil
	}
	var body any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body, nil
}

// flattenQuery 把 URL 查询参数展平为 map[string]string（透传给 agent）
func flattenQuery(r *http.Request) map[string]string {
	query := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			query[key] = values[0]
		}
	}
	return query
}

func writeWafError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	var statusError interface{ StatusCode() int }
	if errors.As(err, &statusError) {
		code = statusError.StatusCode()
	}
	Error(w, code, "%v", err)
}
