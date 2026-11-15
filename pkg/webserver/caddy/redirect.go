package caddy

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 主机名与 URL 重定向各写一个匹配器加 redir，404 重定向写在 handle_errors 块里

// loadRedirects 按写入时的序号还原顺序，否则重新保存会打乱匹配器编号、产生无意义的 diff
func (v *baseVhost) loadRedirects(body *conf.Block) {
	indexed := make(map[int]types.Redirect)
	for _, d := range body.GetAll("redir") {
		index, ok := strings.CutPrefix(d.Arg(0), "@ace_redirect_")
		m := body.Get(d.Arg(0))
		if !ok || m == nil {
			continue
		}
		r := redirectFromArgs(d.Arg(1), d.Arg(2))
		r.From = m.Arg(1)
		r.Type = types.RedirectTypeURL
		if m.Arg(0) == "host" {
			r.Type = types.RedirectTypeHost
		}
		i, _ := strconv.Atoi(index)
		indexed[i] = r
	}
	for _, h := range body.GetAll("handle_errors") {
		if d := h.Get("redir"); h.Arg(0) == "404" && d != nil {
			r := redirectFromArgs(d.Arg(1), d.Arg(2))
			r.Type = types.RedirectType404
			i, _ := strconv.Atoi(h.Meta("redirect"))
			indexed[i] = r
		}
	}
	for _, i := range slices.Sorted(maps.Keys(indexed)) {
		v.RedirectRules = append(v.RedirectRules, indexed[i])
	}
}

func (v *baseVhost) buildRedirects(body *conf.Block) {
	for i, r := range v.RedirectRules {
		name := fmt.Sprintf("@ace_redirect_%d", i)
		to := r.To
		if r.KeepURI {
			to += "{uri}"
		}
		code := strconv.Itoa(r.Status())
		switch r.Type {
		case types.RedirectTypeHost:
			body.Add(name, "host", r.From)
			body.Add("redir", name, to, code)
		case types.RedirectTypeURL:
			body.Add(name, "path", r.From)
			body.Add("redir", name, to, code)
		case types.RedirectType404:
			// redir 的首参以 / 开头会被当成路径匹配器，必须显式带通配匹配器
			h := body.AddBlock("handle_errors", "404")
			h.AddMeta("redirect", strconv.Itoa(i))
			h.Add("redir", "*", to, code)
		}
	}
}

func redirectFromArgs(to, code string) types.Redirect {
	status, _ := strconv.Atoi(code)
	return types.Redirect{
		To:         strings.TrimSuffix(to, "{uri}"),
		KeepURI:    strings.HasSuffix(to, "{uri}"),
		StatusCode: status,
	}
}
