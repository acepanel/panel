package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
)

type UserPasskey struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"index;not null" json:"user_id"`
	Name           string     `gorm:"not null;default:''" json:"name"`
	CredentialID   []byte     `gorm:"uniqueIndex;not null" json:"-"`
	PublicKey      []byte     `gorm:"not null" json:"-"`
	AAGUID         []byte     `json:"-"`
	SignCount      uint32     `gorm:"not null;default:0" json:"-"`
	Transports     string     `gorm:"not null;default:''" json:"transports"` // JSON: ["internal","hybrid"]
	BackupEligible bool       `gorm:"not null;default:false" json:"-"`
	BackupState    bool       `gorm:"not null;default:false" json:"-"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type UserPasskeyRepo interface {
	List(userID uint) ([]*UserPasskey, error)
	Get(userID, id uint) (*UserPasskey, error)
	Create(passkey *UserPasskey) error
	UpdateSignCount(credentialID []byte, signCount uint32) error
	UpdateLastUsed(credentialID []byte) error
	Delete(userID, id uint) error
	GetByCredentialID(credentialID []byte) (*UserPasskey, *User, error)
	HasPasskey(userID uint) (bool, error)
	HasAny() (bool, error)
	DeleteAllByUserID(userID uint) error
}

type UserPasskeyUsecase struct {
	repo UserPasskeyRepo
	log  *slog.Logger
	t    *gotext.Locale
}

func NewUserPasskeyUsecase(t *gotext.Locale, log *slog.Logger, repo UserPasskeyRepo) *UserPasskeyUsecase {
	return &UserPasskeyUsecase{
		repo: repo,
		log:  log,
		t:    t,
	}
}

func (uc *UserPasskeyUsecase) List(userID uint) ([]*UserPasskey, error) {
	return uc.repo.List(userID)
}

func (uc *UserPasskeyUsecase) Create(ctx context.Context, passkey *UserPasskey) error {
	if err := uc.repo.Create(passkey); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("user passkey created"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(passkey.ID)), slog.String("name", passkey.Name), slog.Uint64("user_id", uint64(passkey.UserID)))

	return nil
}

func (uc *UserPasskeyUsecase) UpdateSignCount(credentialID []byte, signCount uint32) error {
	return uc.repo.UpdateSignCount(credentialID, signCount)
}

func (uc *UserPasskeyUsecase) UpdateLastUsed(credentialID []byte) error {
	return uc.repo.UpdateLastUsed(credentialID)
}

func (uc *UserPasskeyUsecase) Delete(ctx context.Context, userID, id uint) error {
	passkey, err := uc.repo.Get(userID, id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(userID, id); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("user passkey deleted"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", passkey.Name), slog.Uint64("user_id", uint64(userID)))

	return nil
}

func (uc *UserPasskeyUsecase) GetByCredentialID(credentialID []byte) (*UserPasskey, *User, error) {
	return uc.repo.GetByCredentialID(credentialID)
}

func (uc *UserPasskeyUsecase) HasPasskey(userID uint) (bool, error) {
	return uc.repo.HasPasskey(userID)
}

func (uc *UserPasskeyUsecase) HasAny() (bool, error) {
	return uc.repo.HasAny()
}

func (uc *UserPasskeyUsecase) DeleteAllByUserID(userID uint) error {
	return uc.repo.DeleteAllByUserID(userID)
}
