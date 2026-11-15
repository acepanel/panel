package data

import (
	"context"
	"errors"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/storage"
)

type backupAccountRepo struct {
	db *gorm.DB
	t  *gotext.Locale
}

func NewBackupAccountRepo(db *gorm.DB, t *gotext.Locale) biz.BackupAccountRepo {
	return &backupAccountRepo{
		db: db,
		t:  t,
	}
}

func (r backupAccountRepo) ListPaged(page, limit uint) ([]*biz.BackupStorage, int64, error) {
	var dbAccounts []*biz.BackupStorage
	var total int64
	if err := r.db.Model(&biz.BackupStorage{}).Order("id asc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&dbAccounts).Error; err != nil {
		return nil, 0, err
	}

	return dbAccounts, total, nil
}

func (r backupAccountRepo) GetByID(id uint) (*biz.BackupStorage, error) {
	account := new(biz.BackupStorage)
	if err := r.db.Model(&biz.BackupStorage{}).Where("id = ?", id).First(account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(r.t.Get("backup storage not found"))
		}
		return nil, err
	}
	return account, nil
}

func (r backupAccountRepo) Create(account *biz.BackupStorage) error {
	return r.db.Create(account).Error
}

func (r backupAccountRepo) Update(account *biz.BackupStorage) error {
	return r.db.Save(account).Error
}

func (r backupAccountRepo) Delete(id uint) error {
	return r.db.Model(&biz.BackupStorage{}).Where("id = ?", id).Delete(&biz.BackupStorage{}).Error
}

// Check 实际连一次存储器，凭据或路径不对在保存前就能发现
func (r backupAccountRepo) Check(ctx context.Context, account *biz.BackupStorage) error {
	client, err := newStorage(r.t, account)
	if err == nil {
		_, err = client.List(ctx, "")
	}
	if err != nil {
		return errors.New(r.t.Get("storage connection error: %v", err))
	}
	return nil
}

func newStorage(t *gotext.Locale, account *biz.BackupStorage) (storage.Storage, error) {
	switch account.Type {
	case biz.BackupStorageTypeLocal:
		return storage.NewLocal(account.Info.Path)
	case biz.BackupStorageTypeS3:
		return storage.NewS3(storage.S3Config{
			Region:          account.Info.Region,
			Bucket:          account.Info.Bucket,
			AccessKey:       account.Info.AccessKey,
			SecretKey:       account.Info.SecretKey,
			Endpoint:        account.Info.Endpoint,
			Scheme:          account.Info.Scheme,
			BasePath:        account.Info.Path,
			AddressingStyle: storage.S3AddressingStyle(account.Info.Style),
		}), nil
	case biz.BackupStorageTypeSFTP:
		return storage.NewSFTP(storage.SFTPConfig{
			Host:       account.Info.Host,
			Port:       account.Info.Port,
			Username:   account.Info.Username,
			Password:   account.Info.Password,
			PrivateKey: account.Info.PrivateKey,
			BasePath:   account.Info.Path,
		})
	case biz.BackupStorageTypeWebDAV:
		return storage.NewWebDav(storage.WebDavConfig{
			URL:      account.Info.URL,
			Username: account.Info.Username,
			Password: account.Info.Password,
			BasePath: account.Info.Path,
		})
	default:
		return nil, errors.New(t.Get("unknown storage type"))
	}
}
