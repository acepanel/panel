package openlitespeed

import (
	"testing"

	"github.com/acepanel/panel/v3/pkg/webserver/internal/golden"
)

// goldenCustom 全功能站点里代理规则的自定义指令，本方言的原生语法
const goldenCustom = "addDefaultCharset        off\n# 注释保留"

func TestGolden(t *testing.T) {
	golden.Run(t, Dialect{}, goldenCustom)
}
