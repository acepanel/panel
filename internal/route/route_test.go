package route

import (
	"testing"

	"github.com/libtnb/validator"

	"github.com/acepanel/panel/v3/internal/rule"
)

// TestSpecJSON 全量端点文档生成防线：任何端点的 Document 定义或 validate tag
// 与 OpenAPI 生成器不兼容都会在此暴露，而不是等到 debug 模式启动时才炸。
func TestSpecJSON(t *testing.T) {
	v := validator.MustNew(
		validator.WithStrictRequired(),
		validator.WithRules(rule.Rules()...),
		validator.WithFallibleRules(rule.FallibleRules(nil)...),
	)

	spec, err := SpecJSON(NewEndpoints(&Services{}), "AcePanel", v)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec) == 0 {
		t.Fatal("生成的 OpenAPI 文档为空")
	}
}
