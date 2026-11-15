package rule

import (
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"gorm.io/gorm"
)

// Rules 面板自定义校验规则
func Rules(t *gotext.Locale) []validator.Rule {
	return []validator.Rule{
		newPassword(t),
		newCron(t),
		newIPCIDR(t),
		newUnixPath(t),
		newEntrance(t),
	}
}

// FallibleRules 依赖数据库查询的校验规则
func FallibleRules(db *gorm.DB, t *gotext.Locale) []validator.FallibleRule {
	return []validator.FallibleRule{
		newExists(db, t),
		newNotExists(db, t),
	}
}

// Messages 覆盖内置规则的消息模板
func Messages(t *gotext.Locale) map[string]string {
	return map[string]string{
		"in":     t.Get("{field} must be one of {0+}"),
		"in_ci":  t.Get("{field} must be one of {0+}"),
		"not_in": t.Get("{field} cannot be {0+}"),
	}
}
