package data

import (
	"log/slog"

	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
)

type certDNSRepo struct {
	db  *gorm.DB
	log *slog.Logger
}

func NewCertDNSRepo(db *gorm.DB, log *slog.Logger) biz.CertDNSRepo {
	return &certDNSRepo{
		db:  db,
		log: log,
	}
}

func (r certDNSRepo) List(page, limit uint) ([]*biz.CertDNS, int64, error) {
	certDNS := make([]*biz.CertDNS, 0)
	var total int64
	err := r.db.Model(&biz.CertDNS{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&certDNS).Error
	return certDNS, total, err
}

func (r certDNSRepo) Get(id uint) (*biz.CertDNS, error) {
	certDNS := new(biz.CertDNS)
	err := r.db.Model(&biz.CertDNS{}).Where("id = ?", id).First(certDNS).Error
	return certDNS, err
}

func (r certDNSRepo) Create(req *request.CertDNSCreate) (*biz.CertDNS, error) {
	certDNS := &biz.CertDNS{
		Name: req.Name,
		Type: req.Type,
		Data: req.Data,
	}

	if err := r.db.Create(certDNS).Error; err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("cert dns created", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(certDNS.ID)), slog.String("name", req.Name))

	return certDNS, nil
}

func (r certDNSRepo) Update(req *request.CertDNSUpdate) error {
	cert, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	cert.Name = req.Name
	cert.Type = req.Type
	cert.Data = req.Data

	if err = r.db.Save(cert).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cert dns updated", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(req.ID)), slog.String("name", req.Name))

	return nil
}

func (r certDNSRepo) Delete(id uint) error {
	certDNS, err := r.Get(id)
	if err != nil {
		return err
	}

	if err = r.db.Model(&biz.CertDNS{}).Where("id = ?", id).Delete(&biz.CertDNS{}).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("cert dns deleted", slog.String("type", biz.OperationTypeCert), slog.Uint64("operator_id", 0), slog.Uint64("id", uint64(id)), slog.String("name", certDNS.Name))

	return nil
}
