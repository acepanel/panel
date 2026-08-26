package rule

import (
	"regexp"

	"github.com/libtnb/validator"
	"github.com/spf13/cast"
)

// Cron 校验规则
type Cron struct {
	re *regexp.Regexp
}

func NewCron() *Cron {
	return &Cron{
		re: regexp.MustCompile(`(@(annually|yearly|monthly|weekly|daily|hourly|reboot))|(@every (\d+(ns|us|µs|ms|s|m|h))+)|((((\d+,)+\d+|((\*|\d+)(\/|-)\d+)|\d+|\*) ?){5,7})`),
	}
}

func (s *Cron) Signature() string { return "cron" }

func (s *Cron) Message() string { return "{field} must be a valid cron expression" }

func (s *Cron) Passes(f *validator.Field) bool {
	if validator.IsEmptyValue(f.Reflect()) {
		return true
	}
	return s.re.MatchString(cast.ToString(f.Reflect().Interface()))
}
