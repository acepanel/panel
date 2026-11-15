package openlitespeed

import (
	"os"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
)

// realIPConf OLS 只能在服务器级信任代理头，对所有站点生效
const realIPConf = panelConfDir + "/realip.conf"

// RealIP 从 X-Forwarded-For 取客户端 IP 的配置
type RealIP struct {
	Enabled bool     `json:"enabled"`
	Trusted []string `json:"trusted"` // 可信代理，留空则信任所有来源
}

// GetRealIP 文件不存在视为关闭
func GetRealIP() (RealIP, error) {
	var r RealIP
	cfg, err := ParseFile(realIPConf)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return r, err
	}

	mode := cfg.Value("useIpInProxyHeader")
	r.Enabled = mode != "" && mode != "0"
	if b := cfg.GetBlock("accessControl"); b != nil {
		for item := range strings.SplitSeq(b.Value("allow"), ",") {
			item = strings.TrimSpace(item)
			if item == "" || strings.EqualFold(item, "ALL") {
				continue
			}
			r.Trusted = append(r.Trusted, strings.TrimRight(item, "tT"))
		}
	}
	return r, nil
}

// SetRealIP 有可信列表时只信任带 T 后缀的来源，否则信任所有来源
func SetRealIP(r RealIP) error {
	cfg := &conf.Config{}
	switch {
	case !r.Enabled:
		cfg.Add("useIpInProxyHeader", "0")
	case len(r.Trusted) == 0:
		cfg.Add("useIpInProxyHeader", "1")
	default:
		cfg.Add("useIpInProxyHeader", "2")
		allow := []string{"ALL"}
		for _, ip := range r.Trusted {
			allow = append(allow, ip+"T")
		}
		cfg.AddBlock("accessControl", "").Add("allow", strings.Join(allow, ", "))
	}
	return os.WriteFile(realIPConf, []byte(Export(cfg)), 0600)
}
