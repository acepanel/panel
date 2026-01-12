package biz

import (
	"context"
	"time"

	"github.com/libtnb/utils/crypt"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/http/request"
)

type DatabaseUserStatus string

const (
	DatabaseUserStatusValid   DatabaseUserStatus = "valid"
	DatabaseUserStatusInvalid DatabaseUserStatus = "invalid"
)

type DatabaseUser struct {
	ID         uint               `gorm:"primaryKey" json:"id"`
	ServerID   uint               `gorm:"not null;default:0" json:"server_id"`
	Username   string             `gorm:"not null;default:''" json:"username"`
	Password   string             `gorm:"not null;default:''" json:"password"`
	Host       string             `gorm:"not null;default:''" json:"host"` // 仅 mysql
	Status     DatabaseUserStatus `gorm:"-:all" json:"status"`             // 仅显示
	Privileges []string           `gorm:"-:all" json:"privileges"`         // 仅显示
	Remark     string             `gorm:"not null;default:''" json:"remark"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`

	Server *DatabaseServer `gorm:"foreignKey:ServerID;references:ID" json:"server"`
}

func (r *DatabaseUser) BeforeSave(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	r.Password, err = crypter.Encrypt([]byte(r.Password))
	if err != nil {
		return err
	}

	return nil

}

func (r *DatabaseUser) AfterFind(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	password, err := crypter.Decrypt(r.Password)
	if err == nil {
		r.Password = string(password)
	}

	return nil
}

type DatabaseUserRepo interface {
	Count() (int64, error)
	List(page, limit uint) ([]*DatabaseUser, int64, error)
	Get(id uint) (*DatabaseUser, error)
	Create(ctx context.Context, req *request.DatabaseUserCreate) error
	Update(req *request.DatabaseUserUpdate) error
	UpdateRemark(req *request.DatabaseUserUpdateRemark) error
	Delete(ctx context.Context, id uint) error
	DeleteByNames(serverID uint, names []string) error
}
