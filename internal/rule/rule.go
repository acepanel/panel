package rule

import (
	"github.com/libtnb/validator"
	"gorm.io/gorm"
)

// Rules 面板自定义校验规则
func Rules() []validator.Rule {
	return []validator.Rule{
		NewPassword(),
		NewCron(),
		NewIPCIDR(),
		NewUnixPath(),
	}
}

// FallibleRules 依赖数据库查询的校验规则
func FallibleRules(db *gorm.DB) []validator.FallibleRule {
	return []validator.FallibleRule{
		NewExists(db),
		NewNotExists(db),
	}
}
