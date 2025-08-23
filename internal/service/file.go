//go:build !windows

package service

import (
	"github.com/gofiber/fiber/v3"
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

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/os"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/tools"
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

func (s *FileService) Create(c fiber.Ctx) error {
	req, err := Bind[request.FileCreate](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if !req.Dir {
		if _, err = shell.Execf("touch %s", req.Path); err != nil {
			return Error(c, http.StatusInternalServerError, "%v", err)
		}
	} else {
		if err = stdos.MkdirAll(req.Path, 0755); err != nil {
			return Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	s.setPermission(req.Path, 0755, "www", "www")
	return Success(c, nil)
}

func (s *FileService) Content(c fiber.Ctx) error {
	req, err := Bind[request.FilePath](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	fileInfo, err := stdos.Stat(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	if fileInfo.IsDir() {
		return Error(c, http.StatusInternalServerError, s.t.Get("target is a directory"))
	}
	if fileInfo.Size() > 10*1024*1024 {
		return Error(c, http.StatusInternalServerError, s.t.Get("file is too large, please download it to view"))
	}

	content, err := stdos.ReadFile(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	mime, err := file.MimeType(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"mime":    mime,
		"content": base64.StdEncoding.EncodeToString(content),
	})
}

func (s *FileService) Save(c fiber.Ctx) error {
	req, err := Bind[request.FileSave](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	fileInfo, err := stdos.Stat(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = io.Write(req.Path, req.Content, fileInfo.Mode()); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FileService) Delete(c fiber.Ctx) error {
	req, err := Bind[request.FilePath](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	banned := []string{"/", app.Root, filepath.Join(app.Root, "server"), filepath.Join(app.Root, "panel")}
	if slices.Contains(banned, req.Path) {
		return Error(c, http.StatusForbidden, s.t.Get("please don't do this"))
	}

	if err = io.Remove(req.Path); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FileService) Upload(c fiber.Ctx) error {
	path := c.FormValue("path")
	file, err := c.FormFile("file")
	if err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("upload file error: %v", err))
	}
	if io.Exists(path) {
		return Error(c, http.StatusForbidden, s.t.Get("target path %s already exists", path))
	}

	if !io.Exists(filepath.Dir(path)) {
		if err = stdos.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return Error(c, http.StatusInternalServerError, s.t.Get("create directory error: %v", err))
		}
	}

	src, _ := file.Open()
	out, err := stdos.OpenFile(path, stdos.O_CREATE|stdos.O_RDWR|stdos.O_TRUNC, 0644)
	if err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("open file error: %v", err))
	}

	if _, err = stdio.Copy(out, src); err != nil {
		return Error(c, http.StatusInternalServerError, s.t.Get("write file error: %v", err))
	}

	_ = src.Close()
	s.setPermission(path, 0755, "www", "www")
	return Success(c, nil)
}

func (s *FileService) Exist(c fiber.Ctx) error {
	var paths []string
	if err := c.Bind().Body(&paths); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	var results []bool
	for item := range slices.Values(paths) {
		results = append(results, io.Exists(item))
	}

	return Success(c, results)
}

func (s *FileService) Move(c fiber.Ctx) error {

	var req []request.FileControl
	if err := c.Bind().Body(&req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	for item := range slices.Values(req) {
		if io.Exists(item.Target) && !item.Force {
			continue
		}

		if io.IsDir(item.Source) && strings.HasPrefix(item.Target, item.Source) {
			return Error(c, http.StatusForbidden, s.t.Get("please don't do this"))
		}

		if err := io.Mv(item.Source, item.Target); err != nil {
			return Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	return Success(c, nil)
}

func (s *FileService) Copy(c fiber.Ctx) error {

	var req []request.FileControl
	if err := c.Bind().Body(&req); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	for item := range slices.Values(req) {
		if io.Exists(item.Target) && !item.Force {
			continue
		}

		if io.IsDir(item.Source) && strings.HasPrefix(item.Target, item.Source) {
			return Error(c, http.StatusForbidden, s.t.Get("please don't do this"))
		}

		if err := io.Cp(item.Source, item.Target); err != nil {
			return Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	return Success(c, nil)
}

func (s *FileService) Download(c fiber.Ctx) error {
	req, err := Bind[request.FilePath](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	info, err := stdos.Stat(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	if info.IsDir() {
		return Error(c, http.StatusInternalServerError, s.t.Get("can't download a directory"))
	}

	return c.Download(req.Path, info.Name())
}

func (s *FileService) RemoteDownload(c fiber.Ctx) error {
	req, err := Bind[request.FileRemoteDownload](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	timestamp := time.Now().Format("20060102150405")
	task := new(biz.Task)
	task.Name = s.t.Get("Download remote file %v", filepath.Base(req.Path))
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`wget -o /tmp/remote-download-%s.log -O '%s' '%s' && chmod 0755 '%s' && chown www:www '%s'`, timestamp, req.Path, req.URL, req.Path, req.Path)
	task.Log = fmt.Sprintf("/tmp/remote-download-%s.log", timestamp)

	if err = s.taskRepo.Push(task); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FileService) Info(c fiber.Ctx) error {
	req, err := Bind[request.FilePath](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	info, err := stdos.Stat(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, chix.M{
		"name":     info.Name(),
		"size":     tools.FormatBytes(float64(info.Size())),
		"mode_str": info.Mode().String(),
		"mode":     fmt.Sprintf("%04o", info.Mode().Perm()),
		"dir":      info.IsDir(),
		"modify":   info.ModTime().Format(time.DateTime),
	})
}

func (s *FileService) Permission(c fiber.Ctx) error {
	req, err := Bind[request.FilePermission](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	// 解析成8进制
	mode, err := strconv.ParseUint(req.Mode, 8, 64)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = io.Chmod(req.Path, stdos.FileMode(mode)); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}
	if err = io.Chown(req.Path, req.Owner, req.Group); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	return Success(c, nil)
}

func (s *FileService) Compress(c fiber.Ctx) error {
	req, err := Bind[request.FileCompress](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = io.Compress(req.Dir, req.Paths, req.File); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	s.setPermission(req.File, 0755, "www", "www")
	return Success(c, nil)
}

func (s *FileService) UnCompress(c fiber.Ctx) error {
	req, err := Bind[request.FileUnCompress](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = io.UnCompress(req.File, req.Path); err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	list, err := io.ListCompress(req.File)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	for item := range slices.Values(list) {
		s.setPermission(filepath.Join(req.Path, item), 0755, "www", "www")
	}

	return Success(c, nil)
}

func (s *FileService) Search(c fiber.Ctx) error {
	req, err := Bind[request.FileSearch](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	results, err := io.SearchX(req.Path, req.Keyword, req.Sub)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	paged, total := Paginate(c, s.formatInfo(results))

	return Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *FileService) List(c fiber.Ctx) error {
	req, err := Bind[request.FileList](c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	list, err := stdos.ReadDir(req.Path)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
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

	paged, total := Paginate(c, s.formatDir(req.Path, list))

	return Success(c, chix.M{
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
			continue
		}

		stat := info.Sys().(*syscall.Stat_t)
		paths = append(paths, map[string]any{
			"name":     info.Name(),
			"full":     filepath.Join(base, info.Name()),
			"size":     tools.FormatBytes(float64(info.Size())),
			"mode_str": info.Mode().String(),
			"mode":     fmt.Sprintf("%04o", info.Mode().Perm()),
			"owner":    os.GetUser(stat.Uid),
			"group":    os.GetGroup(stat.Gid),
			"uid":      stat.Uid,
			"gid":      stat.Gid,
			"hidden":   io.IsHidden(info.Name()),
			"symlink":  io.IsSymlink(info.Mode()),
			"link":     io.GetSymlink(filepath.Join(base, info.Name())),
			"dir":      info.IsDir(),
			"modify":   info.ModTime().Format(time.DateTime),
		})
	}

	return paths
}

// formatInfo 格式化文件信息
func (s *FileService) formatInfo(infos map[string]stdos.FileInfo) []map[string]any {
	var paths []map[string]any
	for path, info := range infos {
		stat := info.Sys().(*syscall.Stat_t)
		paths = append(paths, map[string]any{
			"name":     info.Name(),
			"full":     path,
			"size":     tools.FormatBytes(float64(info.Size())),
			"mode_str": info.Mode().String(),
			"mode":     fmt.Sprintf("%04o", info.Mode().Perm()),
			"owner":    os.GetUser(stat.Uid),
			"group":    os.GetGroup(stat.Gid),
			"uid":      stat.Uid,
			"gid":      stat.Gid,
			"hidden":   io.IsHidden(info.Name()),
			"symlink":  io.IsSymlink(info.Mode()),
			"link":     io.GetSymlink(path),
			"dir":      info.IsDir(),
			"modify":   info.ModTime().Format(time.DateTime),
		})
	}

	slices.SortFunc(paths, func(a, b map[string]any) int {
		if cast.ToBool(a["dir"]) && !cast.ToBool(b["dir"]) {
			return -1
		}
		if !cast.ToBool(a["dir"]) && cast.ToBool(b["dir"]) {
			return 1
		}
		return strings.Compare(strings.ToLower(cast.ToString(a["name"])), strings.ToLower(cast.ToString(b["name"])))
	})

	return paths
}

// setPermission 设置权限
func (s *FileService) setPermission(path string, mode stdos.FileMode, owner, group string) {
	_ = io.Chmod(path, mode)
	_ = io.Chown(path, owner, group)
}
