package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webservertypes "github.com/acepanel/panel/v3/pkg/webserver/types"
)

// wafConfigName WAF 站点配置片段文件名（序号靠后，确保在基础指令之后加载）
const wafConfigName = "011-waf.conf"

// wafManageSocket 本地 acewaf 管理 unix socket（单机直连，socket 文件权限鉴权）
const wafManageSocket = "/opt/ace/waf/run/acewaf-manage.sock"

type wafRepo struct {
	t       *gotext.Locale
	db      *gorm.DB
	log     *slog.Logger
	setting biz.SettingRepo
	client  *resty.Client
	bindMu  sync.Mutex
}

func NewWafRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger, setting biz.SettingRepo) (biz.WafRepo, error) {
	client := resty.New()
	client.SetTimeout(65 * time.Second)
	client.SetTransport(&http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", wafManageSocket)
		},
	})
	client.SetBaseURL("http://acewaf")

	return &wafRepo{
		t:       t,
		db:      db,
		log:     log,
		setting: setting,
		client:  client,
	}, nil
}

// ===================== agent HTTP 透传（本地 unix socket） =====================

type wafError struct {
	status  int
	message string
}

func (e *wafError) Error() string {
	return e.message
}

func (e *wafError) StatusCode() int {
	return e.status
}

// do 通用透传：发起请求并把 agent 的 JSON 原样解析为 any 返回，对 agent 字段新增保持健壮
func (r *wafRepo) do(method, path string, query map[string]string, body any) (any, error) {
	req := r.client.R()
	if len(query) > 0 {
		req.SetQueryParams(query)
	}
	if body != nil {
		req.SetHeader("Content-Type", "application/json").SetBody(body)
	}

	resp, err := req.Execute(method, path)
	if err != nil {
		return nil, errors.New(r.t.Get("failed to request acewaf: %v", err))
	}
	if !resp.IsStatusSuccess() {
		return nil, &wafError{
			status:  resp.StatusCode(),
			message: r.t.Get("acewaf returned an error (%d): %s", resp.StatusCode(), resp.String()),
		}
	}
	if len(resp.Bytes()) == 0 {
		return nil, nil
	}

	var result any
	decoder := json.NewDecoder(bytes.NewReader(resp.Bytes()))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, errors.New(r.t.Get("failed to parse acewaf response: %v", err))
	}
	return result, nil
}

func (r *wafRepo) ListPolicies() (any, error) {
	data, err := r.do(resty.MethodGet, "/api/policies", nil, nil)
	if err != nil {
		return nil, err
	}
	return r.withPolicyApplyState(data)
}

func (r *wafRepo) GetPolicy(policyID string) (any, error) {
	data, err := r.do(resty.MethodGet, fmt.Sprintf("/api/policies/%s", policyID), nil, nil)
	if err != nil {
		return nil, err
	}
	return r.withPolicyApplyState(data)
}

func (r *wafRepo) GetPolicyStatus(policyID string) (any, error) {
	data, err := r.do(resty.MethodGet, fmt.Sprintf("/api/policies/%s/status", policyID), nil, nil)
	if err != nil {
		return nil, err
	}
	return r.withPolicyApplyState(data)
}

func (r *wafRepo) CreatePolicy(body any) (any, error) {
	data, err := r.do(resty.MethodPost, "/api/policies", nil, body)
	if err != nil {
		return nil, err
	}
	return normalizePolicyStatus(data, nil), nil
}

func (r *wafRepo) UpdatePolicy(policyID string, body any) (any, error) {
	boundPolicyIDs, err := r.boundPolicyIDs()
	if err != nil {
		return nil, err
	}

	data, err := r.do(resty.MethodPut, fmt.Sprintf("/api/policies/%s", policyID), nil, body)
	if err != nil {
		return nil, err
	}
	return normalizePolicyStatus(data, boundPolicyIDs), nil
}

func (r *wafRepo) withPolicyApplyState(data any) (any, error) {
	boundPolicyIDs, err := r.boundPolicyIDs()
	if err != nil {
		return nil, err
	}
	return normalizePolicyStatus(data, boundPolicyIDs), nil
}

func (r *wafRepo) boundPolicyIDs() (map[uint64]struct{}, error) {
	var policyIDs []uint64
	if err := r.db.Model(&biz.WafBinding{}).
		Where("enabled = ?", true).
		Distinct().
		Pluck("policy_id", &policyIDs).Error; err != nil {
		return nil, err
	}

	boundPolicyIDs := make(map[uint64]struct{}, len(policyIDs))
	for _, policyID := range policyIDs {
		boundPolicyIDs[policyID] = struct{}{}
	}
	return boundPolicyIDs, nil
}

func normalizePolicyStatus(data any, boundPolicyIDs map[uint64]struct{}) any {
	normalize := func(policy map[string]any) {
		policyID, ok := policyIDFromResponse(policy)
		if !ok {
			return
		}
		if _, ok := policy["target_version"]; !ok {
			if _, hasAppliedVersion := policy["applied_version"]; hasAppliedVersion {
				policy["target_version"] = policy["version"]
			} else {
				policy["target_version"] = nil
			}
		}
		for _, field := range []string{"applied_version", "last_error"} {
			if _, ok := policy[field]; !ok {
				policy[field] = nil
			}
		}

		_, bound := boundPolicyIDs[policyID]
		policy["apply_status"] = policyApplyState(policy, bound)
	}

	switch value := data.(type) {
	case map[string]any:
		normalize(value)
		if items, ok := value["items"].([]any); ok {
			for _, item := range items {
				if policy, ok := item.(map[string]any); ok {
					normalize(policy)
				}
			}
		}
	case []any:
		for _, item := range value {
			if policy, ok := item.(map[string]any); ok {
				normalize(policy)
			}
		}
	}

	return data
}

func policyIDFromResponse(policy map[string]any) (uint64, bool) {
	if policyID, ok := uint64FromJSON(policy["id"]); ok {
		return policyID, true
	}
	return uint64FromJSON(policy["policy_id"])
}

func uint64FromJSON(value any) (uint64, bool) {
	switch number := value.(type) {
	case uint64:
		return number, true
	case int:
		if number >= 0 {
			return uint64(number), true
		}
	case float64:
		if number >= 0 && number < math.MaxUint64 && math.Trunc(number) == number {
			return uint64(number), true
		}
	case json.Number:
		parsed, err := strconv.ParseUint(number.String(), 10, 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseUint(number, 10, 64)
		return parsed, err == nil
	}
	return 0, false
}

func policyApplyState(policy map[string]any, bound bool) biz.WafPolicyApplyState {
	if !bound {
		return biz.WafPolicyApplyStateSaved
	}
	if lastError, ok := policy["last_error"].(string); ok && lastError != "" {
		return biz.WafPolicyApplyStateFailed
	}

	targetVersion, hasTarget := uint64FromJSON(policy["target_version"])
	appliedVersion, hasApplied := uint64FromJSON(policy["applied_version"])
	if hasTarget && targetVersion > 0 && hasApplied && appliedVersion >= targetVersion {
		return biz.WafPolicyApplyStateApplied
	}
	return biz.WafPolicyApplyStatePending
}

func (r *wafRepo) DeletePolicy(policyID string) error {
	r.bindMu.Lock()
	defer r.bindMu.Unlock()

	// 删除前校验该策略是否仍被网站绑定，有引用则拒绝，避免站点配置 waf_policy 指向已删策略
	pid, err := strconv.ParseUint(policyID, 10, 64)
	if err != nil {
		return &wafError{status: http.StatusBadRequest, message: r.t.Get("invalid policy id: %s", policyID)}
	}
	var count int64
	if err = r.db.Model(&biz.WafBinding{}).Where("policy_id = ?", pid).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return &wafError{
			status:  http.StatusConflict,
			message: r.t.Get("the policy is in use by websites, please disable WAF for those websites first"),
		}
	}

	_, err = r.do(resty.MethodDelete, fmt.Sprintf("/api/policies/%s", policyID), nil, nil)
	return err
}

func (r *wafRepo) ListDecisions(query map[string]string) (any, error) {
	return r.do(resty.MethodGet, "/api/decisions", query, nil)
}

func (r *wafRepo) CreateDecision(req *request.WafDecisionCreate) (any, error) {
	return r.do(resty.MethodPost, "/api/decisions", nil, req)
}

func (r *wafRepo) DeleteDecision(query map[string]string) error {
	// agent 用 RESTful 路径 DELETE /api/decisions/{id}，面板对外仍收 ?id= 查询，这里转成路径参
	decisionID := query["id"]
	if decisionID == "" {
		return &wafError{status: http.StatusBadRequest, message: r.t.Get("missing required parameter: id")}
	}
	_, err := r.do(resty.MethodDelete, fmt.Sprintf("/api/decisions/%s", decisionID), nil, nil)
	return err
}

func (r *wafRepo) ListExclusions(policyID string) (any, error) {
	return r.do(resty.MethodGet, fmt.Sprintf("/api/policies/%s/exclusions", policyID), nil, nil)
}

func (r *wafRepo) CreateExclusion(policyID string, body any) (any, error) {
	return r.do(resty.MethodPost, fmt.Sprintf("/api/policies/%s/exclusions", policyID), nil, body)
}

func (r *wafRepo) DeleteExclusion(policyID string, query map[string]string) error {
	// agent 用 RESTful 路径 DELETE /api/policies/{id}/exclusions/{eid}，面板对外仍收 ?id= 查询
	exclusionID := query["id"]
	if exclusionID == "" {
		return &wafError{status: http.StatusBadRequest, message: r.t.Get("missing required parameter: id")}
	}
	_, err := r.do(resty.MethodDelete, fmt.Sprintf("/api/policies/%s/exclusions/%s", policyID, exclusionID), nil, nil)
	return err
}

func (r *wafRepo) Events(query map[string]string) (any, error) {
	return r.do(resty.MethodGet, "/api/events", query, nil)
}

func (r *wafRepo) Stats(query map[string]string) (any, error) {
	return r.do(resty.MethodGet, "/api/stats", query, nil)
}

func (r *wafRepo) AttackMap(query map[string]string) (any, error) {
	return r.do(resty.MethodGet, "/api/attack-map", query, nil)
}

// ===================== 网站绑定 + nginx 启停 =====================

func (r *wafRepo) ListBindings() ([]*biz.WafBinding, error) {
	bindings := make([]*biz.WafBinding, 0)
	if err := r.db.Model(&biz.WafBinding{}).Order("id DESC").Find(&bindings).Error; err != nil {
		return nil, err
	}

	// 补充网站名称（仅显示）
	for _, binding := range bindings {
		website := new(biz.Website)
		if err := r.db.Select("name").Where("id = ?", binding.WebsiteID).First(website).Error; err == nil {
			binding.WebsiteName = website.Name
		}
	}

	return bindings, nil
}

// EnableWebsite 为网站启用 WAF：写 site/011-waf.conf 并 reload
func (r *wafRepo) EnableWebsite(req *request.WafWebsiteToggle) error {
	r.bindMu.Lock()
	defer r.bindMu.Unlock()

	website := new(biz.Website)
	if err := r.db.Where("id = ?", req.WebsiteID).First(website).Error; err != nil {
		return err
	}

	vhost, err := r.getVhost(website)
	if err != nil {
		return err
	}
	if _, err = r.GetPolicy(strconv.FormatUint(req.PolicyID, 10)); err != nil {
		return err
	}

	previousConfig := vhost.Config(wafConfigName, webservertypes.ScopeSite)
	content := fmt.Sprintf("waf on;\nwaf_policy %d;", req.PolicyID)
	if err = r.writeWafConfig(vhost, content, false); err != nil {
		return r.restoreWafConfig(vhost, previousConfig, err)
	}
	if err = r.reloadWebServer(); err != nil {
		return r.restoreWafConfig(vhost, previousConfig, err)
	}

	binding := &biz.WafBinding{
		WebsiteID: req.WebsiteID,
		PolicyID:  req.PolicyID,
		Enabled:   true,
	}
	if err = r.db.Transaction(func(tx *gorm.DB) error {
		var exists int64
		if e := tx.Model(&biz.Website{}).Where("id = ?", req.WebsiteID).Count(&exists).Error; e != nil {
			return e
		}
		if exists == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "website_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"policy_id": req.PolicyID, "enabled": true, "updated_at": time.Now(),
			}),
		}).Create(binding).Error
	}); err != nil {
		return r.restoreWafConfig(vhost, previousConfig, err)
	}

	return nil
}

// DisableWebsite 为网站关闭 WAF：移除 site/011-waf.conf 并 reload
func (r *wafRepo) DisableWebsite(websiteID uint) error {
	r.bindMu.Lock()
	defer r.bindMu.Unlock()

	website := new(biz.Website)
	if err := r.db.Where("id = ?", websiteID).First(website).Error; err != nil {
		return err
	}

	vhost, err := r.getVhost(website)
	if err != nil {
		return err
	}

	previousConfig := vhost.Config(wafConfigName, webservertypes.ScopeSite)
	if previousConfig != "" {
		if err = r.writeWafConfig(vhost, "", false); err != nil {
			return r.restoreWafConfig(vhost, previousConfig, err)
		}
		if err = r.reloadWebServer(); err != nil {
			return r.restoreWafConfig(vhost, previousConfig, err)
		}
	}

	if err = r.db.Where("website_id = ?", websiteID).Delete(&biz.WafBinding{}).Error; err != nil {
		if previousConfig != "" {
			return r.restoreWafConfig(vhost, previousConfig, err)
		}
		return err
	}

	return nil
}

// getVhost 获取网站 vhost（与 website 仓库一致的类型分派）
func (r *wafRepo) getVhost(website *biz.Website) (webservertypes.Vhost, error) {
	webServer, err := r.setting.Get(biz.SettingKeyWebserver)
	if err != nil {
		return nil, err
	}
	if webServer != "nginx" {
		return nil, &wafError{status: http.StatusConflict, message: r.t.Get("WAF requires nginx")}
	}

	configDir := filepath.Join(app.Root, "sites", website.Name, "config")
	switch website.Type {
	case biz.WebsiteTypeProxy:
		return webserver.NewProxyVhost(webserver.TypeNginx, configDir)
	case biz.WebsiteTypePHP:
		return webserver.NewPHPVhost(webserver.TypeNginx, configDir)
	case biz.WebsiteTypeStatic:
		return webserver.NewStaticVhost(webserver.TypeNginx, configDir)
	default:
		return nil, errors.New(r.t.Get("unsupported website type: %s", website.Type))
	}
}

func (r *wafRepo) writeWafConfig(vhost webservertypes.Vhost, content string, raw bool) error {
	var err error
	if content == "" {
		err = vhost.RemoveConfig(wafConfigName, webservertypes.ScopeSite)
	} else if raw {
		err = vhost.SetRawConfig(wafConfigName, webservertypes.ScopeSite, content)
	} else {
		err = vhost.SetConfig(wafConfigName, webservertypes.ScopeSite, content)
	}
	if err != nil {
		return err
	}
	return vhost.Save()
}

func (r *wafRepo) restoreWafConfig(vhost webservertypes.Vhost, content string, cause error) error {
	if err := r.writeWafConfig(vhost, content, true); err != nil {
		return fmt.Errorf("%w; restore WAF config failed: %v", cause, err)
	}
	if err := r.reloadWebServer(); err != nil {
		return fmt.Errorf("%w; restore WAF config failed: %v", cause, err)
	}
	return cause
}

func (r *wafRepo) reloadWebServer() error {
	webServer, err := r.setting.Get(biz.SettingKeyWebserver, "unknown")
	if err != nil {
		return err
	}
	if webServer != "nginx" {
		return errors.New(r.t.Get("unsupported web server: %s", webServer))
	}
	if out, testErr := shell.Execf("nginx -t"); testErr != nil {
		r.log.Warn("nginx config test failed", slog.String("cmd", "nginx -t"), slog.Any("err", testErr))
		return fmt.Errorf("nginx config test failed: %w; output: %s", testErr, out)
	}
	if err = systemctl.Reload("nginx"); err != nil {
		return fmt.Errorf("reload nginx failed: %w", err)
	}

	return nil
}
