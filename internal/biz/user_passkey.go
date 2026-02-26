package biz

import (
	"time"

	"gorm.io/gorm"
)

type UserPasskey struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"index;not null" json:"user_id"`
	Name         string         `gorm:"not null;default:''" json:"name"`
	CredentialID []byte         `gorm:"uniqueIndex;not null" json:"-"`
	PublicKey    []byte         `gorm:"not null" json:"-"`
	AAGUID       []byte         `json:"-"`
	SignCount    uint32         `gorm:"not null;default:0" json:"-"`
	Transports   string         `gorm:"not null;default:''" json:"transports"` // JSON: ["internal","hybrid"]
	BackupState  bool           `gorm:"not null;default:false" json:"-"`
	LastUsedAt   *time.Time     `json:"last_used_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

type UserPasskeyRepo interface {
	List(userID uint) ([]*UserPasskey, error)
	Create(passkey *UserPasskey) error
	UpdateSignCount(credentialID []byte, signCount uint32) error
	UpdateLastUsed(credentialID []byte) error
	Delete(userID, id uint) error
	GetByCredentialID(credentialID []byte) (*UserPasskey, *User, error)
	HasPasskey(userID uint) (bool, error)
	HasAny() (bool, error)
	DeleteAllByUserID(userID uint) error
}
