package rule

import (
	"regexp"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"github.com/spf13/cast"
)

// unixPath 校验 Unix 绝对路径
type unixPath struct {
	t  *gotext.Locale
	re *regexp.Regexp
}

func newUnixPath(t *gotext.Locale) *unixPath {
	return &unixPath{
		t:  t,
		re: regexp.MustCompile(`^/$|^(/[^/\x00]+)+/?$`),
	}
}

func (r *unixPath) Signature() string { return "unix_path" }

func (r *unixPath) Message() string { return r.t.Get("{field} must be a valid Unix absolute path") }

func (r *unixPath) Passes(f *validator.Field) bool {
	if validator.IsEmptyValue(f.Reflect()) {
		return true
	}
	return r.re.MatchString(cast.ToString(f.Reflect().Interface()))
}
