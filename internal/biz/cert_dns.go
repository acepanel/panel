package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/acme"
)

type CertDNS struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"not null;default:''" json:"name"`       // 备注名称
	Type      acme.DnsType  `gorm:"not null;default:'aliyun'" json:"type"` // DNS 提供商
	Data      acme.DNSParam `gorm:"not null;serializer:json" json:"dns_param"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	Certs []*Cert `gorm:"foreignKey:DNSID" json:"-"`
}

type CertDNSRepo interface {
	List(page, limit uint) ([]*CertDNS, int64, error)
	Get(id uint) (*CertDNS, error)
	Create(ctx context.Context, req *request.CertDNSCreate) (*CertDNS, error)
	Update(ctx context.Context, req *request.CertDNSUpdate) error
	Delete(ctx context.Context, id uint) error
}
