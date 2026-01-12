package biz

import (
	"context"
	"time"

	mholtacme "github.com/mholt/acmez/v3/acme"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/acme"
	"github.com/acepanel/panel/pkg/types"
)

type Cert struct {
	ID          uint                  `gorm:"primaryKey" json:"id"`
	AccountID   uint                  `gorm:"not null;default:0" json:"account_id"` // 关联的 ACME 账户 ID
	WebsiteID   uint                  `gorm:"not null;default:0" json:"website_id"` // 关联的网站 ID
	DNSID       uint                  `gorm:"not null;default:0" json:"dns_id"`     // 关联的 DNS ID
	Type        string                `gorm:"not null;default:''" json:"type"`      // 证书类型 (P256, P384, 2048, 3072, 4096)
	Domains     []string              `gorm:"not null;default:'[]';serializer:json" json:"domains"`
	AutoRenewal bool                  `gorm:"not null;default:false" json:"auto_renewal"`                // 自动续签
	RenewalInfo mholtacme.RenewalInfo `gorm:"not null;default:'{}';serializer:json" json:"renewal_info"` // 续签信息
	CertURL     string                `gorm:"not null;default:''" json:"cert_url"`                       // 证书 URL (续签时使用)
	Cert        string                `gorm:"not null;default:''" json:"cert"`                           // 证书内容
	Key         string                `gorm:"not null;default:''" json:"key"`                            // 私钥内容
	Script      string                `gorm:"not null;default:''" json:"script"`                         // 部署脚本
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`

	Website *Website     `gorm:"foreignKey:WebsiteID" json:"website"`
	Account *CertAccount `gorm:"foreignKey:AccountID" json:"account"`
	DNS     *CertDNS     `gorm:"foreignKey:DNSID" json:"dns"`
}

type CertRepo interface {
	List(page, limit uint) ([]*types.CertList, int64, error)
	Get(id uint) (*Cert, error)
	GetByWebsite(WebsiteID uint) (*Cert, error)
	Upload(ctx context.Context, req *request.CertUpload) (*Cert, error)
	Create(ctx context.Context, req *request.CertCreate) (*Cert, error)
	Update(ctx context.Context, req *request.CertUpdate) error
	Delete(ctx context.Context, id uint) error
	ObtainAuto(id uint) (*acme.Certificate, error)
	ObtainManual(id uint) (*acme.Certificate, error)
	ObtainPanel(account *CertAccount, ips []string) ([]byte, []byte, error)
	ObtainSelfSigned(id uint) error
	Renew(id uint) (*acme.Certificate, error)
	RefreshRenewalInfo(id uint) (mholtacme.RenewalInfo, error)
	ManualDNS(id uint) ([]acme.DNSRecord, error)
	Deploy(ID, WebsiteID uint) error
}
