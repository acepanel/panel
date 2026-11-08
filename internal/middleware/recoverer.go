package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/leonelquinteros/gotext"
)

// Recoverer 捕获 panic，记录日志并返回 JSON 错误响应
func Recoverer(t *gotext.Locale) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if err, ok := rvr.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(rvr)
					}

					slog.Error(t.Get("panic recovered"), slog.Any("panic", rvr), slog.String("stack", string(debug.Stack())))

					// WebSocket 等升级连接无法写入 HTTP 响应
					if r.Header.Get("Connection") == "Upgrade" {
						return
					}

					Abort(w, http.StatusInternalServerError, fmt.Sprintf("%v", rvr))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
