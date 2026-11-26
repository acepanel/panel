package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestMustLoginLimitsSignedBody(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want int
	}{
		{name: "刚好达到上限", size: maxSignedBody, want: http.StatusOK},
		{name: "超出上限", size: maxSignedBody + 1, want: http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 与真实实现一样，验签前读完请求体
			tokens := &mocks.UserTokenRepo{ValidateReqFunc: func(r *http.Request) (uint, error) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					return 0, err
				}
				return 1, nil
			}}

			var handler http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			handler = mustLogin(gotext.NewLocale("", "en"), &config.Config{}, nil, tokens, nil)(handler)

			req := httptest.NewRequest(http.MethodPost, "/api/file/upload", io.LimitReader(zeroReader{}, tt.size))
			req.Header.Set("Authorization", "HMAC-SHA256 Credential=1, Signature=x")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			check.Equal(t, rec.Code, tt.want)
		})
	}
}

func TestMustLoginSession(t *testing.T) {
	const loginPath, infoPath = "/api/user/login", "/api/user/info"

	tests := []struct {
		name       string
		path       string
		token      string
		wantStatus int
		wantUserID uint64
	}{
		{name: "有效会话", path: infoPath, token: "valid", wantStatus: http.StatusOK, wantUserID: 7},
		{name: "失效会话", path: infoPath, token: "stale", wantStatus: http.StatusUnauthorized},
		{name: "没有会话", path: infoPath, wantStatus: http.StatusUnauthorized},
		{name: "白名单路径不需要会话", path: loginPath, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := &mocks.UserSessionRepo{AuthenticateFunc: func(token, ip string) (*biz.UserSession, error) {
				check.Equal(t, ip, "203.0.113.7")
				if token != "valid" {
					return nil, errors.New("session expired")
				}
				return &biz.UserSession{UserID: 7}, nil
			}}

			var userID uint64
			var handler http.Handler = http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				userID = biz.OperatorID(r.Context())
			})
			handler = mustLogin(gotext.NewLocale("", "en"), &config.Config{}, sessions, nil, []string{loginPath})(handler)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.RemoteAddr = "203.0.113.7:54321"
			if tt.token != "" {
				req.Header.Set("Cookie", SessionCookie+"="+tt.token)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			check.Equal(t, rec.Code, tt.wantStatus)
			check.Equal(t, userID, tt.wantUserID)
		})
	}
}

// 活跃的会话每次请求都顺延 Cookie 的有效期
func TestMustLoginRefreshesSessionCookie(t *testing.T) {
	sessions := &mocks.UserSessionRepo{AuthenticateFunc: func(string, string) (*biz.UserSession, error) {
		return &biz.UserSession{UserID: 7}, nil
	}}
	conf := &config.Config{}
	conf.Session.Lifetime = 30

	var handler http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	handler = mustLogin(gotext.NewLocale("", "en"), conf, sessions, nil, nil)(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
	req.Header.Set("Cookie", SessionCookie+"=valid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	must.Equal(t, len(cookies), 1)
	check.Equal(t, cookies[0].Name, SessionCookie)
	check.Equal(t, cookies[0].Value, "valid")
	check.Equal(t, cookies[0].MaxAge, 30*60)
}
