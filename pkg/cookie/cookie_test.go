package cookie

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestSet(t *testing.T) {
	tests := []struct {
		name       string
		tls        bool
		wantSecure bool
	}{
		{name: "明文连接", wantSecure: false},
		{name: "TLS 连接", tls: true, wantSecure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()
			Set(w, r, "session", "token", time.Hour)

			cookies := w.Result().Cookies()
			must.Equal(t, len(cookies), 1)
			check.Equal(t, cookies[0].Value, "token")
			check.Equal(t, cookies[0].Path, "/")
			check.Equal(t, cookies[0].MaxAge, 3600)
			check.True(t, cookies[0].HttpOnly)
			check.Equal(t, cookies[0].Secure, tt.wantSecure)
			check.Equal(t, cookies[0].SameSite, http.SameSiteLaxMode)
		})
	}
}

func TestGet(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	check.Equal(t, Get(r, "session"), "")

	r.Header.Set("Cookie", "session=token")
	check.Equal(t, Get(r, "session"), "token")
}

func TestClear(t *testing.T) {
	w := httptest.NewRecorder()
	Clear(w, httptest.NewRequest(http.MethodGet, "/", nil), "session")

	cookies := w.Result().Cookies()
	must.Equal(t, len(cookies), 1)
	check.Equal(t, cookies[0].Value, "")
	check.True(t, cookies[0].MaxAge < 0)
}
