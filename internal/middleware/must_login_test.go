package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sessions"
	sessionmiddleware "github.com/libtnb/sessions/middleware"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

type memoryStore struct {
	mu   sync.Mutex
	data map[string]string
}

func (s *memoryStore) Close() error { return nil }

func (s *memoryStore) Gc(int) error { return nil }

func (s *memoryStore) Destroy(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

func (s *memoryStore) Read(id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.data[id]
	return data, ok, nil
}

func (s *memoryStore) Touch(id string) (bool, error) {
	_, ok, _ := s.Read(id)
	return ok, nil
}

func (s *memoryStore) Write(id string, data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = data
	return nil
}

func newSessionsForTest(t *testing.T) *sessions.Manager {
	t.Helper()
	manager, err := sessions.NewManager(&sessions.ManagerOptions{
		Key:                  "0123456789abcdef0123456789abcdef",
		DisableDefaultDriver: true,
	})
	must.NoError(t, err)
	must.NoError(t, manager.Extend("default", &memoryStore{data: map[string]string{}}))
	return manager
}

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
			manager := newSessionsForTest(t)
			// 与真实实现一样，验签前读完请求体
			tokens := &mocks.UserTokenRepo{ValidateReqFunc: func(r *http.Request) (uint, error) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					return 0, err
				}
				return 1, nil
			}}

			var handler http.Handler = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			handler = mustLogin(gotext.NewLocale("", "en"), &config.Config{}, manager, nil, tokens, nil)(handler)
			handler = sessionmiddleware.StartSession(manager)(handler)

			req := httptest.NewRequest(http.MethodPost, "/api/file/upload", io.LimitReader(zeroReader{}, tt.size))
			req.Header.Set("Authorization", "HMAC-SHA256 Credential=1, Signature=x")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			check.Equal(t, rec.Code, tt.want)
		})
	}
}

func TestMustLoginSessionStamp(t *testing.T) {
	const loginPath, infoPath = "/api/user/login", "/api/user/info"
	loggedIn := &biz.User{ID: 1, Password: "hash"}

	tests := []struct {
		name string
		user *biz.User // 请求到来时库里的用户，nil 表示已删除
		want int
	}{
		{name: "用户未变更", user: &biz.User{ID: 1, Password: "hash"}, want: http.StatusOK},
		{name: "密码已变更", user: &biz.User{ID: 1, Password: "new hash"}, want: http.StatusUnauthorized},
		{name: "用户已删除", want: http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := newSessionsForTest(t)
			users := &mocks.UserRepo{GetFunc: func(uint) (*biz.User, error) {
				if tt.user == nil {
					return nil, errors.New("record not found")
				}
				return tt.user, nil
			}}

			var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == loginPath {
					sess, err := manager.GetSession(r)
					must.NoError(t, err)
					sess.Put("user_id", loggedIn.ID)
					sess.Put("user_stamp", loggedIn.SessionStamp())
				}
			})
			handler = mustLogin(gotext.NewLocale("", "en"), &config.Config{}, manager, users, nil, []string{loginPath})(handler)
			handler = sessionmiddleware.StartSession(manager)(handler)

			login := httptest.NewRecorder()
			handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, loginPath, nil))

			req := httptest.NewRequest(http.MethodGet, infoPath, nil)
			for _, cookie := range login.Result().Cookies() {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			check.Equal(t, rec.Code, tt.want)
		})
	}
}
