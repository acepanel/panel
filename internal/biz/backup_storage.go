package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type BackupStorageType string

const (
	BackupStorageTypeLocal  BackupStorageType = "local"
	BackupStorageTypeS3     BackupStorageType = "s3"
	BackupStorageTypeSFTP   BackupStorageType = "sftp"
	BackupStorageTypeWebDAV BackupStorageType = "webdav"
)

type BackupStorage struct {
	ID        uint                    `gorm:"primaryKey" json:"id"`
	Type      BackupStorageType       `gorm:"not null;default:''" json:"type"`
	Name      string                  `gorm:"not null;default:''" json:"name"`
	Info      types.BackupStorageInfo `gorm:"not null;default:'{}';serializer:json" json:"info"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

func (r *BackupStorage) BeforeSave(tx *gorm.DB) error {
	return encryptFields(r.secrets()...)
}

func (r *BackupStorage) AfterFind(tx *gorm.DB) error {
	return decryptFields(r.secrets()...)
}

// secrets 返回当前存储类型下加密落库的字段
func (r *BackupStorage) secrets() []*string {
	switch r.Type {
	case BackupStorageTypeS3:
		return []*string{&r.Info.AccessKey, &r.Info.SecretKey}
	case BackupStorageTypeSFTP:
		// 密码和私钥留空的那个保持空串，不加密
		fields := []*string{&r.Info.Username}
		if r.Info.Password != "" {
			fields = append(fields, &r.Info.Password)
		}
		if r.Info.PrivateKey != "" {
			fields = append(fields, &r.Info.PrivateKey)
		}
		return fields
	case BackupStorageTypeWebDAV:
		return []*string{&r.Info.Username, &r.Info.Password}
	default:
		return nil
	}
}

type BackupAccountRepo interface {
	ListPaged(page, limit uint) ([]*BackupStorage, int64, error)
	GetByID(id uint) (*BackupStorage, error)
	Create(account *BackupStorage) error
	Update(account *BackupStorage) error
	Delete(id uint) error
	Check(ctx context.Context, account *BackupStorage) error
}

type BackupAccountUsecase struct {
	repo    BackupAccountRepo
	setting SettingRepo
	t       *gotext.Locale
	log     *slog.Logger
}

func NewBackupAccountUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	backupAccountRepo BackupAccountRepo,
	settingRepo SettingRepo,
) *BackupAccountUsecase {
	return &BackupAccountUsecase{
		repo:    backupAccountRepo,
		setting: settingRepo,
		t:       t,
		log:     log,
	}
}

func (uc *BackupAccountUsecase) List(page, limit uint) ([]*BackupStorage, int64, error) {
	// 本地存储
	localStorage, err := uc.Get(0)
	if err != nil {
		return nil, 0, err
	}

	dbAccounts, total, err := uc.repo.ListPaged(page, limit)
	if err != nil {
		return nil, 0, err
	}

	accounts := make([]*BackupStorage, 0, len(dbAccounts)+1)
	if page == 1 {
		accounts = append(accounts, localStorage)
	}
	accounts = append(accounts, dbAccounts...)

	return accounts, total + 1, nil
}

func (uc *BackupAccountUsecase) Get(id uint) (*BackupStorage, error) {
	if id == 0 {
		path, err := uc.setting.Get(SettingKeyBackupPath)
		if err != nil {
			return nil, err
		}
		return &BackupStorage{
			ID:   0,
			Type: BackupStorageTypeLocal,
			Name: uc.t.Get("Local Storage"),
			Info: types.BackupStorageInfo{
				Path: path,
			},
		}, nil
	}

	return uc.repo.GetByID(id)
}

func (uc *BackupAccountUsecase) Create(ctx context.Context, req *request.BackupStorageCreate) (*BackupStorage, error) {
	account := &BackupStorage{
		Type: BackupStorageType(req.Type),
		Name: req.Name,
		Info: req.Info,
	}

	if err := uc.repo.Check(ctx, account); err != nil {
		return nil, err
	}
	if err := uc.repo.Create(account); err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("backup storage created"), slog.String("type", OperationTypeBackup), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(account.ID)), slog.String("account_type", req.Type), slog.String("name", req.Name))

	// 落库时凭据已被加密，重新读取以返回解密后的实体
	return uc.repo.GetByID(account.ID)
}

func (uc *BackupAccountUsecase) Update(ctx context.Context, req *request.BackupStorageUpdate) error {
	account, err := uc.Get(req.ID)
	if err != nil {
		return err
	}

	oldName := account.Name
	account.Type = BackupStorageType(req.Type)
	account.Name = req.Name
	account.Info = req.Info

	if err = uc.repo.Check(ctx, account); err != nil {
		return err
	}
	if err = uc.repo.Update(account); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("backup storage updated"), slog.String("type", OperationTypeBackup), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", req.Name), slog.String("old_name", oldName), slog.String("account_type", req.Type))

	return nil
}

func (uc *BackupAccountUsecase) Delete(ctx context.Context, id uint) error {
	account, err := uc.repo.GetByID(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("backup storage deleted"), slog.String("type", OperationTypeBackup), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", account.Name), slog.String("account_type", string(account.Type)))

	return nil
}
