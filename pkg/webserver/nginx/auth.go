package nginx

import (
	"regexp"
	"slices"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	"github.com/acepanel/panel/v3/pkg/webserver/types"
)

const authConfName = "011-auth.conf"

// 基本认证 map 变量名前缀
const (
	authRealmVarPrefix = "$ace_auth_realm_"
	authFileVarPrefix  = "$ace_auth_file_"
)

func parseAuthMaps(content string) []types.BasicAuth {
	cfg, err := Parse(content)
	if err != nil {
		return nil
	}
	var auths []types.BasicAuth
	for _, m := range cfg.Blocks("map") {
		if !strings.HasPrefix(m.Arg(1), authFileVarPrefix) {
			continue
		}
		for _, d := range m.All() {
			if d.Arg(0) == "" {
				continue
			}
			if d.Name == "default" {
				auths = append(auths, types.BasicAuth{Path: "/", UserFile: d.Arg(0)})
			} else {
				auths = append(auths, types.BasicAuth{Path: authPatternPath(d.Name), UserFile: d.Arg(0)})
			}
		}
	}
	return auths
}

// generateAuthMaps 用 realm/file 两个 map 按 $uri 匹配实现目录级认证，对所有 location 统一生效，值为 off 即不认证
func generateAuthMaps(siteName string, auths []types.BasicAuth) string {
	realmVar, fileVar := authVarNames(siteName)
	realmDefault, fileDefault := conf.Arg{Value: "off"}, quoted("")
	var dirs []types.BasicAuth
	for _, auth := range auths {
		if auth.Path == "/" {
			realmDefault, fileDefault = quoted("Restricted"), quoted(auth.UserFile)
			continue
		}
		dirs = append(dirs, auth)
	}
	// map 按声明顺序取首个命中的正则，更长（更精确）的路径需排在前面
	slices.SortStableFunc(dirs, func(a, b types.BasicAuth) int {
		return len(b.Path) - len(a.Path)
	})

	cfg := &conf.Config{}
	realm := cfg.AddBlock("map", "$uri", realmVar)
	realm.Append(&conf.Directive{Name: "default", Args: []conf.Arg{realmDefault}})
	file := cfg.AddBlock("map", "$uri", fileVar)
	file.Append(&conf.Directive{Name: "default", Args: []conf.Arg{fileDefault}})
	for _, dir := range dirs {
		pattern := quoted(authPathPattern(dir.Path))
		realm.Append(&conf.Directive{Name: pattern.Value, Args: []conf.Arg{quoted("Restricted")}})
		file.Append(&conf.Directive{Name: pattern.Value, Args: []conf.Arg{quoted(dir.UserFile)}})
	}
	return Export(cfg)
}

// authVarNames 返回站点专属的 realm/file map 变量名
func authVarNames(siteName string) (string, string) {
	suffix := types.SafeName(siteName)
	return authRealmVarPrefix + suffix, authFileVarPrefix + suffix
}

// authPathPattern 将路径转换为 map 正则，如 /admin -> ~^/admin(/.*)?$
func authPathPattern(path string) string {
	return "~^" + regexp.QuoteMeta(path) + "(/.*)?$"
}

func authPatternPath(pattern string) string {
	path := strings.TrimPrefix(pattern, "~^")
	path = strings.TrimSuffix(path, "(/.*)?$")
	return strings.ReplaceAll(path, `\`, "")
}

func quoted(value string) conf.Arg {
	return conf.Arg{Value: value, Quote: conf.QuoteDouble}
}
