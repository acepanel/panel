package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/middleware"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

// loginClient 模拟登录页：取公钥和随机数、加密、提交
type loginClient struct {
	t   *testing.T
	svc *UserService
}

func newLoginClient(t *testing.T) (*loginClient, *mocks.UserSessionRepo) {
	t.Helper()
	locale := gotext.NewLocale("", "en")
	log := slog.New(slog.DiscardHandler)

	users := &mocks.UserRepo{
		CheckPasswordFunc: func(username, password string) (*biz.User, error) {
			if username != "admin" || password != "secret-1" {
				return nil, errors.New("username or password error")
			}
			return &biz.User{ID: 7, Username: "admin"}, nil
		},
		GetFunc: func(id uint) (*biz.User, error) {
			return &biz.User{ID: id, Username: "admin"}, nil
		},
	}
	sessions := &mocks.UserSessionRepo{
		CreateFunc: func(uint, string, string, bool) (string, error) {
			return "token", nil
		},
		AuthenticateFunc: func(token, _ string) (*biz.UserSession, error) {
			if token != "token" {
				return nil, errors.New("session expired")
			}
			return &biz.UserSession{UserID: 7}, nil
		},
		DeleteFunc: func(string) error {
			return nil
		},
	}
	// 没有订阅任何事件，登录通知静默跳过
	settings := &mocks.SettingRepo{
		GetSliceFunc: func(biz.SettingKey, ...[]string) ([]string, error) { return nil, nil },
		GetFunc:      func(biz.SettingKey, ...string) (string, error) { return "", nil },
	}

	svc := NewUserService(
		biz.NewNotifyUsecase(locale, log, &mocks.NotifyChannelRepo{}, settings),
		biz.NewUserSessionUsecase(sessions),
		biz.NewUserUsecase(locale, log, users),
		&config.Config{},
		locale,
		log,
	)
	return &loginClient{t: t, svc: svc}, sessions
}

// credentials 取公钥和随机数，返回加密后的用户名与密码
func (c *loginClient) credentials(username, password string) (string, string) {
	c.t.Helper()
	rec := httptest.NewRecorder()
	c.svc.GetKey(rec, httptest.NewRequest(http.MethodGet, "/api/user/key", nil))
	must.Equal(c.t, rec.Code, http.StatusOK)

	var resp struct {
		Data struct {
			Key   string `json:"key"`
			Nonce string `json:"nonce"`
		} `json:"data"`
	}
	must.NoError(c.t, json.Unmarshal(rec.Body.Bytes(), &resp))
	block, _ := pem.Decode([]byte(resp.Data.Key))
	must.NotNil(c.t, block)
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	must.NoError(c.t, err)
	key, ok := parsed.(*rsa.PublicKey)
	must.True(c.t, ok)

	return c.encrypt(key, username), c.encrypt(key, resp.Data.Nonce+":"+password)
}

func (c *loginClient) encrypt(key *rsa.PublicKey, plain string) string {
	c.t.Helper()
	cipher, err := rsa.EncryptOAEP(sha512.New(), rand.Reader, key, []byte(plain), nil)
	must.NoError(c.t, err)
	return base64.StdEncoding.EncodeToString(cipher)
}

func (c *loginClient) login(username, password string) *httptest.ResponseRecorder {
	c.t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	must.NoError(c.t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
	rec := httptest.NewRecorder()
	c.svc.Login(rec, req)
	return rec
}

func TestUserLogin(t *testing.T) {
	client, sessions := newLoginClient(t)

	rec := client.login(client.credentials("admin", "secret-1"))
	must.Equal(t, rec.Code, http.StatusOK)
	cookies := rec.Result().Cookies()
	must.Equal(t, len(cookies), 1)
	check.Equal(t, cookies[0].Name, middleware.SessionCookie)
	check.Equal(t, cookies[0].Value, "token")

	rec = client.login(client.credentials("admin", "wrong"))
	check.Equal(t, rec.Code, http.StatusForbidden)
	check.Equal(t, len(rec.Result().Cookies()), 0)
	check.Equal(t, len(sessions.CreateCalls()), 1)
}

// 随机数只能用一次，截获的登录请求不能重放
func TestUserLoginRejectsReplay(t *testing.T) {
	client, sessions := newLoginClient(t)
	username, password := client.credentials("admin", "secret-1")

	must.Equal(t, client.login(username, password).Code, http.StatusOK)
	check.Equal(t, client.login(username, password).Code, http.StatusForbidden)
	check.Equal(t, len(sessions.CreateCalls()), 1)
}

// 密码密文里必须带服务端签发的随机数
func TestUserLoginRequiresNonce(t *testing.T) {
	client, sessions := newLoginClient(t)
	key, err := client.svc.key()
	must.NoError(t, err)

	username := client.encrypt(&key.PublicKey, "admin")
	for _, password := range []string{"secret-1", ":secret-1", "0123456789abcdef:secret-1"} {
		rec := client.login(username, client.encrypt(&key.PublicKey, password))
		check.Equal(t, rec.Code, http.StatusForbidden)
	}
	check.Equal(t, len(sessions.CreateCalls()), 0)
}

func TestUserLogout(t *testing.T) {
	client, sessions := newLoginClient(t)

	req := httptest.NewRequest(http.MethodPost, "/api/user/logout", nil)
	req.Header.Set("Cookie", middleware.SessionCookie+"=token")
	rec := httptest.NewRecorder()
	client.svc.Logout(rec, req)

	must.Equal(t, rec.Code, http.StatusOK)
	must.Equal(t, len(sessions.DeleteCalls()), 1)
	check.Equal(t, sessions.DeleteCalls()[0].Token, "token")
	cookies := rec.Result().Cookies()
	must.Equal(t, len(cookies), 1)
	check.True(t, cookies[0].MaxAge < 0)
}

func TestUserIsLogin(t *testing.T) {
	client, _ := newLoginClient(t)

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "有效会话", token: "token", want: "true"},
		{name: "失效会话", token: "stale", want: "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/user/is_login", nil)
			req.Header.Set("Cookie", middleware.SessionCookie+"="+tt.token)
			rec := httptest.NewRecorder()
			client.svc.IsLogin(rec, req)

			var resp struct {
				Data json.RawMessage `json:"data"`
			}
			must.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			check.Equal(t, string(resp.Data), tt.want)
		})
	}
}
