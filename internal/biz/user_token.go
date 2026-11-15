package biz

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"
)

type UserToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Token     string    `gorm:"not null;default:'';unique" json:"-"`
	IPs       []string  `gorm:"not null;default:'[]';serializer:json" json:"ips"`
	ExpiredAt time.Time `json:"expired_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *UserToken) BeforeSave(tx *gorm.DB) error {
	return encryptFields(&r.Token)
}

func (r *UserToken) AfterFind(tx *gorm.DB) error {
	return decryptFields(&r.Token)
}

type UserTokenRepo interface {
	List(userID, page, limit uint) ([]*UserToken, int64, error)
	Get(id uint) (*UserToken, error)
	Create(userID uint, ips []string, expired time.Time) (*UserToken, error)
	Update(id uint, ips []string, expired time.Time) (*UserToken, error)
	Delete(id uint) error
	ValidateReq(req *http.Request) (uint, error)
}

type UserTokenUsecase struct {
	repo UserTokenRepo
	log  *slog.Logger
	t    *gotext.Locale
}

func NewUserTokenUsecase(t *gotext.Locale, log *slog.Logger, repo UserTokenRepo) *UserTokenUsecase {
	return &UserTokenUsecase{
		repo: repo,
		log:  log,
		t:    t,
	}
}

func (uc *UserTokenUsecase) List(ctx context.Context, userID, page, limit uint) ([]*UserToken, int64, error) {
	userTokens, total, err := uc.repo.List(userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("user token list viewed"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("user_id", uint64(userID)), slog.Uint64("page", uint64(page)), slog.Uint64("limit", uint64(limit)))

	return userTokens, total, nil
}

func (uc *UserTokenUsecase) Create(
	ctx context.Context,
	userID uint,
	ips []string,
	expired time.Time,
) (*UserToken, error) {
	userToken, err := uc.repo.Create(userID, ips, expired)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("user token created"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(userToken.ID)), slog.Uint64("user_id", uint64(userToken.UserID)), slog.Any("ips", userToken.IPs), slog.Time("expired_at", userToken.ExpiredAt))

	return userToken, nil
}

func (uc *UserTokenUsecase) Update(ctx context.Context, id uint, ips []string, expired time.Time) (*UserToken, error) {
	userToken, err := uc.repo.Update(id, ips, expired)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("user token updated"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(userToken.ID)), slog.Uint64("user_id", uint64(userToken.UserID)), slog.Any("ips", userToken.IPs), slog.Time("expired_at", userToken.ExpiredAt))

	return userToken, nil
}

func (uc *UserTokenUsecase) Delete(ctx context.Context, id uint) error {
	userToken, err := uc.repo.Get(id)
	if err != nil {
		return err
	}
	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("user token deleted"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.Uint64("user_id", uint64(userToken.UserID)))

	return nil
}
