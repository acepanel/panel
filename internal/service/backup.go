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
	"github.com/libtnb/utils/str"
	"github.com/samber/do/v2"

	"github.com/acepanel/panel/v3/internal/app"
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
	var backupCmd string
	if req.Type == "website" {
		backupCmd = fmt.Sprintf("acepanel backup website -n '%s' -s '%d'", req.Target, req.Storage)
	} else {
		backupCmd = fmt.Sprintf("acepanel backup database -t '%s' -n '%s' -s '%d'", req.Type, req.Target, req.Storage)
	}

	// 备份进程被杀后无法自清临时目录，包一层独享 TMPDIR 供取消时精确清理，不误伤并发的其他备份
	// 放在面板目录下，避免大文件撑爆 /tmp（常为内存盘）
	tmpDir := filepath.Join(app.Root, "tmp", "ace-backup-task-"+str.Random(16))
	cmd := fmt.Sprintf(`%sexport TMPDIR="%s"
mkdir -p "$TMPDIR"
%s
rc=$?
rm -rf "$TMPDIR"
exit $rc`, pathEnv, tmpDir, backupCmd)

	task := &biz.Task{
		Key:         fmt.Sprintf("backup:%s:%s", req.Type, req.Target),
		Name:        s.t.Get("Backup %s: %s", req.Type, req.Target),
		Status:      biz.TaskStatusWaiting,
		Shell:       cmd,
		CancelShell: fmt.Sprintf(`rm -rf "%s"`, tmpDir),
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

func (s *BackupService) Download(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupFile](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	file := filepath.Join(s.backupRepo.GetDefaultPath(biz.BackupType(req.Type)), req.File)
	info, err := os.Stat(file)
	if err != nil {
		Error(w, http.StatusNotFound, "%v", err)
		return
	}
	if info.IsDir() {
		Error(w, http.StatusForbidden, s.t.Get("can't download a directory"))
		return
	}

	render := chix.NewRender(w, r)
	defer render.Release()
	render.Download(file, info.Name())
}

func (s *BackupService) Restore(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupRestore](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 恢复可能耗时较长（大库/大站），提交到后台任务队列异步执行
	pathEnv := "export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH\n"
	var restoreCmd string
	if req.Type == "website" {
		restoreCmd = fmt.Sprintf("acepanel restore website -n '%s' -f '%s'", req.Target, req.File)
	} else {
		restoreCmd = fmt.Sprintf("acepanel restore database -t '%s' -n '%s' -f '%s'", req.Type, req.Target, req.File)
	}

	// 恢复进程被杀后无法自清临时目录，包一层独享 TMPDIR 供取消时精确清理，不误伤并发的其他任务
	// 放在面板目录下，避免大文件撑爆 /tmp（常为内存盘）
	tmpDir := filepath.Join(app.Root, "tmp", "ace-restore-task-"+str.Random(16))
	cmd := fmt.Sprintf(`%sexport TMPDIR="%s"
mkdir -p "$TMPDIR"
%s
rc=$?
rm -rf "$TMPDIR"
exit $rc`, pathEnv, tmpDir, restoreCmd)

	task := &biz.Task{
		Key:         fmt.Sprintf("restore:%s:%s", req.Type, req.Target),
		Name:        s.t.Get("Restore %s: %s", req.Type, req.Target),
		Status:      biz.TaskStatusWaiting,
		Shell:       cmd,
		CancelShell: fmt.Sprintf(`rm -rf "%s"`, tmpDir),
	}
	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
