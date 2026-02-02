//go:build !windows

package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	stdio "io"
	"mime/multipart"
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
	force := r.FormValue("force") == "true"
	_, handler, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("upload file error: %v", err))
		return
	}
	if io.Exists(path) && !force {
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

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get file system info"))
		return
	}

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

	// 前缀 - 表示降序
	sortKey := req.Sort
	sortDesc := false
	if strings.HasPrefix(sortKey, "-") {
		sortDesc = true
		sortKey = strings.TrimPrefix(sortKey, "-")
	}

	// 获取文件信息用于排序
	type entryWithInfo struct {
		entry stdos.DirEntry
		info  stdos.FileInfo
	}
	entriesWithInfo := make([]entryWithInfo, 0, len(list))
	for _, entry := range list {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		entriesWithInfo = append(entriesWithInfo, entryWithInfo{entry: entry, info: info})
	}

	// 排序
	slices.SortFunc(entriesWithInfo, func(a, b entryWithInfo) int {
		// 文件夹始终排在前面（除非按特定字段排序）
		if sortKey == "" {
			if a.info.IsDir() && !b.info.IsDir() {
				return -1
			}
			if !a.info.IsDir() && b.info.IsDir() {
				return 1
			}
		}

		var cmp int
		switch sortKey {
		case "size":
			// 按大小排序
			if a.info.Size() < b.info.Size() {
				cmp = -1
			} else if a.info.Size() > b.info.Size() {
				cmp = 1
			} else {
				cmp = 0
			}
		case "modify":
			// 按修改时间排序
			if a.info.ModTime().Before(b.info.ModTime()) {
				cmp = -1
			} else if a.info.ModTime().After(b.info.ModTime()) {
				cmp = 1
			} else {
				cmp = 0
			}
		case "name":
			// 按名称排序
			cmp = strings.Compare(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name()))
		default:
			// 默认按名称排序
			cmp = strings.Compare(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name()))
		}

		if sortDesc {
			cmp = -cmp
		}
		return cmp
	})

	// 转换回 DirEntry 列表
	sortedList := make([]stdos.DirEntry, len(entriesWithInfo))
	for i, e := range entriesWithInfo {
		sortedList[i] = e.entry
	}

	paged, total := Paginate(r, s.formatDir(req.Path, sortedList))

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// ChunkUploadStart 开始分块上传
func (s *FileService) ChunkUploadStart(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ChunkUploadStart](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	targetPath := filepath.Join(req.Path, req.FileName)
	if io.Exists(targetPath) && !req.Force {
		Error(w, http.StatusForbidden, s.t.Get("target path %s already exists", targetPath))
		return
	}

	// 确保目标目录存在
	if !io.Exists(req.Path) {
		if err = stdos.MkdirAll(req.Path, 0755); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("create directory error: %v", err))
			return
		}
	}

	// 扫描目录中已存在的分块文件
	prefix := s.getChunkTempFilePrefix(req.FileName, req.FileHash)
	entries, err := stdos.ReadDir(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("read directory error: %v", err))
		return
	}

	uploadedChunks := make([]int, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			// 提取分块索引
			indexStr := strings.TrimPrefix(name, prefix)
			if index, err := strconv.Atoi(indexStr); err == nil && index >= 0 && index < req.ChunkCount {
				uploadedChunks = append(uploadedChunks, index)
			}
		}
	}

	Success(w, chix.M{
		"uploaded_chunks": uploadedChunks,
	})
}

// ChunkUploadChunk 上传单个分块
func (s *FileService) ChunkUploadChunk(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := r.FormValue("path")
	fileName := r.FormValue("file_name")
	fileHash := r.FormValue("file_hash")
	chunkIndex, _ := strconv.Atoi(r.FormValue("chunk_index"))
	chunkHash := r.FormValue("chunk_hash")

	if path == "" || fileName == "" || fileHash == "" {
		Error(w, http.StatusBadRequest, s.t.Get("path, file_name and file_hash are required"))
		return
	}

	// 获取上传的文件
	_, handler, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("get upload file error: %v", err))
		return
	}

	src, err := handler.Open()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("open upload file error: %v", err))
		return
	}
	defer func(src multipart.File) { _ = src.Close() }(src)

	// 读取分块内容
	chunkData, err := stdio.ReadAll(src)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("read chunk data error: %v", err))
		return
	}

	// 校验分块 hash
	if chunkHash != "" {
		hash := sha256.Sum256(chunkData)
		actualHash := hex.EncodeToString(hash[:])
		if actualHash != chunkHash {
			Error(w, http.StatusBadRequest, s.t.Get("chunk hash mismatch"))
			return
		}
	}

	// 保存分块到目标目录
	// 格式: .{filename}.{hash前16位}.chunk.{index}
	prefix := s.getChunkTempFilePrefix(fileName, fileHash)
	chunkPath := filepath.Join(path, fmt.Sprintf("%s%d", prefix, chunkIndex))
	if err = stdos.WriteFile(chunkPath, chunkData, 0644); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("save chunk error: %v", err))
		return
	}

	Success(w, chix.M{
		"chunk_index": chunkIndex,
	})
}

// ChunkUploadFinish 完成分块上传
func (s *FileService) ChunkUploadFinish(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ChunkUploadFinish](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	targetPath := filepath.Join(req.Path, req.FileName)

	// 检查目标文件是否已存在
	if io.Exists(targetPath) && !req.Force {
		Error(w, http.StatusForbidden, s.t.Get("target path %s already exists", targetPath))
		return
	}

	// 创建目标文件
	outFile, err := stdos.OpenFile(targetPath, stdos.O_CREATE|stdos.O_WRONLY|stdos.O_TRUNC, 0644)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("create target file error: %v", err))
		return
	}
	defer func(outFile *stdos.File) { _ = outFile.Close() }(outFile)

	// 按顺序合并分块
	prefix := s.getChunkTempFilePrefix(req.FileName, req.FileHash)
	var chunkPaths []string
	for i := 0; i < req.ChunkCount; i++ {
		chunkPath := filepath.Join(req.Path, fmt.Sprintf("%s%d", prefix, i))
		chunkPaths = append(chunkPaths, chunkPath)

		chunkData, err := stdos.ReadFile(chunkPath)
		if err != nil {
			// 删除已创建的目标文件
			_ = outFile.Close()
			_ = stdos.Remove(targetPath)
			Error(w, http.StatusInternalServerError, s.t.Get("read chunk %d error: %v", i, err))
			return
		}
		if _, err = outFile.Write(chunkData); err != nil {
			_ = outFile.Close()
			_ = stdos.Remove(targetPath)
			Error(w, http.StatusInternalServerError, s.t.Get("write chunk %d error: %v", i, err))
			return
		}
	}

	// 设置权限
	s.setPermission(targetPath, 0755, "www", "www")

	// 清理临时分块文件
	for _, chunkPath := range chunkPaths {
		_ = stdos.Remove(chunkPath)
	}

	Success(w, chix.M{
		"path": targetPath,
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

// getChunkTempFilePrefix 获取分块临时文件前缀
// 格式: .{filename}.{hash前16位}.chunk.
func (s *FileService) getChunkTempFilePrefix(fileName, fileHash string) string {
	hashPrefix := fileHash
	if len(hashPrefix) > 16 {
		hashPrefix = hashPrefix[:16]
	}
	return fmt.Sprintf(".%s.%s.chunk.", fileName, hashPrefix)
}
