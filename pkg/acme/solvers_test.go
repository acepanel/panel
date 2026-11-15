package acme

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/mholt/acmez/v3/acme"
)

// 靠 -race 发现问题：内置服务器的请求协程与后续 Present 并发访问 tokens
func TestPanelSolverConcurrentTokens(t *testing.T) {
	const prefix = "/.well-known/acme-challenge/"
	// 置为内置服务器已启动的状态，Present 只追加 token，不抢全局锁也不监听 80 端口
	s := &panelSolver{tokens: make(map[string]string), server: new(http.Server), presentCount: 1}

	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range 1000 {
			_ = s.Present(context.Background(), acme.Challenge{Token: strconv.Itoa(i), KeyAuthorization: "auth"})
		}
	})
	wg.Go(func() {
		for range 1000 {
			s.serveToken(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, prefix+"0", nil))
		}
	})
	wg.Wait()

	w := httptest.NewRecorder()
	s.serveToken(w, httptest.NewRequest(http.MethodGet, prefix+"999", nil))
	check.Equal(t, w.Code, http.StatusOK)
	check.Equal(t, w.Body.String(), "auth")

	w = httptest.NewRecorder()
	s.serveToken(w, httptest.NewRequest(http.MethodGet, prefix+"missing", nil))
	check.Equal(t, w.Code, http.StatusNotFound)
}
