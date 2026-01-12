package service

import (
	stdio "io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/io"
)

type BackupService struct {
	t          *gotext.Locale
	backupRepo biz.BackupRepo
}

func NewBackupService(t *gotext.Locale, backup biz.BackupRepo) *BackupService {
	return &BackupService{
		t:          t,
		backupRepo: backup,
	}
}

func (s *BackupService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	list, _ := s.backupRepo.List(biz.BackupType(req.Type))
	paged, total := Paginate(r, list)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *BackupService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupCreate](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.backupRepo.Create(r.Context(), biz.BackupType(req.Type), req.Target, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *BackupService) Upload(w http.ResponseWriter, r *http.Request) {
	binder := chix.NewBind(r)
	defer binder.Release()

	req := new(request.BackupUpload)
	if err := binder.MultipartForm(req, 2<<30); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if err := binder.URI(req); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 只允许上传 .sql .zip .tar .gz .tgz .bz2 .xz .7z
	if !slices.Contains([]string{".sql", ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z"}, filepath.Ext(req.File.Filename)) {
		Error(w, http.StatusForbidden, s.t.Get("unsupported file type"))
		return
	}

	path, err := s.backupRepo.GetPath(biz.BackupType(req.Type))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if io.Exists(filepath.Join(path, req.File.Filename)) {
		Error(w, http.StatusForbidden, s.t.Get("target backup %s already exists", path))
		return
	}

	src, _ := req.File.Open()
	out, err := os.OpenFile(filepath.Join(path, req.File.Filename), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if _, err = stdio.Copy(out, src); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	_ = src.Close()
	Success(w, nil)
}

func (s *BackupService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupFile](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.backupRepo.Delete(r.Context(), biz.BackupType(req.Type), req.File); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *BackupService) Restore(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupRestore](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.backupRepo.Restore(r.Context(), biz.BackupType(req.Type), req.File, req.Target); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
