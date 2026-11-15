package common

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/pkg/webserver"
)

func decode(t *testing.T, rec *httptest.ResponseRecorder) (msg string, data any) {
	t.Helper()

	var body struct {
		Msg  string `json:"msg"`
		Data any    `json:"data"`
	}
	must.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	return body.Msg, body.Data
}

func TestServeConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")

	// 文件不存在当作空内容，docker 的 daemon.json 默认就不存在
	rec := httptest.NewRecorder()
	ServeConfig(rec, path)
	check.Equal(t, rec.Code, http.StatusOK)
	_, data := decode(t, rec)
	check.Equal(t, data, "")

	must.NoError(t, os.WriteFile(path, []byte("{}\n"), 0600))
	rec = httptest.NewRecorder()
	ServeConfig(rec, path)
	_, data = decode(t, rec)
	check.Equal(t, data, "{}\n")

	// 其它读取错误要报出来，不能当成空配置让用户保存覆盖
	rec = httptest.NewRecorder()
	ServeConfig(rec, dir)
	check.Equal(t, rec.Code, http.StatusInternalServerError)
}

// saveConfig 经路由调用 SaveConfig，cancel 用来在生效途中掐断请求
func saveConfig(
	t *testing.T,
	path, body string,
	apply func(ctx context.Context, cancel context.CancelFunc) error,
) *httptest.ResponseRecorder {
	t.Helper()

	router := chi.NewRouter()
	router.Post("/config", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		SaveConfig(w, r.WithContext(ctx), slog.New(slog.DiscardHandler), gotext.NewLocale("", "en"), "test", path, 0600, func(ctx context.Context) error {
			return apply(ctx, cancel)
		})
	})

	req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func TestSaveConfigAppliesWithoutCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.conf")

	applied := false
	rec := saveConfig(t, path, `{"config":"port 6380\n"}`, func(ctx context.Context, cancel context.CancelFunc) error {
		applied = true
		// 配置已落盘，生效不能跟着请求一起被取消
		cancel()
		check.NoError(t, ctx.Err())
		content, err := os.ReadFile(path)
		check.NoError(t, err)
		check.Equal(t, string(content), "port 6380\n")
		return nil
	})

	check.Equal(t, rec.Code, http.StatusOK)
	check.True(t, applied)
}

func TestSaveConfigReportsApplyError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.conf")

	rec := saveConfig(t, path, `{"config":"port 6380\n"}`, func(context.Context, context.CancelFunc) error {
		return errors.New("restart failed")
	})

	check.Equal(t, rec.Code, http.StatusInternalServerError)
	msg, _ := decode(t, rec)
	check.Equal(t, msg, "restart failed")
}

func TestWebServerClearErrorLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "error.log")
	s := NewWebServer(gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler), "nginx", webserver.TypeNginx, filepath.Join(dir, "nginx.conf"), logPath)

	// 日志还没生成时没有可清的内容
	rec := httptest.NewRecorder()
	s.ClearErrorLog(rec, httptest.NewRequest(http.MethodPost, "/clear_error_log", nil))
	check.Equal(t, rec.Code, http.StatusOK)

	must.NoError(t, os.WriteFile(logPath, []byte("[error] something\n"), 0600))
	rec = httptest.NewRecorder()
	s.ClearErrorLog(rec, httptest.NewRequest(http.MethodPost, "/clear_error_log", nil))
	check.Equal(t, rec.Code, http.StatusOK)
	content, err := os.ReadFile(logPath)
	must.NoError(t, err)
	check.Empty(t, content)

	rec = httptest.NewRecorder()
	s.ErrorLog(rec, httptest.NewRequest(http.MethodGet, "/error_log", nil))
	_, data := decode(t, rec)
	check.Equal(t, data, any(logPath))
}
