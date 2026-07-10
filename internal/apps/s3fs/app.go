package s3fs

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(i do.Injector) (*App, error) {
	t := do.MustInvoke[*gotext.Locale](i)
	return &App{
		t: t,
	}, nil
}

func (s *App) Route(r chi.Router) {
	r.Get("/mounts", s.List)
	r.Post("/mounts", s.Create)
	r.Delete("/mounts", s.Delete)
}

func (s *App) Status() string {
	return types.AppStatusNA
}

// List 所有 S3fs 挂载
func (s *App) List(w http.ResponseWriter, r *http.Request) {
	list, err := s.mounts()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get s3fs list: %v", err))
		return
	}

	paged, total := service.Paginate(r, list)

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// Create 添加 S3fs 挂载
func (s *App) Create(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Create](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 检查下地域节点中是否包含bucket，如果包含了，肯定是错误的
	if strings.Contains(req.URL, req.Bucket) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("endpoint should not contain bucket"))
		return
	}

	// 检查挂载目录是否存在且为空
	if !io.Exists(req.Path) {
		if err = os.MkdirAll(req.Path, 0755); err != nil {
			service.Error(w, http.StatusUnprocessableEntity, s.t.Get("failed to create mount path: %v", err))
			return
		}
	}
	if !io.Empty(req.Path) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("mount path is not empty"))
		return
	}

	list, err := s.mounts()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get s3fs list: %v", err))
		return
	}

	if lo.ContainsBy(list, func(item Mount) bool { return item.Path == req.Path }) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("mount path already exists"))
		return
	}

	id := time.Now().UnixMicro()
	password := req.Ak + ":" + req.Sk
	if err = io.Write("/etc/passwd-s3fs-"+cast.ToString(id), password, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to create passwd file: %v", err))
		return
	}
	if _, err = shell.Execf(`echo 's3fs#%s %s fuse3 _netdev,allow_other,url=%s,passwd_file=/etc/passwd-s3fs-%s 0 0' >> /etc/fstab`, req.Bucket, req.Path, req.URL, cast.ToString(id)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf("mount -a"); err != nil {
		_, _ = shell.Execf(`sed -i 's@^s3fs#%s\s%s.*$@@g' /etc/fstab`, req.Bucket, req.Path)
		_ = os.Remove("/etc/passwd-s3fs-" + cast.ToString(id))
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf(`df -h | grep '%s'`, req.Path); err != nil {
		_, _ = shell.Execf(`sed -i 's@^s3fs#%s\s%s.*$@@g' /etc/fstab`, req.Bucket, req.Path)
		_ = os.Remove("/etc/passwd-s3fs-" + cast.ToString(id))
		service.Error(w, http.StatusInternalServerError, s.t.Get("mount failed: %v", err))
		return
	}

	service.Success(w, nil)
}

// Delete 删除 S3fs 挂载
func (s *App) Delete(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[Delete](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	list, err := s.mounts()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get s3fs list: %v", err))
		return
	}

	mount, ok := lo.Find(list, func(item Mount) bool { return item.ID == req.ID })
	if !ok {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("mount not found"))
		return
	}

	_, _ = shell.Execf(`fusermount3 -uz '%s'`, mount.Path)
	_, err2 := shell.Execf(`umount -lf '%s'`, mount.Path)
	// 卸载之后再检查下是否还有挂载
	if _, err = shell.Execf(`df -h | grep '%s'`, mount.Path); err == nil {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("failed to unmount: %v", err2))
		return
	}

	if _, err = shell.Execf(`sed -i 's@^s3fs#%s\s%s.*$@@g' /etc/fstab`, mount.Bucket, mount.Path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if _, err = shell.Execf("mount -a"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Remove("/etc/passwd-s3fs-" + cast.ToString(mount.ID)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) mounts() ([]Mount, error) {
	re := regexp.MustCompile(`^s3fs#(.*?)\s+(.*?)\s+fuse.*?url=(.*?),passwd_file=/etc/passwd-s3fs-(.*?)\s+`)
	fstab, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(fstab), "\n")

	var mounts []Mount

	ids, err := shell.Exec("find /etc -maxdepth 1 -name 'passwd-s3fs-*'")
	if err != nil {
		return nil, err
	}
	for id := range strings.SplitSeq(ids, "\n") {
		if id == "" {
			continue
		}
		id = strings.TrimPrefix(id, "/etc/passwd-s3fs-")
		id = strings.TrimSuffix(id, "\n")
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		mount := Mount{
			ID: cast.ToInt64(id),
		}
		for _, line := range lines {
			if line == "" {
				continue
			}
			if strings.Contains(line, id) {
				matches := re.FindStringSubmatch(line)
				if len(matches) == 5 {
					mount.Bucket = matches[1]
					mount.Path = matches[2]
					mount.URL = matches[3]
					break
				}
			}
		}

		if mount.ID == 0 || mount.Path == "" || mount.Bucket == "" || mount.URL == "" {
			continue
		}

		mounts = append(mounts, mount)
	}

	return mounts, nil
}
