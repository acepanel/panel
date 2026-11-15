package middleware

import (
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
)

// status 检查程序状态
func status(t *gotext.Locale) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch app.Status.Load() {
			case app.StatusUpgrade:
				abort(w, http.StatusServiceUnavailable, t.Get("panel is upgrading, please refresh later"))
				return
			case app.StatusMaintain:
				abort(w, http.StatusServiceUnavailable, t.Get("panel is maintaining, please refresh later"))
				return
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
