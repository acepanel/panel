package rule

import (
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"github.com/spf13/cast"
)

// cron 校验规则
type cron struct {
	t  *gotext.Locale
	re *regexp.Regexp
}

func newCron(t *gotext.Locale) *cron {
	// 每个字段是逗号分隔的若干项，每项是 *、数字或月份星期的英文缩写，可带范围与步长；整串 5 到 7 个字段
	const (
		atom  = `(?:\d+|(?i:JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|SUN|MON|TUE|WED|THU|FRI|SAT))`
		item  = `(?:\*|` + atom + `(?:-` + atom + `)?)(?:/\d+)?`
		field = item + `(?:,` + item + `)*`
	)

	return &cron{
		t: t,
		re: regexp.MustCompile(`^(?:@(?:annually|yearly|monthly|weekly|daily|hourly|reboot)|@every (?:\d+(?:ns|us|µs|ms|s|m|h))+|` +
			field + `(?: +` + field + `){4,6})$`),
	}
}

func (s *cron) Signature() string { return "cron" }

func (s *cron) Message() string { return s.t.Get("{field} must be a valid cron expression") }

func (s *cron) Passes(f *validator.Field) bool {
	if validator.IsEmptyValue(f.Reflect()) {
		return true
	}
	return s.re.MatchString(strings.TrimSpace(cast.ToString(f.Reflect().Interface())))
}
