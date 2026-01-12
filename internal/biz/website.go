package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/types"
)

type WebsiteType string

const (
	WebsiteTypeProxy  WebsiteType = "proxy"
	WebsiteTypePHP    WebsiteType = "php"
	WebsiteTypeStatic WebsiteType = "static"
)

type Website struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	Name      string      `gorm:"not null;default:'';unique" json:"name"`
	Type      WebsiteType `gorm:"not null;index;default:'static'" json:"type"`
	Status    bool        `gorm:"not null;default:true" json:"status"`
	Path      string      `gorm:"not null;default:''" json:"path"`
	SSL       bool        `gorm:"not null;default:false" json:"ssl"`
	Remark    string      `gorm:"not null;default:''" json:"remark"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`

	CertExpire string `gorm:"-:all" json:"cert_expire"` // 仅显示
	PHP        uint   `gorm:"-:all" json:"php"`         // 仅显示

	Cert *Cert `gorm:"foreignKey:WebsiteID" json:"cert"`
}

type WebsiteRepo interface {
	GetRewrites() (map[string]string, error)
	UpdateDefaultConfig(req *request.WebsiteDefaultConfig) error
	Count() (int64, error)
	Get(id uint) (*types.WebsiteSetting, error)
	GetByName(name string) (*types.WebsiteSetting, error)
	List(typ string, page, limit uint) ([]*Website, int64, error)
	Create(ctx context.Context, req *request.WebsiteCreate) (*Website, error)
	Update(ctx context.Context, req *request.WebsiteUpdate) error
	Delete(ctx context.Context, req *request.WebsiteDelete) error
	ClearLog(id uint) error
	UpdateRemark(id uint, remark string) error
	ResetConfig(id uint) error
	UpdateStatus(id uint, status bool) error
	UpdateCert(req *request.WebsiteUpdateCert) error
	ObtainCert(ctx context.Context, id uint) error
}
