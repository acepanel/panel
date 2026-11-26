package data

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"github.com/libtnb/utils/str"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("body must not be read") }

func newUserTokenRepoForTest(t *testing.T) userTokenRepo {
	t.Helper()
	app.Key = "0123456789abcdef0123456789abcdef"
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	must.NoError(t, db.AutoMigrate(&biz.UserToken{}))
	return userTokenRepo{t: gotext.NewLocale("", "en"), conf: &config.Config{}, db: db}
}

// signedRequest 按文档的签名算法构造一个 API 请求
func signedRequest(repo userTokenRepo, token *biz.UserToken, timestamp int64, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/website?page=1", strings.NewReader(body))
	canonical := fmt.Sprintf("%s\n%s\n%s\n%s", req.Method, req.URL.Path, req.URL.Query().Encode(), str.SHA256(body))
	signature := repo.hmacsha256(fmt.Sprintf("HMAC-SHA256\n%d\n%s", timestamp, str.SHA256(canonical)), token.Token)
	req.Header.Set("Authorization", fmt.Sprintf("HMAC-SHA256 Credential=%d, Signature=%s", token.ID, signature))
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
	return req
}

func TestUserTokenValidateReq(t *testing.T) {
	repo := newUserTokenRepoForTest(t)
	token, err := repo.Create(7, nil, time.Now().Add(time.Hour))
	must.NoError(t, err)
	now := time.Now().Unix()

	tests := []struct {
		name      string
		timestamp int64
		wantErr   bool
	}{
		{name: "当前时间", timestamp: now},
		{name: "窗口内的过去时间", timestamp: now - 200},
		{name: "过期的时间戳", timestamp: now - 400, wantErr: true},
		{name: "未来的时间戳", timestamp: now + 400, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := signedRequest(repo, token, tt.timestamp, `{"name":"test"}`)
			userID, err := repo.ValidateReq(req)
			check.Equal(t, err != nil, tt.wantErr)
			if !tt.wantErr {
				check.Equal(t, userID, uint(7))
				body, _ := io.ReadAll(req.Body)
				check.Equal(t, string(body), `{"name":"test"}`)
			}
		})
	}
}

func TestUserTokenValidateReqRejectsTamperedBody(t *testing.T) {
	repo := newUserTokenRepoForTest(t)
	token, err := repo.Create(7, nil, time.Now().Add(time.Hour))
	must.NoError(t, err)

	req := signedRequest(repo, token, time.Now().Unix(), `{"name":"test"}`)
	req.Body = io.NopCloser(strings.NewReader(`{"name":"evil"}`))
	_, err = repo.ValidateReq(req)
	check.Error(t, err)
}

// 中间件靠 *http.MaxBytesError 识别请求体超限，这里不能把它换成别的错误
func TestUserTokenValidateReqKeepsBodyLimitError(t *testing.T) {
	repo := newUserTokenRepoForTest(t)
	token, err := repo.Create(7, nil, time.Now().Add(time.Hour))
	must.NoError(t, err)

	req := signedRequest(repo, token, time.Now().Unix(), `{"name":"test"}`)
	req.Body = http.MaxBytesReader(nil, req.Body, 4)
	_, err = repo.ValidateReq(req)
	check.True(t, errors.As(err, new(*http.MaxBytesError)))
}

func TestUserTokenValidateReqChecksIPBeforeBody(t *testing.T) {
	repo := newUserTokenRepoForTest(t)
	token, err := repo.Create(7, []string{"203.0.113.7"}, time.Now().Add(time.Hour))
	must.NoError(t, err)

	req := signedRequest(repo, token, time.Now().Unix(), "")
	req.RemoteAddr = "198.51.100.9:54321"
	req.Body = io.NopCloser(failingBody{})
	_, err = repo.ValidateReq(req)
	must.Error(t, err)
	check.True(t, strings.Contains(err.Error(), "198.51.100.9"))
}
