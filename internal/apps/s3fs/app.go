package s3fs

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/mounts", s.List)
	r.Post("/mounts", s.Create)
	r.Delete("/mounts", s.Delete)
}

// List 所有 S3fs 挂载
func (s *App) List(c fiber.Ctx) error {
	list, err := s.mounts()
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get s3fs list: %v", err))
	}

	paged, total := service.Paginate(c, list)

	return service.Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

// Create 添加 S3fs 挂载
func (s *App) Create(c fiber.Ctx) error {
	req, err := service.Bind[Create](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	// 检查下地域节点中是否包含bucket，如果包含了，肯定是错误的
	if strings.Contains(req.URL, req.Bucket) {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("endpoint should not contain bucket"))
	}

	// 检查挂载目录是否存在且为空
	if !io.Exists(req.Path) {
		if err = os.MkdirAll(req.Path, 0755); err != nil {
			return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("failed to create mount path: %v", err))
		}
	}
	if !io.Empty(req.Path) {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("mount path is not empty"))
	}

	list, err := s.mounts()
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get s3fs list: %v", err))
	}

	for _, item := range list {
		if item.Path == req.Path {
			return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("mount path already exists"))
		}
	}

	id := time.Now().UnixMicro()
	password := req.Ak + ":" + req.Sk
	if err = io.Write("/etc/passwd-s3fs-"+cast.ToString(id), password, 0600); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to create passwd file: %v", err))
	}
	if _, err = shell.Execf(`echo 's3fs#%s %s fuse _netdev,allow_other,nonempty,url=%s,passwd_file=/etc/passwd-s3fs-%s 0 0' >> /etc/fstab`, req.Bucket, req.Path, req.URL, cast.ToString(id)); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if _, err = shell.Execf("mount -a"); err != nil {
		_, _ = shell.Execf(`sed -i 's@^s3fs#%s\s%s.*$@@g' /etc/fstab`, req.Bucket, req.Path)
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if _, err = shell.Execf(`df -h | grep '%s'`, req.Path); err != nil {
		_, _ = shell.Execf(`sed -i 's@^s3fs#%s\s%s.*$@@g' /etc/fstab`, req.Bucket, req.Path)
		return service.Error(c, http.StatusInternalServerError, s.t.Get("mount failed: %v", err))
	}

	return service.Success(c, nil)
}

// Delete 删除 S3fs 挂载
func (s *App) Delete(c fiber.Ctx) error {
	req, err := service.Bind[Delete](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	list, err := s.mounts()
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get s3fs list: %v", err))
	}

	var mount Mount
	for _, item := range list {
		if item.ID == req.ID {
			mount = item
			break
		}
	}
	if mount.ID == 0 {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("mount not found"))
	}

	_, _ = shell.Execf(`fusermount -uz '%s'`, mount.Path)
	_, err2 := shell.Execf(`umount -lf '%s'`, mount.Path)
	// 卸载之后再检查下是否还有挂载
	if _, err = shell.Execf(`df -h | grep '%s'`, mount.Path); err == nil {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("failed to unmount: %v", err2))
	}

	if _, err = shell.Execf(`sed -i 's@^s3fs#%s\s%s.*$@@g' /etc/fstab`, mount.Bucket, mount.Path); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if _, err = shell.Execf("mount -a"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	if err = io.Remove("/etc/passwd-s3fs-" + cast.ToString(mount.ID)); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
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
	for _, id := range strings.Split(ids, "\n") {
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
