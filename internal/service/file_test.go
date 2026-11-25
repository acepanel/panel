package service

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	stdos "os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/validator"

	"github.com/acepanel/panel/v3/internal/biz"
	mockbiz "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/internal/rule"
	pkgio "github.com/acepanel/panel/v3/pkg/io"
)

func TestFormatEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	must.NoError(t, stdos.WriteFile(path, []byte("12345"), 0640))
	info, err := stdos.Lstat(path)
	must.NoError(t, err)

	owners := newOwnerNames()
	entry := new(FileService).formatEntry(path, info, owners)
	check.Equal(t, entry["name"], ".env")
	check.Equal(t, entry["full"], any(path))
	check.Equal(t, entry["size"], "5.00 B")
	check.Equal(t, entry["mode"], "0640")
	check.Equal(t, entry["hidden"], true)
	check.Equal(t, entry["dir"], false)
	check.Equal(t, entry["symlink"], false)
	check.Len(t, owners.users, 1)
	check.Len(t, owners.groups, 1)
}

func TestFormatEntryDoesNotOpenFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	must.NoError(t, syscall.Mkfifo(fifo, 0644))
	link := filepath.Join(dir, "link")
	must.NoError(t, stdos.Symlink(fifo, link))

	// 没有写端的 FIFO 一旦被 open 就永久阻塞；List 用 Lstat，Info 用 Stat
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, path := range []string{fifo, link} {
			for _, stat := range []func(string) (stdos.FileInfo, error){stdos.Lstat, stdos.Stat} {
				info, err := stat(path)
				if err != nil {
					t.Error(err)
					return
				}
				check.Equal(t, new(FileService).formatEntry(path, info, newOwnerNames())["immutable"], false)
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("formatEntry blocked on a FIFO")
	}
}

func TestFileUnCompressPermission(t *testing.T) {
	locale := gotext.NewLocale("", "en")
	prev := defaultValidator
	SetValidator(validator.MustNew(validator.WithStrictRequired(), validator.WithRules(rule.Rules(locale)...)))
	t.Cleanup(func() { SetValidator(prev) })

	wantCmd, err := pkgio.UnCompressShell("/tmp/archive.zip", "/opt/ace/sites/demo", true)
	must.NoError(t, err)
	svc := &FileService{t: locale, log: slog.New(slog.DiscardHandler)}

	cases := []struct {
		name      string
		body      string
		wantShell string
	}{
		{
			name:      "set permission",
			body:      `{"file":"/tmp/archive.zip","path":"/opt/ace/sites/demo","set_permission":true}`,
			wantShell: wantCmd + " && " + svc.wwwShell("/opt/ace/sites/demo", true),
		},
		{
			name:      "skip permission",
			body:      `{"file":"/tmp/archive.zip","path":"/opt/ace/sites/demo","set_permission":false}`,
			wantShell: wantCmd,
		},
		{
			name:      "default skips permission",
			body:      `{"file":"/tmp/archive.zip","path":"/opt/ace/sites/demo"}`,
			wantShell: wantCmd,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var pushed *biz.Task
			svc.taskRepo = biz.NewTaskUsecase(locale, svc.log, &mockbiz.TaskRepo{
				PushFunc: func(task *biz.Task) error {
					pushed = task
					return nil
				},
			})

			req := httptest.NewRequest(http.MethodPost, "/api/file/un_compress", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext()))
			rec := httptest.NewRecorder()
			svc.UnCompress(rec, req)

			must.Equal(t, rec.Code, http.StatusOK)
			must.NotNil(t, pushed)
			check.Equal(t, pushed.Shell, tc.wantShell)
		})
	}
}
