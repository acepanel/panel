package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/acme"
)

type CertDNS struct {
	ID        uint          `gorm:"primaryKey" json:"id"`
	Name      string        `gorm:"not null;default:''" json:"name"`       // 备注名称
	Type      acme.DnsType  `gorm:"not null;default:'aliyun'" json:"type"` // DNS 提供商
	Data      acme.DNSParam `gorm:"not null;default:'{}';serializer:json" json:"dns_param"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	Certs []*Cert `gorm:"foreignKey:DNSID" json:"-"`
}

type CertDNSRepo interface {
	List(page, limit uint) ([]*CertDNS, int64, error)
	Get(id uint) (*CertDNS, error)
	Create(req *request.CertDNSCreate) (*CertDNS, error)
	Update(req *request.CertDNSUpdate) error
	Delete(id uint) error
}

type CertDNSUsecase struct {
	repo CertDNSRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewCertDNSUsecase(t *gotext.Locale, log *slog.Logger, repo CertDNSRepo) *CertDNSUsecase {
	return &CertDNSUsecase{repo: repo, t: t, log: log}
}

func (uc *CertDNSUsecase) List(page, limit uint) ([]*CertDNS, int64, error) {
	return uc.repo.List(page, limit)
}

func (uc *CertDNSUsecase) Get(id uint) (*CertDNS, error) {
	return uc.repo.Get(id)
}

func (uc *CertDNSUsecase) Create(ctx context.Context, req *request.CertDNSCreate) (*CertDNS, error) {
	certDNS, err := uc.repo.Create(req)
	if err != nil {
		return nil, err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert dns created"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(certDNS.ID)), slog.String("name", req.Name))

	return certDNS, nil
}

func (uc *CertDNSUsecase) Update(ctx context.Context, req *request.CertDNSUpdate) error {
	if err := uc.repo.Update(req); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("cert dns updated"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", req.Name))

	return nil
}

func (uc *CertDNSUsecase) Delete(ctx context.Context, id uint) error {
	certDNS, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("cert dns deleted"), slog.String("type", OperationTypeCert), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", certDNS.Name))

	return nil
}
