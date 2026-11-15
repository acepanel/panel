package nginx

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

var redirectFilePattern = regexp.MustCompile(`^(\d{3})-redirect\.conf$`)

func parseRedirectFiles(siteDir string) []types.Redirect {
	var redirects []types.Redirect
	for _, file := range conf.ListFiles(siteDir, redirectFilePattern, redirectStartNum, redirectEndNum) {
		cfg, err := ParseFile(file)
		if err != nil {
			continue
		}
		if r := parseRedirect(cfg); r != nil {
			redirects = append(redirects, *r)
		}
	}
	return redirects
}

// parseRedirect 三种重定向各只含一条 return：主机名在 if 块里，404 由 error_page 引到命名 location，URL 在精确匹配 location 里
func parseRedirect(cfg *conf.Config) *types.Redirect {
	var ret *conf.Directive
	cfg.Walk(func(d *conf.Directive) {
		if ret == nil && d.Name == "return" && len(d.Args) >= 2 {
			ret = d
		}
	})
	if ret == nil {
		return nil
	}
	r := &types.Redirect{To: strings.Join(ret.ArgsFrom(1), " ")}
	r.StatusCode, _ = strconv.Atoi(ret.Arg(0))
	if r.KeepURI = strings.HasSuffix(r.To, "$request_uri"); r.KeepURI {
		r.To = strings.TrimSuffix(r.To, "$request_uri")
	}

	// if ($host = old.example.com)，主机名可能带引号或与 ) 连写
	for _, d := range cfg.GetAll("if") {
		if args := d.Values(); len(args) >= 3 && strings.Contains(args[0], "$host") {
			r.Type = types.RedirectTypeHost
			r.From = strings.TrimSuffix(args[2], ")")
			return r
		}
	}
	for _, d := range cfg.GetAll("error_page") {
		if d.Arg(0) == "404" {
			r.Type = types.RedirectType404
			return r
		}
	}
	for _, d := range cfg.GetAll("location") {
		if d.Arg(0) == "=" && d.Arg(1) != "" {
			r.Type = types.RedirectTypeURL
			r.From = d.Arg(1)
			return r
		}
	}
	return nil
}

func writeRedirectFiles(siteDir string, redirects []types.Redirect) error {
	if len(redirects) > redirectEndNum-redirectStartNum+1 {
		return fmt.Errorf("redirect rules exceed limit (%d)", redirectEndNum-redirectStartNum+1)
	}
	if err := conf.ClearFiles(siteDir, redirectFilePattern, redirectStartNum, redirectEndNum); err != nil {
		return err
	}
	for i, redirect := range redirects {
		if err := writeFragment(filepath.Join(siteDir, fmt.Sprintf("%03d-redirect.conf", redirectStartNum+i)), redirectNodes(redirect)...); err != nil {
			return err
		}
	}
	return nil
}

func redirectNodes(r types.Redirect) []conf.Node {
	status := strconv.Itoa(r.Status())
	to := r.To
	if r.KeepURI {
		to += "$request_uri"
	}
	ret := conf.Dir("return", status, to)

	switch r.Type {
	case types.RedirectTypeURL:
		return []conf.Node{
			conf.Cmt(fmt.Sprintf("URL redirect: %s -> %s", r.From, r.To)),
			conf.Blk("location", "=", r.From).Append(ret),
		}
	case types.RedirectTypeHost:
		return []conf.Node{
			conf.Cmt(fmt.Sprintf("Host redirect: %s -> %s", r.From, r.To)),
			conf.Blk("if", "($host", "=", r.From+")").Append(ret),
		}
	case types.RedirectType404:
		return []conf.Node{
			conf.Cmt("404 redirect -> " + r.To),
			conf.Dir("error_page", "404", "=", "@redirect_404"),
			conf.Blk("location", "@redirect_404").Append(ret),
		}
	}
	return nil
}
