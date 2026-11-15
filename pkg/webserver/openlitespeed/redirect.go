package openlitespeed

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

// 主机名与 URL 重定向写为 rewrite 规则，404 重定向写为 errorpage

const (
	// redirectMeta 重定向分散在 errorpage 与 rewrite 块里，靠它记下用户定义的顺序
	redirectMeta = "redirect"
	// keepURISuffix URL 重定向保留 URI 时接在目标后面的原始路径
	keepURISuffix = "%{REQUEST_URI}"
)

// 重写规则的目标里 % 与 $ 是反向引用的前缀，目标地址里的字面量（如 %20）要转义
var (
	targetEscaper   = strings.NewReplacer(`%`, `\%`, `$`, `\$`)
	targetUnescaper = strings.NewReplacer(`\%`, `%`, `\$`, `$`)
)

var (
	hostCondPattern     = regexp.MustCompile(`^%\{HTTP_HOST\}\s+\^(.+)\$\s+\[NC\]$`)
	redirectRulePattern = regexp.MustCompile(`^\^(.+)\$\s+(\S+)\s+\[R=(\d+),L(?:,QSD)?]$`)
)

// loadRedirects 按写入时的序号还原顺序；旧文件没有序号，稳定排序下保持 URL、404、主机名的先后
func (v *baseVhost) loadRedirects(cfg *conf.Config) {
	type indexed struct {
		index    int
		redirect types.Redirect
	}
	var found []indexed
	add := func(meta string, r types.Redirect) {
		index, _ := strconv.Atoi(meta)
		found = append(found, indexed{index: index, redirect: r})
	}

	// 旧版的 URL 重定向是 redirect 上下文，保留 URI 时写成 ^/old(.*)$ 加 $1
	for _, ctx := range cfg.Blocks("context") {
		if !strings.EqualFold(ctx.Value("type"), "redirect") {
			continue
		}
		from, keepURI := strings.CutSuffix(strings.TrimPrefix(ctx.Arg(0), "exp:^"), "(.*)$")
		code, _ := strconv.Atoi(ctx.Value("statusCode"))
		add("", types.Redirect{
			Type:       types.RedirectTypeURL,
			From:       strings.TrimSuffix(from, "$"),
			To:         strings.TrimSuffix(ctx.Value("location"), "$1"),
			KeepURI:    keepURI,
			StatusCode: code,
		})
	}

	for _, page := range cfg.Blocks("errorpage") {
		if page.Arg(0) == "404" {
			add(page.Meta(redirectMeta), types.Redirect{
				Type:       types.RedirectType404,
				To:         page.Value("url"),
				StatusCode: types.DefaultRedirectStatus,
			})
		}
	}

	if rw := cfg.GetBlock("rewrite"); rw != nil {
		// 序号注释后面那条规则是重定向；主机名重定向前面还有一条 HTTP_HOST 条件，旧文件里的没有序号
		host, meta, marked := "", "", false
		for _, n := range rw.Nodes {
			if c, ok := n.(*conf.Comment); ok {
				if rest, ok := strings.CutPrefix(strings.TrimSpace(c.Text), "ace:"+redirectMeta+" "); ok {
					meta, marked = rest, true
				}
				continue
			}
			d, ok := n.(*conf.Directive)
			if !ok {
				continue
			}
			switch {
			case strings.EqualFold(d.Name, "RewriteCond"):
				host = ""
				if m := hostCondPattern.FindStringSubmatch(d.Arg(0)); m != nil {
					host = strings.ReplaceAll(m[1], `\.`, ".")
				}
			case strings.EqualFold(d.Name, "RewriteRule"):
				m := redirectRulePattern.FindStringSubmatch(d.Arg(0))
				switch {
				case m == nil:
				case host != "":
					code, _ := strconv.Atoi(m[3])
					to, keepURI := cutKeepURI(m[2], "$1")
					add(meta, types.Redirect{Type: types.RedirectTypeHost, From: host, To: to, KeepURI: keepURI, StatusCode: code})
				case marked:
					code, _ := strconv.Atoi(m[3])
					to, keepURI := cutKeepURI(m[2], keepURISuffix)
					add(meta, types.Redirect{Type: types.RedirectTypeURL, From: types.UnquoteMeta(m[1]), To: to, KeepURI: keepURI, StatusCode: code})
				}
				host, meta, marked = "", "", false
			}
		}
	}

	sort.SliceStable(found, func(a, b int) bool { return found[a].index < found[b].index })
	for _, item := range found {
		v.RedirectRules = append(v.RedirectRules, item.redirect)
	}
}

func (v *baseVhost) buildRedirects(cfg *conf.Config) {
	for i, r := range v.RedirectRules {
		if r.Type == types.RedirectType404 {
			page := cfg.AddBlock("errorpage", "404")
			page.AddMeta(redirectMeta, strconv.Itoa(i))
			page.Add("url", r.To)
		}
	}
}

// addRedirectRules 主机名重定向排在 URL 重定向前面，同 nginx 里 server 级的 if 先于 location
func (v *baseVhost) addRedirectRules(rw *conf.Directive) {
	for _, typ := range []types.RedirectType{types.RedirectTypeHost, types.RedirectTypeURL} {
		for i, r := range v.RedirectRules {
			if r.Type != typ {
				continue
			}
			pattern, uri := "^(.*)$", "$1"
			if typ == types.RedirectTypeURL {
				pattern, uri = "^"+regexp.QuoteMeta(r.From)+"$", keepURISuffix
			}
			// 与 nginx 一致：保留 URI 即接上原始路径，查询串由重写引擎自动带上；不保留时连查询串一起丢掉
			to, flags := targetEscaper.Replace(r.To), fmt.Sprintf("[R=%d,L,QSD]", r.Status())
			if r.KeepURI {
				to, flags = to+uri, fmt.Sprintf("[R=%d,L]", r.Status())
			}

			rw.AddMeta(redirectMeta, strconv.Itoa(i))
			if typ == types.RedirectTypeHost {
				rw.Add("RewriteCond", fmt.Sprintf("%%{HTTP_HOST} ^%s$ [NC]", strings.ReplaceAll(r.From, ".", `\.`)))
			}
			rw.Add("RewriteRule", pattern+" "+to+" "+flags)
		}
	}
}

// cutKeepURI 拆出目标与接在后面的 URI 后缀，后缀前面是转义符时它是目标自己的字面量
func cutKeepURI(target, suffix string) (string, bool) {
	to, keepURI := strings.CutSuffix(target, suffix)
	if keepURI && strings.HasSuffix(to, `\`) {
		to, keepURI = target, false
	}
	return targetUnescaper.Replace(to), keepURI
}
