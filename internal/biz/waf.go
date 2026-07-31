package biz

import (
	"time"

	"github.com/acepanel/panel/v3/internal/request"
)

type WafPolicyApplyState string

const (
	WafPolicyApplyStateSaved   WafPolicyApplyState = "saved"
	WafPolicyApplyStatePending WafPolicyApplyState = "pending"
	WafPolicyApplyStateApplied WafPolicyApplyState = "applied"
	WafPolicyApplyStateFailed  WafPolicyApplyState = "failed"
)

// WafBinding 网站与 WAF 策略的绑定关系
type WafBinding struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	WebsiteID uint      `gorm:"not null;uniqueIndex;default:0" json:"website_id"` // 面板网站 ID 一个网站至多一条绑定
	PolicyID  uint64    `gorm:"not null;default:0" json:"policy_id"`              // agent 侧策略 ID
	Enabled   bool      `gorm:"not null;default:false" json:"enabled"`            // 是否已写入 nginx 并启用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	WebsiteName string `gorm:"-:all" json:"website_name"` // 仅显示
}

// WafRepo 单机
type WafRepo interface {
	// 透传 agent 策略
	ListPolicies() (any, error)
	GetPolicy(policyID string) (any, error)
	GetPolicyStatus(policyID string) (any, error)
	CreatePolicy(body any) (any, error)
	UpdatePolicy(policyID string, body any) (any, error)
	DeletePolicy(policyID string) error

	// 透传 agent 决策（黑白名单/拉黑）
	ListDecisions(query map[string]string) (any, error)
	CreateDecision(req *request.WafDecisionCreate) (any, error)
	DeleteDecision(query map[string]string) error

	// 透传 agent 误报加白
	ListExclusions(policyID string) (any, error)
	CreateExclusion(policyID string, body any) (any, error)
	DeleteExclusion(policyID string, query map[string]string) error

	// 透传 agent 报表
	Events(query map[string]string) (any, error)
	Stats(query map[string]string) (any, error)
	AttackMap(query map[string]string) (any, error)

	// 网站绑定 + nginx 启停
	ListBindings() ([]*WafBinding, error)
	EnableWebsite(req *request.WafWebsiteToggle) error
	DisableWebsite(websiteID uint) error
}

// WafUsecase WAF 用例：编排与外部交互均封装在 repo 原语，此处透传
type WafUsecase struct {
	repo WafRepo
}

func NewWafUsecase(repo WafRepo) *WafUsecase {
	return &WafUsecase{repo: repo}
}

func (uc *WafUsecase) ListPolicies() (any, error) { return uc.repo.ListPolicies() }

func (uc *WafUsecase) GetPolicy(policyID string) (any, error) { return uc.repo.GetPolicy(policyID) }

func (uc *WafUsecase) GetPolicyStatus(policyID string) (any, error) {
	return uc.repo.GetPolicyStatus(policyID)
}

func (uc *WafUsecase) CreatePolicy(body any) (any, error) { return uc.repo.CreatePolicy(body) }

func (uc *WafUsecase) UpdatePolicy(policyID string, body any) (any, error) {
	return uc.repo.UpdatePolicy(policyID, body)
}

func (uc *WafUsecase) DeletePolicy(policyID string) error { return uc.repo.DeletePolicy(policyID) }

func (uc *WafUsecase) ListDecisions(query map[string]string) (any, error) {
	return uc.repo.ListDecisions(query)
}

func (uc *WafUsecase) CreateDecision(req *request.WafDecisionCreate) (any, error) {
	return uc.repo.CreateDecision(req)
}

func (uc *WafUsecase) DeleteDecision(query map[string]string) error {
	return uc.repo.DeleteDecision(query)
}

func (uc *WafUsecase) ListExclusions(policyID string) (any, error) {
	return uc.repo.ListExclusions(policyID)
}

func (uc *WafUsecase) CreateExclusion(policyID string, body any) (any, error) {
	return uc.repo.CreateExclusion(policyID, body)
}

func (uc *WafUsecase) DeleteExclusion(policyID string, query map[string]string) error {
	return uc.repo.DeleteExclusion(policyID, query)
}

func (uc *WafUsecase) Events(query map[string]string) (any, error) { return uc.repo.Events(query) }

func (uc *WafUsecase) Stats(query map[string]string) (any, error) { return uc.repo.Stats(query) }

func (uc *WafUsecase) AttackMap(query map[string]string) (any, error) {
	return uc.repo.AttackMap(query)
}

func (uc *WafUsecase) ListBindings() ([]*WafBinding, error) { return uc.repo.ListBindings() }

func (uc *WafUsecase) EnableWebsite(req *request.WafWebsiteToggle) error {
	return uc.repo.EnableWebsite(req)
}

func (uc *WafUsecase) DisableWebsite(websiteID uint) error { return uc.repo.DisableWebsite(websiteID) }
