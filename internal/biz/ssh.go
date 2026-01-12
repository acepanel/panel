package biz

import (
	"context"
	"time"

	"github.com/libtnb/utils/crypt"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/ssh"
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

type SSHRepo interface {
	List(page, limit uint) ([]*SSH, int64, error)
	Get(id uint) (*SSH, error)
	Create(ctx context.Context, req *request.SSHCreate) error
	Update(ctx context.Context, req *request.SSHUpdate) error
	Delete(ctx context.Context, id uint) error
}
