package service

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"github.com/libtnb/chix/v2"
	"github.com/libtnb/utils/file"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/charset"
	"github.com/acepanel/panel/v3/pkg/chattr"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type FileService struct {
	t             *gotext.Locale
	taskRepo      *biz.TaskUsecase
	containerRepo *biz.ContainerUsecase
	tamperRepo    *biz.TamperUsecase
}

func NewFileService(containerUsecase *biz.ContainerUsecase, tamperUsecase *biz.TamperUsecase, taskUsecase *biz.TaskUsecase, t *gotext.Locale) *FileService {
	return &FileService{
		t:             t,
		taskRepo:      taskUsecase,
		containerRepo: containerUsecase,
		tamperRepo:    tamperUsecase,
	}
}

func (s *FileService) Create(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileCreate](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if !req.Dir {
		f, err := stdos.OpenFile(req.Path, stdos.O_CREATE|stdos.O_WRONLY, 0644)
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		_ = f.Close()
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
	req, err := Bind[request.FileContent](r)
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

	if req.Encoding == charset.Auto {
		req.Encoding = charset.Detect(content)
	}
	if content, err = charset.Decode(content, req.Encoding); err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	Success(w, chix.M{
		"mime":     mime,
		"encoding": req.Encoding,
		"content":  base64.StdEncoding.EncodeToString(content),
	})
}

// Tail 反向分页读取文件或 systemd 服务日志
// offset: 从末尾起跳过的行数（0 表示从最末尾开始）
// limit: 读取的行数（最多 5000）
// 返回: lines 当前块的行（按文件正序），has_more 是否还有更早的内容，size 文件总字节
func (s *FileService) Tail(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileTail](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if req.Limit <= 0 {
		req.Limit = 500
	}
	if req.Limit > 5000 {
		req.Limit = 5000
	}
	// 限制回溯深度，三种日志源共用；再深的历史交给下载原始日志
	req.Offset = min(max(req.Offset, 0), 10000)

	if req.Path == "" && req.Service == "" && req.Container == "" {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("path, service or container is required"))
		return
	}

	if req.Service != "" {
		s.tailService(r.Context(), w, req)
		return
	}

	if req.Container != "" {
		s.tailContainer(r.Context(), w, req)
		return
	}

	f, err := stdos.Open(req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func(f *stdos.File) { _ = f.Close() }(f)

	stat, err := f.Stat()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	size := stat.Size()
	if size == 0 {
		Success(w, chix.M{"lines": []string{}, "has_more": false, "size": size})
		return
	}
	// 以首屏返回的大小为锚点反向分页，否则跟踪期间写入的新行会顶掉偏移量导致翻页重复
	anchor := size
	if req.Size > 0 {
		anchor = min(req.Size, size)
	}

	// 从锚点反向读取，直到攒够 offset+limit+1 个换行符（多 1 是为了避免读到不完整的首行）
	// 首块按预估行长一次读足，绝大多数请求一两次系统调用即可完成；maxScan 兜住超长行
	const maxScan = int64(64 << 20)
	needLines := req.Offset + req.Limit + 1
	readSize := min(int64(needLines)*256, maxScan)
	pos := anchor
	chunks := make([][]byte, 0, 4)
	newlineCount := 0
	for pos > 0 && newlineCount < needLines && anchor-pos < maxScan {
		readSize = min(readSize, pos)
		pos -= readSize
		buf := make([]byte, readSize)
		if _, rerr := f.ReadAt(buf, pos); rerr != nil && rerr != stdio.EOF {
			Error(w, http.StatusInternalServerError, "%v", rerr)
			return
		}
		newlineCount += bytes.Count(buf, []byte{'\n'})
		chunks = append(chunks, buf)
	}
	slices.Reverse(chunks)
	data := bytes.Join(chunks, nil)

	// 按字节切分，只把真正返回的那一页转成 string，避免整个扫描窗口再复制一份
	all := bytes.Split(bytes.TrimRight(data, "\n"), []byte{'\n'})
	totalLoaded := len(all)

	// 当 pos > 0 时第一行可能不完整，丢弃以避免半行被显示
	startBoundary := 0
	if pos > 0 {
		startBoundary = 1
	}

	endIdx := totalLoaded - req.Offset
	startIdx := max(endIdx-req.Limit, startBoundary)
	if endIdx < startIdx {
		endIdx = startIdx
	}
	if endIdx > totalLoaded {
		endIdx = totalLoaded
	}

	hasMore := pos > 0 || startIdx > startBoundary

	result := make([]string, 0, max(endIdx-startIdx, 0))
	for _, line := range all[startIdx:endIdx] {
		result = append(result, string(line))
	}

	Success(w, chix.M{
		"lines":    result,
		"has_more": hasMore,
		"size":     size,
	})
}

// Truncate 截断文件至 0 长度（保留文件本身和元数据）
func (s *FileService) Truncate(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = stdos.Truncate(req.Path, 0); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
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

	content, err := charset.Encode([]byte(req.Content), req.Encoding)
	if err != nil {
		if unsupported, ok := errors.AsType[*charset.UnsupportedRuneError](err); ok {
			Error(w, http.StatusUnprocessableEntity, s.t.Get("character %s on line %d cannot be saved as %s", strconv.QuoteRune(unsupported.Rune), unsupported.Line, req.Encoding))
			return
		}
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if s.tamperRepo.Unlock(req.Path) {
		defer s.tamperRepo.Relock(req.Path)
	}

	if err = io.Write(req.Path, string(content), fileInfo.Mode()); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// protectedPath 面板所在路径及其祖先
func protectedPath(path string) bool {
	path = filepath.Clean(path)
	for _, p := range []string{app.Root, filepath.Join(app.Root, "server"), filepath.Join(app.Root, "panel")} {
		if p == path || strings.HasPrefix(p, strings.TrimSuffix(path, "/")+"/") {
			return true
		}
	}
	return false
}

func (s *FileService) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePath](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if protectedPath(req.Path) {
		Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
		return
	}

	// 解除防篡改保护后再删除
	unlocked := s.tamperRepo.Unlock(req.Path)
	if err = io.Remove(req.Path); err != nil {
		if unlocked {
			s.tamperRepo.Relock(req.Path)
		}
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *FileService) Upload(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileUpload](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if io.Exists(req.Path) && !req.Force {
		Error(w, http.StatusForbidden, s.t.Get("target path %s already exists", req.Path))
		return
	}

	dir := filepath.Dir(req.Path)
	if err = stdos.MkdirAll(dir, 0755); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("create directory error: %v", err))
		return
	}

	// 先写同目录临时文件再 rename 替换，中途失败不留半截文件；替换成功后临时文件已不存在，Remove 无害
	tmp, err := stdos.CreateTemp(dir, "."+filepath.Base(req.Path)+".*.part")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("open file error: %v", err))
		return
	}
	defer func() {
		_ = tmp.Close()
		_ = stdos.Remove(tmp.Name())
	}()

	src, err := req.File.Open()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("upload file error: %v", err))
		return
	}
	_, err = stdio.Copy(tmp, src)
	_ = src.Close()
	_ = tmp.Close()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("write file error: %v", err))
		return
	}
	if err = s.replaceFile(tmp.Name(), req.Path); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("write file error: %v", err))
		return
	}

	Success(w, nil)
}

// replaceFile 用临时文件原子替换目标文件；目标受防篡改保护时先解锁，替换后重新登记
// rename 不会打开目标文件，覆盖运行中的二进制也不会遇到 ETXTBSY
func (s *FileService) replaceFile(tmp, target string) error {
	if io.Exists(target) && s.tamperRepo.Unlock(target) {
		defer s.tamperRepo.Relock(target)
	}
	if err := stdos.Rename(tmp, target); err != nil {
		return err
	}
	s.setPermission(target, 0755, "www", "www")
	return nil
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

	ctx := r.Context()
	for item := range slices.Values(req) {
		// 源和目标相同，跳过（同目录粘贴覆盖的情况）
		if item.Source == item.Target {
			continue
		}

		if io.Exists(item.Target) && !item.Force {
			continue
		}

		if io.IsDir(item.Source) && strings.HasPrefix(item.Target, item.Source+"/") {
			Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
			return
		}

		if err := s.moveItem(ctx, item.Source, item.Target); err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	Success(w, nil)
}

// moveItem 移动单个文件；源受防篡改保护时先解锁，结束后按最终落点重新登记
// mv 失败（如请求取消导致 SIGKILL）时源仍在原地，不按源路径补登记会让该文件永久失去保护
func (s *FileService) moveItem(ctx context.Context, source, target string) (err error) {
	if s.tamperRepo.Unlock(source) {
		defer func() {
			if err != nil {
				s.tamperRepo.Relock(source)
			} else {
				s.tamperRepo.Relock(target)
			}
		}()
	}

	return io.Mv(ctx, source, target)
}

func (s *FileService) Copy(w http.ResponseWriter, r *http.Request) {
	binder := chix.NewBind(r)
	defer binder.Release()

	var req []request.FileControl
	if err := binder.Body(&req); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	ctx := r.Context()
	for item := range slices.Values(req) {
		// 源和目标相同，跳过（同目录粘贴覆盖的情况）
		if item.Source == item.Target {
			continue
		}

		if io.Exists(item.Target) && !item.Force {
			continue
		}

		if io.IsDir(item.Source) && strings.HasPrefix(item.Target, item.Source+"/") {
			Error(w, http.StatusForbidden, s.t.Get("please don't do this"))
			return
		}

		if err := io.Cp(ctx, item.Source, item.Target); err != nil {
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

	task := new(biz.Task)
	task.Key = "download:" + req.Path
	task.Name = s.t.Get("Download remote file %v", filepath.Base(req.Path))
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`aria2c -c --file-allocation=falloc --allow-overwrite=true --auto-file-renaming=false --check-certificate=false --retry-wait=5 --max-tries=5 -x 16 -s 16 -k 1M -d '%s' -o '%s' '%s' && %s`, filepath.Dir(req.Path), filepath.Base(req.Path), req.URL, s.wwwShell(req.Path, false))
	task.CancelShell = fmt.Sprintf(`rm -f '%s' '%s.aria2'`, req.Path, req.Path)

	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, task)
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
		"hidden":    strings.HasPrefix(info.Name(), "."),
		"symlink":   info.Mode()&stdos.ModeSymlink != 0,
		"link":      readlink(req.Path),
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
	size, err := io.Size(r.Context(), req.Path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, tools.FormatBytes(float64(size)))
}

func (s *FileService) Permission(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FilePermission](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 解析成8进制
	mode, err := strconv.ParseUint(req.Mode, 8, 32)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if req.Recursive {
		if err = io.ChmodR(r.Context(), req.Path, stdos.FileMode(mode)); err == nil {
			err = io.ChownR(r.Context(), req.Path, req.Owner, req.Group)
		}
	} else if err = io.Chmod(req.Path, stdos.FileMode(mode)); err == nil {
		err = io.Chown(req.Path, req.Owner, req.Group)
	}
	if err != nil {
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

	cmd, err := io.CompressShell(req.Dir, req.Paths, req.File, true)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	task := new(biz.Task)
	task.Key = "compress:" + req.File
	task.Name = s.t.Get("Compress %v", filepath.Base(req.File))
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd + " && " + s.wwwShell(req.File, false)
	task.CancelShell = "rm -f " + shell.Quote(req.File)

	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, task)
}

func (s *FileService) UnCompress(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileUnCompress](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	cmd, err := io.UnCompressShell(req.File, req.Path, true)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	task := new(biz.Task)
	task.Key = fmt.Sprintf("uncompress:%s:%s", req.File, req.Path)
	task.Name = s.t.Get("Uncompress %v", filepath.Base(req.File))
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd + " && " + s.wwwShell(req.Path, true)

	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, task)
}

func (s *FileService) List(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileList](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var entries []fileEntry
	if req.Keyword != "" {
		found, err := io.Search(r.Context(), req.Path, req.Keyword, req.Sub)
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		entries = lo.Map(found, func(e io.Entry, _ int) fileEntry { return fileEntry{path: e.Path, info: e.Info} })
	} else {
		list, err := stdos.ReadDir(req.Path)
		if err != nil {
			Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		entries = lo.FilterMap(list, func(d stdos.DirEntry, _ int) (fileEntry, bool) {
			info, err := d.Info()
			if err != nil {
				return fileEntry{}, false
			}
			return fileEntry{path: filepath.Join(req.Path, d.Name()), info: info}, true
		})
	}

	if req.ExcludeHidden {
		entries = lo.Reject(entries, func(e fileEntry, _ int) bool { return strings.HasPrefix(e.info.Name(), ".") })
	}

	// 前缀 - 表示降序
	sortKey := req.Sort
	sortDesc := false
	if strings.HasPrefix(sortKey, "-") {
		sortDesc = true
		sortKey = strings.TrimPrefix(sortKey, "-")
	}

	slices.SortFunc(entries, func(a, b fileEntry) int {
		// 文件夹始终排在前面（除非按特定字段排序）
		if sortKey == "" {
			if a.info.IsDir() && !b.info.IsDir() {
				return -1
			}
			if !a.info.IsDir() && b.info.IsDir() {
				return 1
			}
		}

		var order int
		switch sortKey {
		case "size":
			order = cmp.Compare(a.info.Size(), b.info.Size())
		case "modify":
			order = a.info.ModTime().Compare(b.info.ModTime())
		default:
			order = strings.Compare(strings.ToLower(a.info.Name()), strings.ToLower(b.info.Name()))
		}

		if sortDesc {
			order = -order
		}
		return order
	})

	paged, total := Paginate(r, s.formatEntries(entries))

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// 分块上传的位图文件：前 8 字节是分块大小(uint64 小端)，之后每个分块 1 字节完成标记
const chunkMapHeader = 8

// ChunkUploadStart 开始分块上传，返回已完成的分块供续传
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
	if err = stdos.MkdirAll(req.Path, 0755); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("create directory error: %v", err))
		return
	}

	// 分块数或分块大小对不上（换了分块参数或不是同一次上传），从头开始
	part, mapPath := s.chunkTempPaths(req.Path, req.FileName, req.FileHash)
	bitmap, _ := stdos.ReadFile(mapPath)
	//nolint:gosec
	if len(bitmap) != chunkMapHeader+req.ChunkCount ||
		binary.LittleEndian.Uint64(bitmap) != uint64(req.ChunkSize) {
		bitmap = make([]byte, chunkMapHeader+req.ChunkCount)
		binary.LittleEndian.PutUint64(bitmap, uint64(req.ChunkSize)) //nolint:gosec
		if err = stdos.WriteFile(mapPath, bitmap, 0644); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("save chunk error: %v", err))
			return
		}
		_ = stdos.Remove(part)
	}

	uploadedChunks := make([]int, 0)
	for i, done := range bitmap[chunkMapHeader:] {
		if done == 1 {
			uploadedChunks = append(uploadedChunks, i)
		}
	}

	Success(w, chix.M{
		"uploaded_chunks": uploadedChunks,
	})
}

// ChunkUploadChunk 上传单个分块，流式写入数据文件的对应偏移
func (s *FileService) ChunkUploadChunk(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ChunkUpload](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 位图由 start 创建，同时提供分块总数和分块大小
	part, mapPath := s.chunkTempPaths(req.Path, req.FileName, req.FileHash)
	bitmap, err := stdos.ReadFile(mapPath)
	if err != nil || len(bitmap) <= chunkMapHeader || binary.LittleEndian.Uint64(bitmap) == 0 {
		Error(w, http.StatusBadRequest, s.t.Get("chunk upload not started"))
		return
	}
	chunkCount := len(bitmap) - chunkMapHeader
	chunkSize := int64(binary.LittleEndian.Uint64(bitmap)) //nolint:gosec
	if req.ChunkIndex >= chunkCount {
		Error(w, http.StatusBadRequest, s.t.Get("chunk index out of range"))
		return
	}

	src, err := req.File.Open()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("open upload file error: %v", err))
		return
	}
	defer func() { _ = src.Close() }()

	file, err := stdos.OpenFile(part, stdos.O_WRONLY|stdos.O_CREATE, 0644)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("save chunk error: %v", err))
		return
	}
	defer func() { _ = file.Close() }()

	// 边写边算 hash；LimitReader 保证不会写到本块范围之外
	hasher := sha256.New()
	writer := stdio.MultiWriter(stdio.NewOffsetWriter(file, int64(req.ChunkIndex)*chunkSize), hasher)
	n, err := stdio.Copy(writer, stdio.LimitReader(src, chunkSize))
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("save chunk error: %v", err))
		return
	}

	// 非末块必须是整块，末块不能超过分块大小，有多余数据说明分块大小对不上
	extra := make([]byte, 1)
	if m, _ := src.Read(extra); m > 0 || n == 0 || (req.ChunkIndex < chunkCount-1 && n != chunkSize) {
		Error(w, http.StatusBadRequest, s.t.Get("chunk size mismatch"))
		return
	}
	if req.ChunkHash != "" && !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), req.ChunkHash) {
		Error(w, http.StatusBadRequest, s.t.Get("chunk hash mismatch"))
		return
	}

	// 标记完成：不同分块写不同字节，无需加锁
	mapFile, err := stdos.OpenFile(mapPath, stdos.O_WRONLY, 0644)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("save chunk error: %v", err))
		return
	}
	defer func() { _ = mapFile.Close() }()
	if _, err = mapFile.WriteAt([]byte{1}, int64(chunkMapHeader+req.ChunkIndex)); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("save chunk error: %v", err))
		return
	}

	Success(w, chix.M{
		"chunk_index": req.ChunkIndex,
	})
}

// ChunkUploadFinish 完成分块上传：校验位图后把数据文件 rename 成目标文件
func (s *FileService) ChunkUploadFinish(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ChunkUploadFinish](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	part, mapPath := s.chunkTempPaths(req.Path, req.FileName, req.FileHash)
	bitmap, err := stdos.ReadFile(mapPath)
	if err != nil || len(bitmap) != chunkMapHeader+req.ChunkCount {
		Error(w, http.StatusBadRequest, s.t.Get("chunk upload not started or chunk count mismatch"))
		return
	}
	for i, done := range bitmap[chunkMapHeader:] {
		if done != 1 {
			Error(w, http.StatusBadRequest, s.t.Get("chunk %d is missing", i))
			return
		}
	}

	targetPath := filepath.Join(req.Path, req.FileName)
	if io.Exists(targetPath) && !req.Force {
		Error(w, http.StatusForbidden, s.t.Get("target path %s already exists", targetPath))
		return
	}
	if err = s.replaceFile(part, targetPath); err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("write file error: %v", err))
		return
	}
	_ = stdos.Remove(mapPath)

	Success(w, chix.M{
		"path": targetPath,
	})
}

// ChunkUploadCancel 取消分块上传，删除临时文件
func (s *FileService) ChunkUploadCancel(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ChunkUploadFile](r)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	part, mapPath := s.chunkTempPaths(req.Path, req.FileName, req.FileHash)
	_ = stdos.Remove(part)
	_ = stdos.Remove(mapPath)

	Success(w, nil)
}

type fileEntry struct {
	path string
	info stdos.FileInfo
}

func (s *FileService) formatEntries(entries []fileEntry) []any {
	var paths []any
	for e := range slices.Values(entries) {
		info := e.info
		// Linux 下 Sys() 必定是 *syscall.Stat_t
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			stat = &syscall.Stat_t{}
		}
		// 对于目录，size 返回空字符串，需要用户手动计算
		size := ""
		if !info.IsDir() {
			size = tools.FormatBytes(float64(info.Size()))
		}

		// 检查是否有 immutable 属性
		immutable := false
		if f, err := stdos.OpenFile(e.path, stdos.O_RDONLY, 0); err == nil {
			immutable, _ = chattr.IsAttr(f, chattr.FS_IMMUTABLE_FL)
			_ = f.Close()
		}

		paths = append(paths, map[string]any{
			"name":      info.Name(),
			"full":      e.path,
			"size":      size,
			"mode_str":  info.Mode().String(),
			"mode":      fmt.Sprintf("%04o", info.Mode().Perm()),
			"owner":     os.GetUser(stat.Uid),
			"group":     os.GetGroup(stat.Gid),
			"uid":       stat.Uid,
			"gid":       stat.Gid,
			"hidden":    strings.HasPrefix(info.Name(), "."),
			"symlink":   info.Mode()&stdos.ModeSymlink != 0,
			"link":      readlink(e.path),
			"dir":       info.IsDir(),
			"modify":    info.ModTime().Format(time.DateTime),
			"immutable": immutable,
		})
	}

	return paths
}

func readlink(path string) string {
	link, _ := stdos.Readlink(path)
	return link
}

// setPermission 设置权限
func (s *FileService) setPermission(path string, mode stdos.FileMode, owner, group string) {
	_ = io.Chmod(path, mode)
	_ = io.Chown(path, owner, group)
}

func (s *FileService) wwwShell(path string, recursive bool) string {
	flag := lo.Ternary(recursive, "-R ", "")
	quoted := shell.Quote(path)
	return fmt.Sprintf("{ chmod %s0755 %s; chown %swww:www %s; true; }", flag, quoted, flag, quoted)
}

// chunkTempPaths 分块上传的临时文件：同目录下的稀疏数据文件和位图文件
func (s *FileService) chunkTempPaths(dir, fileName, fileHash string) (string, string) {
	part := filepath.Join(dir, fmt.Sprintf(".%s.%s.part", fileName, fileHash[:16]))
	return part, part + ".map"
}

// tailService 用 journalctl cursor 反向分页读取 systemd 服务日志
func (s *FileService) tailService(ctx context.Context, w http.ResponseWriter, req *request.FileTail) {
	cmd := fmt.Sprintf("journalctl --no-pager -o json -u %s -n %d", req.Service, req.Limit)
	if req.Cursor != "" {
		cmd += fmt.Sprintf(" --before-cursor %q", req.Cursor)
	}
	out, err := shell.Exec(ctx, cmd)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	type entry struct {
		Cursor    string `json:"__CURSOR"`
		Timestamp string `json:"__REALTIME_TIMESTAMP"`
		Hostname  string `json:"_HOSTNAME"`
		Ident     string `json:"SYSLOG_IDENTIFIER"`
		Comm      string `json:"_COMM"`
		PID       string `json:"_PID"`
		Message   string `json:"MESSAGE"`
	}

	entries := make([]entry, 0, req.Limit)
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var e entry
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		entries = append(entries, e)
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, formatJournalLine(e.Timestamp, e.Hostname, e.Ident, e.Comm, e.PID, e.Message))
	}

	// next_cursor 是本次结果中最早那条的 cursor，供下一页 --before-cursor 继续往前翻
	nextCursor := ""
	if len(entries) > 0 {
		nextCursor = entries[0].Cursor
	}
	hasMore := len(lines) >= req.Limit

	Success(w, chix.M{
		"lines":       lines,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
		"size":        0,
	})
}

// formatJournalLine 按 syslog 风格拼装单行日志
func formatJournalLine(ts, hostname, ident, comm, pid, message string) string {
	var sb strings.Builder
	if us, err := strconv.ParseInt(ts, 10, 64); err == nil {
		sb.WriteString(time.Unix(0, us*int64(time.Microsecond)).Format("Jan 02 15:04:05"))
		sb.WriteByte(' ')
	}
	if hostname != "" {
		sb.WriteString(hostname)
		sb.WriteByte(' ')
	}
	if ident == "" {
		ident = comm
	}
	if ident != "" {
		sb.WriteString(ident)
		if pid != "" {
			sb.WriteByte('[')
			sb.WriteString(pid)
			sb.WriteByte(']')
		}
		sb.WriteString(": ")
	}
	sb.WriteString(message)
	return sb.String()
}

// tailContainer 反向读取容器末尾日志
func (s *FileService) tailContainer(ctx context.Context, w http.ResponseWriter, req *request.FileTail) {
	// 容器日志只能整段拉取再切片，回溯深度由 Tail 顶部统一钳制
	total := req.Offset + req.Limit
	out, err := s.containerRepo.Logs(ctx, req.Container, total)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	all := strings.Split(strings.TrimRight(out, "\n"), "\n")
	totalLoaded := len(all)
	endIdx := totalLoaded - req.Offset
	startIdx := max(endIdx-req.Limit, 0)
	if endIdx < startIdx {
		endIdx = startIdx
	}
	if endIdx > totalLoaded {
		endIdx = totalLoaded
	}
	result := []string{}
	if startIdx < endIdx {
		result = all[startIdx:endIdx]
	}
	// 取到的行数达到请求总数，说明可能还有更早的日志
	hasMore := totalLoaded >= total
	Success(w, chix.M{
		"lines":    result,
		"has_more": hasMore,
		"size":     0,
	})
}
