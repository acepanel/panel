package service

import (
	"fmt"
	stdio "io"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/io"
)

type BackupService struct {
	t          *gotext.Locale
	backupRepo *biz.BackupUsecase
	taskRepo   *biz.TaskUsecase
}

func NewBackupService(i do.Injector) (*BackupService, error) {
	return &BackupService{
		t:          do.MustInvoke[*gotext.Locale](i),
		backupRepo: do.MustInvoke[*biz.BackupUsecase](i),
		taskRepo:   do.MustInvoke[*biz.TaskUsecase](i),
	}, nil
}

func (s *BackupService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	list, err := s.backupRepo.List(biz.BackupType(req.Type))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

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

	// 备份可能耗时较长（大库），提交到后台任务队列异步执行
	pathEnv := "export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH\n"
	var cmd string
	if req.Type == "website" {
		cmd = fmt.Sprintf("%sacepanel backup website -n '%s' -s '%d'", pathEnv, req.Target, req.Storage)
	} else {
		cmd = fmt.Sprintf("%sacepanel backup database -t '%s' -n '%s' -s '%d'", pathEnv, req.Type, req.Target, req.Storage)
	}

	task := &biz.Task{
		Name:   s.t.Get("Backup %s: %s", req.Type, req.Target),
		Status: biz.TaskStatusWaiting,
		Shell:  cmd,
	}
	if err = s.taskRepo.Push(task); err != nil {
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

	// 只允许上传 .sql .zip .tar .gz .tgz .bz2 .xz .zst .7z
	if !slices.Contains([]string{".sql", ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".zst", ".7z"}, filepath.Ext(req.File.Filename)) {
		Error(w, http.StatusForbidden, s.t.Get("unsupported file type"))
		return
	}

	path := s.backupRepo.GetDefaultPath(biz.BackupType(req.Type))
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
