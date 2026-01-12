package biz

import (
	"context"
	"image"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"not null;default:'';unique" json:"username"`
	Password  string         `gorm:"not null;default:''" json:"password"`
	Email     string         `gorm:"not null;default:''" json:"email"`
	TwoFA     string         `gorm:"not null;default:''" json:"two_fa"` // 2FA secret，为空表示未开启
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	Tokens []*UserToken `gorm:"foreignKey:UserID" json:"-"`
}

type UserRepo interface {
	List(page, limit uint) ([]*User, int64, error)
	Get(id uint) (*User, error)
	Create(ctx context.Context, username, password, email string) (*User, error)
	UpdateUsername(ctx context.Context, id uint, username string) error
	UpdatePassword(ctx context.Context, id uint, password string) error
	UpdateEmail(ctx context.Context, id uint, email string) error
	Delete(ctx context.Context, id uint) error
	CheckPassword(username, password string) (*User, error)
	IsTwoFA(username string) (bool, error)
	GenerateTwoFA(id uint) (image.Image, string, string, error)
	UpdateTwoFA(id uint, code, secret string) error
}
