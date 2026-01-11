//go:build !windows

package service

import (
	"encoding/base64"
	"fmt"
	stdio "io"
	"net/http"
	stdos "os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/utils/file"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/chattr"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/os"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/tools"
)

type FileService struct {
	t        *gotext.Locale
	taskRepo biz.TaskRepo
}

func NewFileService(t *gotext.Locale, task biz.TaskRepo) *FileService {
	return &FileService{
		t:        t,
		taskRepo: task,
	}
}

func (s *FileService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileCreate](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if !req.Dir {
		if _, err = shell.Execf("touch %s", req.Path); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	} else {
		if err = stdos.MkdirAll(req.Path, 0755); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	s.setPermission(req.Path, 0755, "www", "www")
	Success(w, nil)
}

func (s *FileService) Content(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	fileInfo, err := stdos.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if fileInfo.IsDir() {
		Error(w, http.StatusInternalServerError, s.t.Get("target is a directory"))
		return
	}
	if fileInfo.Size() > 10*1024*1024 {
		Error(w, http.StatusInternalServerError, s.t.Get("file is too large, please download it to view"))
		return
	}

	content, err := stdos.ReadFile(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	mime, err := file.MimeType(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"mime":    mime,
		"content": base64.StdEncoding.EncodeToString(content),
	})
}

func (s *FileService) Save(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileSave](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	fileInfo, err := stdos.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(req.Path, req.Content, fileInfo.Mode()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	banned := []string{"/", app.Root, filepath.Join(app.Root, "server"), filepath.Join(app.Root, "panel")}
	if slices.Contains(banned, req.Path) {
		Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
		return
	}

	if err = io.Remove(req.Path); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileService) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 30); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := r.FormValue("path")
	_, handler, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("upload file error: %v", err))
		return
	}
	if io.Exists(path) {
		Error(w, http.StatusForbidden, s.t.Get("target path %s already exists", path))
		return
	}

	if !io.Exists(filepath.Dir(path)) {
		if err = stdos.MkdirAll(filepath.Dir(path), 0755); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("create directory error: %v", err))
			return
		}
	}

	src, _ := handler.Open()
	out, err := stdos.OpenFile(path, stdos.O_CREATE|stdos.O_RDWR|stdos.O_TRUNC, 0644)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("open file error: %v", err))
		return
	}

	if _, err = stdio.Copy(out, src); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("write file error: %v", err))
		return
	}

	_ = src.Close()
	s.setPermission(path, 0755, "www", "www")
	Success(w, nil)
}

func (s *FileService) Exist(w http.ResponseWriter, r *http.Request) {
	binder := chix.NewBind(r)
	defer binder.Release()

	var paths []string
	if err := binder.Body(&paths); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var results []bool
	for item := range slices.Values(paths) {
		results = append(results, io.Exists(item))
	}

	Success(w, results)
}

func (s *FileService) Move(w http.ResponseWriter, r *http.Request) {
	binder := chix.NewBind(r)
	defer binder.Release()

	var req []request.FileControl
	if err := binder.Body(&req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	for item := range slices.Values(req) {
		if io.Exists(item.Target) && !item.Force {
			continue
		}

		if io.IsDir(item.Source) && strings.HasPrefix(item.Target, item.Source+"/") {
			Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
			return
		}

		if err := io.Mv(item.Source, item.Target); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	Success(w, nil)
}

func (s *FileService) Copy(w http.ResponseWriter, r *http.Request) {
	binder := chix.NewBind(r)
	defer binder.Release()

	var req []request.FileControl
	if err := binder.Body(&req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	for item := range slices.Values(req) {
		if io.Exists(item.Target) && !item.Force {
			continue
		}

		if io.IsDir(item.Source) && strings.HasPrefix(item.Target, item.Source+"/") {
			Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
			return
		}

		if err := io.Cp(item.Source, item.Target); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	Success(w, nil)
}

func (s *FileService) Download(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	info, err := stdos.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if info.IsDir() {
		Error(w, http.StatusInternalServerError, s.t.Get("can't download a directory"))
		return
	}

	render := chix.NewRender(w, r)
	defer render.Release()
	render.Download(req.Path, info.Name())
}

func (s *FileService) RemoteDownload(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileRemoteDownload](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	timestamp := time.Now().Format("20060102150405")
	task := new(biz.Task)
	task.Name = s.t.Get("Download remote file %v", filepath.Base(req.Path))
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`wget -o /tmp/remote-download-%s.log -O '%s' '%s' && chmod 0755 '%s' && chown www:www '%s'`, timestamp, req.Path, req.URL, req.Path, req.Path)
	task.Log = fmt.Sprintf("/tmp/remote-download-%s.log", timestamp)

	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileService) Info(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	info, err := stdos.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	stat := info.Sys().(*syscall.Stat_t)

	// 检查是否有 immutable 属性
	immutable := false
	if f, err := stdos.OpenFile(req.Path, stdos.O_RDONLY, 0); err == nil {
		immutable, _ = chattr.IsAttr(f, chattr.FS_IMMUTABLE_FL)
		_ = f.Close()
	}

	Success(w, chix.M{
		"name":      info.Name(),
		"full":      req.Path,
		"size":      tools.FormatBytes(float64(info.Size())),
		"mode_str":  info.Mode().String(),
		"mode":      fmt.Sprintf("%04o", info.Mode().Perm()),
		"owner":     os.GetUser(stat.Uid),
		"group":     os.GetGroup(stat.Gid),
		"uid":       stat.Uid,
		"gid":       stat.Gid,
		"hidden":    io.IsHidden(info.Name()),
		"symlink":   io.IsSymlink(info.Mode()),
		"link":      io.GetSymlink(req.Path),
		"dir":       info.IsDir(),
		"modify":    info.ModTime().Format(time.DateTime),
		"immutable": immutable,
	})
}

// Size 计算大小
func (s *FileService) Size(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	info, err := stdos.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !info.IsDir() {
		// 如果不是目录，直接返回文件大小
		Success(w, chix.M{
			"size": tools.FormatBytes(float64(info.Size())),
		})
		return
	}

	// 计算目录大小
	output, err := shell.Execf("du -sb '%s' | awk '{print $1}'", req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, tools.FormatBytes(cast.ToFloat64(output)))
}

func (s *FileService) Permission(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePermission](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 解析成8进制
	mode, err := strconv.ParseUint(req.Mode, 8, 64)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Chmod(req.Path, stdos.FileMode(mode)); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Chown(req.Path, req.Owner, req.Group); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileService) Compress(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileCompress](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Compress(req.Dir, req.Paths, req.File); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.setPermission(req.File, 0755, "www", "www")
	Success(w, nil)
}

func (s *FileService) UnCompress(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileUnCompress](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.UnCompress(req.File, req.Path); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	list, err := io.ListCompress(req.File)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	for item := range slices.Values(list) {
		s.setPermission(filepath.Join(req.Path, item), 0755, "www", "www")
	}

	Success(w, nil)
}

func (s *FileService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileList](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var list []stdos.DirEntry
	if req.Keyword != "" {
		list, err = io.SearchX(req.Path, req.Keyword, req.Sub)
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	} else {
		list, err = stdos.ReadDir(req.Path)
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	switch req.Sort {
	case "asc":
		slices.SortFunc(list, func(a, b stdos.DirEntry) int {
			return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
		})
	case "desc":
		slices.SortFunc(list, func(a, b stdos.DirEntry) int {
			return strings.Compare(strings.ToLower(b.Name()), strings.ToLower(a.Name()))
		})
	default:
		slices.SortFunc(list, func(a, b stdos.DirEntry) int {
			if a.IsDir() && !b.IsDir() {
				return -1
			}
			if !a.IsDir() && b.IsDir() {
				return 1
			}
			return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
		})
	}

	paged, total := Paginate(r, s.formatDir(req.Path, list))

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// formatDir 格式化目录信息
func (s *FileService) formatDir(base string, entries []stdos.DirEntry) []any {
	var paths []any
	for item := range slices.Values(entries) {
		info, err := item.Info()
		if err != nil {
			continue // 直接跳过，不返回错误，不然很烦人的
		}
		if de, ok := item.(*io.SearchEntry); ok {
			base = filepath.Dir(de.Path())
		}

		stat := info.Sys().(*syscall.Stat_t)
		// 对于目录，size 返回空字符串，需要用户手动计算
		size := ""
		if !info.IsDir() {
			size = tools.FormatBytes(float64(info.Size()))
		}

		// 检查是否有 immutable 属性
		fullPath := filepath.Join(base, info.Name())
		immutable := false
		if f, err := stdos.OpenFile(fullPath, stdos.O_RDONLY, 0); err == nil {
			immutable, _ = chattr.IsAttr(f, chattr.FS_IMMUTABLE_FL)
			_ = f.Close()
		}

		paths = append(paths, map[string]any{
			"name":      info.Name(),
			"full":      fullPath,
			"size":      size,
			"mode_str":  info.Mode().String(),
			"mode":      fmt.Sprintf("%04o", info.Mode().Perm()),
			"owner":     os.GetUser(stat.Uid),
			"group":     os.GetGroup(stat.Gid),
			"uid":       stat.Uid,
			"gid":       stat.Gid,
			"hidden":    io.IsHidden(info.Name()),
			"symlink":   io.IsSymlink(info.Mode()),
			"link":      io.GetSymlink(fullPath),
			"dir":       info.IsDir(),
			"modify":    info.ModTime().Format(time.DateTime),
			"immutable": immutable,
		})
	}

	return paths
}

// setPermission 设置权限
func (s *FileService) setPermission(path string, mode stdos.FileMode, owner, group string) {
	_ = io.Chmod(path, mode)
	_ = io.Chown(path, owner, group)
}
