package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/config"
)

func newEntranceConf(entrance string) *config.Config {
	conf := &config.Config{}
	conf.App.Key = "0123456789abcdef0123456789abcdef"
	conf.HTTP.Entrance = entrance
	conf.Session.Lifetime = 120
	return conf
}

func TestEntrancePass(t *testing.T) {
	conf := newEntranceConf("/secret")
	now := time.Now()
	valid := entrancePass(conf, "/secret", now.Add(time.Hour))
	_, signature, _ := strings.Cut(valid, ".")

	otherKey := newEntranceConf("/secret")
	otherKey.App.Key = "fedcba9876543210fedcba9876543210"

	tests := []struct {
		name     string
		conf     *config.Config
		entrance string
		pass     string
		want     bool
	}{
		{name: "有效凭证", conf: conf, entrance: "/secret", pass: valid, want: true},
		{name: "已过期", conf: conf, entrance: "/secret", pass: entrancePass(conf, "/secret", now.Add(-time.Second))},
		{name: "入口已修改", conf: conf, entrance: "/other", pass: valid},
		{name: "面板密钥不同", conf: otherKey, entrance: "/secret", pass: valid},
		{name: "篡改过期时间", conf: conf, entrance: "/secret", pass: strconv.FormatInt(now.Add(24*time.Hour).Unix(), 10) + "." + signature},
		{name: "没有签名", conf: conf, entrance: "/secret", pass: strconv.FormatInt(now.Add(time.Hour).Unix(), 10)},
		{name: "空凭证", conf: conf, entrance: "/secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Equal(t, !entranceExpires(tt.conf, tt.entrance, tt.pass).IsZero(), tt.want)
		})
	}
}

func TestEntrance(t *testing.T) {
	serve := func(conf *config.Config, path, pass string) *httptest.ResponseRecorder {
		handler := entrance(conf)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if pass != "" {
			req.Header.Set("Cookie", entranceCookie+"="+pass)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	conf := newEntranceConf("/secret")
	fresh := entrancePass(conf, "/secret", time.Now().Add(conf.Session.TTL()))

	t.Run("访问入口下发凭证并跳到登录页", func(t *testing.T) {
		rec := serve(conf, "/secret", "")
		check.Equal(t, rec.Header().Get("Location"), "/login")
		cookies := rec.Result().Cookies()
		must.Equal(t, len(cookies), 1)
		check.Equal(t, cookies[0].Name, entranceCookie)
		check.False(t, entranceExpires(conf, "/secret", cookies[0].Value).IsZero())
	})

	t.Run("没有凭证", func(t *testing.T) {
		for _, path := range []string{"/login", "/api/user/key", "/download/a/b"} {
			check.Equal(t, serve(conf, path, "").Code, http.StatusTeapot)
		}
	})

	t.Run("免入口的路径", func(t *testing.T) {
		for _, path := range []string{"/robots.txt", "/download/abc", "/webhook/abc"} {
			check.Equal(t, serve(conf, path, "").Code, http.StatusOK)
		}
	})

	t.Run("有效凭证放行且不重复下发", func(t *testing.T) {
		rec := serve(conf, "/login", fresh)
		check.Equal(t, rec.Code, http.StatusOK)
		check.Equal(t, len(rec.Result().Cookies()), 0)
	})

	t.Run("凭证过半续期", func(t *testing.T) {
		aging := entrancePass(conf, "/secret", time.Now().Add(conf.Session.TTL()/2-time.Minute))
		rec := serve(conf, "/login", aging)
		check.Equal(t, rec.Code, http.StatusOK)
		cookies := rec.Result().Cookies()
		must.Equal(t, len(cookies), 1)
		check.True(t, entranceExpires(conf, "/secret", cookies[0].Value).After(entranceExpires(conf, "/secret", aging)))
	})

	t.Run("伪造的凭证", func(t *testing.T) {
		forged := entrancePass(newEntranceConf("/other"), "/other", time.Now().Add(time.Hour))
		check.Equal(t, serve(conf, "/login", forged).Code, http.StatusTeapot)
	})

	t.Run("未设置入口时不需要凭证", func(t *testing.T) {
		rec := serve(newEntranceConf("/"), "/login", "")
		check.Equal(t, rec.Code, http.StatusOK)
		check.Equal(t, len(rec.Result().Cookies()), 0)
	})

	t.Run("调试模式不校验入口", func(t *testing.T) {
		debug := newEntranceConf("/secret")
		debug.App.Debug = true
		check.Equal(t, serve(debug, "/login", "").Code, http.StatusOK)
	})
}

func TestIsPublicPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "webhook 回调", path: "/webhook/abc", want: true},
		{name: "分享下载", path: "/download/abc", want: true},
		{name: "缺少标识", path: "/webhook/"},
		{name: "多出一段路径", path: "/download/abc/def"},
		{name: "前缀混淆", path: "/downloads/abc"},
		{name: "其他路径", path: "/login"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Equal(t, isPublicPath(tt.path), tt.want)
		})
	}
}

func TestTrimEntranceAPIPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		entrance string
		wantPath string
		wantOK   bool
	}{
		{name: "root api", path: "/api/user/info", entrance: "/", wantPath: "/api/user/info", wantOK: true},
		{name: "root api exact", path: "/api", entrance: "/", wantPath: "/api", wantOK: true},
		{name: "root api prefix confusion", path: "/apiary", entrance: "/", wantOK: false},
		{name: "entrance api", path: "/secret/api/user/info", entrance: "/secret", wantPath: "/api/user/info", wantOK: true},
		{name: "entrance api exact", path: "/secret/api", entrance: "/secret", wantPath: "/api", wantOK: true},
		{name: "entrance page", path: "/secret/login", entrance: "/secret", wantOK: false},
		{name: "entrance prefix confusion", path: "/secretx/api/user/info", entrance: "/secret", wantOK: false},
		{name: "api prefix confusion", path: "/secret/apiary", entrance: "/secret", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, gotOK := trimEntranceAPIPath(tt.path, tt.entrance)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotPath != tt.wantPath {
				t.Fatalf("path = %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}
