package service

import (
	"github.com/gofiber/fiber/v3"
	stdio "io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/io"
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

func (s *BackupService) List(c fiber.Ctx) error {
	req, err := Bind[request.BackupList](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	list, _ := s.backupRepo.List(biz.BackupType(req.Type))
	paged, total := Paginate(c, list)

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *BackupService) Create(c fiber.Ctx) error {
	req, err := Bind[request.BackupCreate](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.backupRepo.Create(biz.BackupType(req.Type), req.Target, req.Path); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *BackupService) Upload(c fiber.Ctx) error {
	req, err := Bind[request.BackupUpload](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	// 只允许上传 .sql .zip .tar .gz .tgz .bz2 .xz .7z
	if !slices.Contains([]string{".sql", ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".7z"}, filepath.Ext(req.File.Filename)) {
		return Error(c, http.StatusForbidden, s.t.Get("unsupported file type"))
	}

	path, err := s.backupRepo.GetPath(biz.BackupType(req.Type))
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	if io.Exists(filepath.Join(path, req.File.Filename)) {
		return Error(c, http.StatusForbidden, s.t.Get("target backup %s already exists", path))
	}

	src, _ := req.File.Open()
	out, err := os.OpenFile(filepath.Join(path, req.File.Filename), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if _, err = stdio.Copy(out, src); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	_ = src.Close()
	return Success(c, nil)
}

func (s *BackupService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.BackupFile](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.backupRepo.Delete(biz.BackupType(req.Type), req.File); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *BackupService) Restore(c fiber.Ctx) error {
	req, err := Bind[request.BackupRestore](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = s.backupRepo.Restore(biz.BackupType(req.Type), req.File, req.Target); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}
