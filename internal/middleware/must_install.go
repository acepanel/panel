package middleware

import (
	"net/http"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

// mustInstall 确保已安装应用
func mustInstall(t *gotext.Locale, app biz.AppRepo) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var slugs []string
			switch {
			case strings.HasPrefix(r.URL.Path, "/api/website"):
				slugs = append(slugs, "nginx", "openresty", "apache", "openlitespeed", "caddy")
			case strings.HasPrefix(r.URL.Path, "/api/container"):
				slugs = append(slugs, "podman", "docker")
			case strings.HasPrefix(r.URL.Path, "/api/apps/"):
				if slug := strings.Split(r.URL.Path, "/")[3]; slug != "" {
					slugs = append(slugs, slug)
				}
			}

			if len(slugs) > 0 {
				if installed, _ := app.IsInstalled("slug IN ?", slugs); !installed {
					abort(w, http.StatusForbidden, t.Get("app %s not installed", slugs))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
