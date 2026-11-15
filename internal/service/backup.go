package service

import (
	"fmt"
	stdio "io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/libtnb/utils/str"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/io"
)

type BackupService struct {
	t          *gotext.Locale
	log        *slog.Logger
	backupRepo *biz.BackupUsecase
	taskRepo   *biz.TaskUsecase
}

func NewBackupService(
	backupUsecase *biz.BackupUsecase,
	taskUsecase *biz.TaskUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) *BackupService {
	return &BackupService{
		t:          t,
		log:        log,
		backupRepo: backupUsecase,
		taskRepo:   taskUsecase,
	}
}

func (s *BackupService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.BackupList](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	list, err := s.backupRepo.ListGroup(biz.BackupType(req.Type))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	paged, total := Paginate(r, list)

	s.log.Info(s.t.Get("backup list viewed"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("backup_type", req.Type))

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
	var backupCmd string
	if req.Type == "website" {
		backupCmd = fmt.Sprintf("acepanel backup website -n '%s' -s '%d'", req.Target, req.Storage)
	} else {
		backupCmd = fmt.Sprintf("acepanel backup database -t '%s' -n '%s' -s '%d'", req.Type, req.Target, req.Storage)
	}

	script, cancelScript := s.taskScript("backup", backupCmd)
	task := &biz.Task{
		Key:         fmt.Sprintf("backup:%s:%s", req.Type, req.Target),
		Name:        s.t.Get("Backup %s: %s", req.Type, req.Target),
		Status:      biz.TaskStatusWaiting,
		Shell:       script,
		CancelShell: cancelScript,
	}
	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("backup task created"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("backup_type", req.Type), slog.String("target", req.Target), slog.Uint64("storage_id", uint64(req.Storage)), slog.Uint64("task_id", uint64(task.ID)))

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
	if req.File == nil {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("file is required"))
		return
	}

	// 只允许上传 .sql .zip .tar .gz .tgz .bz2 .xz .zst .7z
	if !slices.Contains([]string{".sql", ".zip", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".zst", ".7z"}, filepath.Ext(req.File.Filename)) {
		Error(w, http.StatusForbidden, s.t.Get("unsupported file type"))
		return
	}

	path := s.backupRepo.GetDefaultPath(biz.BackupType(req.Type))
	target := filepath.Join(path, req.File.Filename)
	if io.Exists(target) {
		Error(w, http.StatusForbidden, s.t.Get("target backup %s already exists", path))
		return
	}

	if err := s.saveUpload(req.File, target); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("backup uploaded"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("backup_type", req.Type), slog.String("name", req.File.Filename), slog.String("path", target), slog.Int64("size", req.File.Size))

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

	s.log.Info(s.t.Get("backup downloaded"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("backup_type", req.Type), slog.String("name", info.Name()), slog.String("path", file))

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
	var restoreCmd string
	if req.Type == "website" {
		restoreCmd = fmt.Sprintf("acepanel restore website -n '%s' -f '%s'", req.Target, req.File)
	} else {
		restoreCmd = fmt.Sprintf("acepanel restore database -t '%s' -n '%s' -f '%s'", req.Type, req.Target, req.File)
	}

	script, cancelScript := s.taskScript("restore", restoreCmd)
	task := &biz.Task{
		Key:         fmt.Sprintf("restore:%s:%s", req.Type, req.Target),
		Name:        s.t.Get("Restore %s: %s", req.Type, req.Target),
		Status:      biz.TaskStatusWaiting,
		Shell:       script,
		CancelShell: cancelScript,
	}
	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("restore task created"), slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("backup_type", req.Type), slog.String("target", req.Target), slog.String("name", filepath.Base(req.File)), slog.String("path", req.File), slog.Uint64("task_id", uint64(task.ID)))

	Success(w, nil)
}

// saveUpload 先写同目录临时文件再 rename，中途失败不留半截文件
func (s *BackupService) saveUpload(file *multipart.FileHeader, target string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*.part")
	if err != nil {
		return err
	}
	// 改名成功后临时文件已不存在，Remove 无害
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	if _, err = stdio.Copy(tmp, src); err != nil {
		return err
	}
	if err = tmp.Chmod(0644); err != nil {
		return err
	}
	// 写入错误可能要到 Close 才暴露（如磁盘写满），这里不能吞
	if err = tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), target)
}

// taskScript 给任务命令补上 PATH 与独享的 TMPDIR，返回任务脚本和取消时的清理脚本
// 进程被杀后无法自清临时目录，独享目录供取消时精确清理，不误伤并发的其他任务
// 放在面板目录下，避免大文件撑爆 /tmp（常为内存盘）
func (s *BackupService) taskScript(kind, command string) (string, string) {
	tmpDir := filepath.Join(app.Root, "tmp", "ace-"+kind+"-task-"+str.Random(16))
	script := fmt.Sprintf(`export PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:$PATH
export TMPDIR="%s"
mkdir -p "$TMPDIR"
%s
rc=$?
rm -rf "$TMPDIR"
exit $rc`, tmpDir, command)

	return script, fmt.Sprintf(`rm -rf "%s"`, tmpDir)
}
