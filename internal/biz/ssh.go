package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/crypt"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/ssh"
)

type SSH struct {
	ID        uint             `gorm:"primaryKey" json:"id"`
	Name      string           `gorm:"not null;default:''" json:"name"`
	Host      string           `gorm:"not null;default:''" json:"host"`
	Port      uint             `gorm:"not null;default:0" json:"port"`
	Config    ssh.ClientConfig `gorm:"not null;default:'{}';serializer:json" json:"config"`
	Remark    string           `gorm:"not null;default:''" json:"remark"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

func (r *SSH) BeforeSave(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	r.Config.Key, err = crypter.Encrypt([]byte(r.Config.Key))
	if err != nil {
		return err
	}
	r.Config.Password, err = crypter.Encrypt([]byte(r.Config.Password))
	if err != nil {
		return err
	}

	return nil
}

func (r *SSH) AfterFind(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	key, err := crypter.Decrypt(r.Config.Key)
	if err == nil {
		r.Config.Key = string(key)
	}
	password, err := crypter.Decrypt(r.Config.Password)
	if err == nil {
		r.Config.Password = string(password)
	}

	return nil
}

// SSHFileInfo SFTP 文件信息
type SSHFileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	IsLink  bool   `json:"is_link"`
}

type SSHRepo interface {
	List(page, limit uint) ([]*SSH, int64, error)
	Get(id uint) (*SSH, error)
	Create(req *request.SSHCreate) error
	Update(req *request.SSHUpdate) error
	Delete(id uint) error
	// 文件互传相关,hostID 为 0 表示面板本机
	ListFiles(hostID uint, path string) ([]*SSHFileInfo, error)
	Mkdir(hostID uint, path string) error
	TransferFile(ctx context.Context, srcID uint, srcPath string, dstID uint, dstPath string, progress func(transferred, total int64)) error
}

// SSHUsecase SSH 业务逻辑
type SSHUsecase struct {
	repo SSHRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewSSHUsecase(t *gotext.Locale, log *slog.Logger, repo SSHRepo) *SSHUsecase {
	return &SSHUsecase{repo: repo, t: t, log: log}
}

func (uc *SSHUsecase) List(page, limit uint) ([]*SSH, int64, error) {
	return uc.repo.List(page, limit)
}

func (uc *SSHUsecase) Get(id uint) (*SSH, error) {
	return uc.repo.Get(id)
}

func (uc *SSHUsecase) Create(ctx context.Context, req *request.SSHCreate) error {
	if err := uc.repo.Create(req); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("ssh host created"), slog.String("type", OperationTypeSSH), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.String("host", req.Host), slog.Uint64("port", uint64(req.Port)), slog.String("user", req.User), slog.String("auth_method", req.AuthMethod))

	return nil
}

func (uc *SSHUsecase) Update(ctx context.Context, req *request.SSHUpdate) error {
	if err := uc.repo.Update(req); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("ssh host updated"), slog.String("type", OperationTypeSSH), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", req.Name), slog.String("host", req.Host), slog.Uint64("port", uint64(req.Port)), slog.String("user", req.User), slog.String("auth_method", req.AuthMethod))

	return nil
}

func (uc *SSHUsecase) Delete(ctx context.Context, id uint) error {
	s, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("ssh host deleted"), slog.String("type", OperationTypeSSH), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", s.Name), slog.String("host", s.Host), slog.Uint64("port", uint64(s.Port)))

	return nil
}

func (uc *SSHUsecase) ListFiles(hostID uint, path string) ([]*SSHFileInfo, error) {
	return uc.repo.ListFiles(hostID, path)
}

func (uc *SSHUsecase) Mkdir(ctx context.Context, hostID uint, path string) error {
	if err := uc.repo.Mkdir(hostID, path); err != nil {
		return err
	}

	attrs := []any{slog.String("type", OperationTypeSSH), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("host_id", uint64(hostID)), slog.String("path", path)}
	if hostID != 0 {
		if s, err := uc.repo.Get(hostID); err == nil {
			attrs = append(attrs, slog.String("name", s.Name), slog.String("host", s.Host))
		}
	}
	uc.log.Info(uc.t.Get("ssh directory created"), attrs...)

	return nil
}

func (uc *SSHUsecase) TransferFile(ctx context.Context, srcID uint, srcPath string, dstID uint, dstPath string, progress func(transferred, total int64)) error {
	if err := uc.repo.TransferFile(ctx, srcID, srcPath, dstID, dstPath, progress); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("ssh file transferred"), slog.String("type", OperationTypeSSH), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("src_id", uint64(srcID)), slog.String("src_path", srcPath), slog.Uint64("dst_id", uint64(dstID)), slog.String("dst_path", dstPath))

	return nil
}
