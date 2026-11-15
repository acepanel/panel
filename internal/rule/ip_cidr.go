package rule

import (
	"net"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"github.com/spf13/cast"
)

// ipCIDR 验证一个值是否是有效的 IP 或 CIDR
type ipCIDR struct {
	t *gotext.Locale
}

func newIPCIDR(t *gotext.Locale) *ipCIDR {
	return &ipCIDR{t: t}
}

func (r *ipCIDR) Signature() string { return "ipcidr" }

func (r *ipCIDR) Message() string {
	return r.t.Get("{field} must be a valid IP address or CIDR notation")
}

func (r *ipCIDR) Passes(f *validator.Field) bool {
	if validator.IsEmptyValue(f.Reflect()) {
		return true
	}
	str := cast.ToString(f.Reflect().Interface())
	if net.ParseIP(str) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(str); err == nil {
		return true
	}
	return false
}
