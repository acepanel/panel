package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
)

type CertAccount struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Email       string    `gorm:"not null;default:''" json:"email"`
	CA          string    `gorm:"not null;default:'letsencrypt'" json:"ca"` // CA 提供商 (letsencrypt, zerossl, sslcom, google, buypass)
	Kid         string    `gorm:"not null;default:''" json:"kid"`
	HmacEncoded string    `gorm:"not null;default:''" json:"hmac_encoded"`
	PrivateKey  string    `gorm:"not null;default:''" json:"private_key"`
	KeyType     string    `gorm:"not null;default:'P256'" json:"key_type"` // 密钥类型 (P256, P384, 2048, 3072, 4096)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Certs []*Cert `gorm:"foreignKey:AccountID" json:"-"`
}

type CertAccountRepo interface {
	List(page, limit uint) ([]*CertAccount, int64, error)
	GetDefault(userID uint) (*CertAccount, error)
	Get(id uint) (*CertAccount, error)
	Create(ctx context.Context, req *request.CertAccountCreate) (*CertAccount, error)
	Update(ctx context.Context, req *request.CertAccountUpdate) error
	Delete(ctx context.Context, id uint) error
}
