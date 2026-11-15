package service

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestBackupUploadWithoutFile(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	must.NoError(t, writer.WriteField("type", "website"))
	must.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("type", "website")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	rec := httptest.NewRecorder()
	(&BackupService{t: gotext.NewLocale("", "en")}).Upload(rec, req)
	check.Equal(t, rec.Code, http.StatusUnprocessableEntity)
}

func TestBackupSaveUpload(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "site.tar.gz")
	must.NoError(t, err)
	_, err = part.Write([]byte("archive"))
	must.NoError(t, err)
	must.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	must.NoError(t, req.ParseMultipartForm(1<<20))
	file := req.MultipartForm.File["file"][0]

	dir := t.TempDir()
	target := filepath.Join(dir, file.Filename)
	must.NoError(t, new(BackupService).saveUpload(file, target))

	data, err := os.ReadFile(target)
	must.NoError(t, err)
	check.Equal(t, string(data), "archive")
	info, err := os.Stat(target)
	must.NoError(t, err)
	check.Equal(t, info.Mode().Perm(), os.FileMode(0644))

	entries, err := os.ReadDir(dir)
	must.NoError(t, err)
	check.Len(t, entries, 1, check.Msgf("不应留下临时文件"))

	// 目标目录不存在时失败，且不留下任何文件
	missing := filepath.Join(dir, "missing", file.Filename)
	check.Error(t, new(BackupService).saveUpload(file, missing))
	entries, err = os.ReadDir(dir)
	must.NoError(t, err)
	check.Len(t, entries, 1)
}
