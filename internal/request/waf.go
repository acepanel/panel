package request

// WafWebsiteToggle 网站启用 WAF
type WafWebsiteToggle struct {
	WebsiteID uint   `form:"website_id" json:"website_id" validate:"required && exists:websites,id"`
	PolicyID  uint64 `form:"policy_id" json:"policy_id" validate:"required"`
}

// WafDecisionCreate 手工创建动态决策，Scope 由 Value 推导。
type WafDecisionCreate struct {
	Type  string `json:"type" validate:"required && in:ban,allow,captcha"`
	Scope string `json:"scope"`
	Value string `json:"value" validate:"required"`
	Until int64  `json:"until" validate:"min:0"`
}
