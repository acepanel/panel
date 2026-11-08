package biz

import (
	"context"
	"errors"
	"image"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"not null;default:'';unique" json:"username"`
	Password  string    `gorm:"not null;default:''" json:"password"`
	Email     string    `gorm:"not null;default:''" json:"email"`
	TwoFA     string    `gorm:"not null;default:''" json:"two_fa"` // 2FA secret，为空表示未开启
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserRepo interface {
	List(page, limit uint) ([]*User, int64, error)
	Get(id uint) (*User, error)
	Count() (int64, error)
	Create(username, password, email string) (*User, error)
	UpdateUsername(id uint, username string) error
	UpdatePassword(id uint, password string) error
	UpdateEmail(id uint, email string) error
	Delete(id uint) (string, error)
	CheckPassword(username, password string) (*User, error)
	IsTwoFA(username string) (bool, error)
	GenerateTwoFA(id uint) (image.Image, string, string, error)
	UpdateTwoFA(id uint, code, secret string) error
}

type UserUsecase struct {
	repo UserRepo
	log  *slog.Logger
	t    *gotext.Locale
}

func NewUserUsecase(t *gotext.Locale, log *slog.Logger, userRepo UserRepo) *UserUsecase {
	return &UserUsecase{
		repo: userRepo,
		log:  log,
		t:    t,
	}
}

func (uc *UserUsecase) List(ctx context.Context, page, limit uint) ([]*User, int64, error) {
	users, total, err := uc.repo.List(page, limit)
	if err != nil {
		return nil, 0, err
	}

	uc.log.Info(uc.t.Get("user list viewed"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("page", uint64(page)), slog.Uint64("limit", uint64(limit)))

	return users, total, nil
}

func (uc *UserUsecase) Get(id uint) (*User, error) {
	return uc.repo.Get(id)
}

func (uc *UserUsecase) Create(ctx context.Context, username, password, email string) (*User, error) {
	user, err := uc.repo.Create(username, password, email)
	if err != nil {
		return nil, err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("user created"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(user.ID)), slog.String("username", username))

	return user, nil
}

func (uc *UserUsecase) UpdateUsername(ctx context.Context, id uint, username string) error {
	user, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdateUsername(id, username); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("user username updated"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("old_username", user.Username), slog.String("username", username))

	return nil
}

func (uc *UserUsecase) UpdatePassword(ctx context.Context, id uint, password string) error {
	user, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdatePassword(id, password); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("user password updated"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", user.Username))

	return nil
}

func (uc *UserUsecase) UpdateEmail(ctx context.Context, id uint, email string) error {
	user, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdateEmail(id, email); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("user email updated"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", user.Username), slog.String("old_email", user.Email), slog.String("email", email))

	return nil
}

func (uc *UserUsecase) Delete(ctx context.Context, id uint) error {
	count, err := uc.repo.Count()
	if err != nil {
		return err
	}
	if count <= 1 {
		return errors.New(uc.t.Get("please don't do this"))
	}

	username, err := uc.repo.Delete(id)
	if err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("user deleted"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", username))

	return nil
}

func (uc *UserUsecase) CheckPassword(username, password string) (*User, error) {
	return uc.repo.CheckPassword(username, password)
}

func (uc *UserUsecase) IsTwoFA(username string) (bool, error) {
	return uc.repo.IsTwoFA(username)
}

func (uc *UserUsecase) GenerateTwoFA(id uint) (image.Image, string, string, error) {
	return uc.repo.GenerateTwoFA(id)
}

func (uc *UserUsecase) UpdateTwoFA(ctx context.Context, id uint, code, secret string) error {
	user, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdateTwoFA(id, code, secret); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("user 2fa updated"), slog.String("type", OperationTypeUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", user.Username), slog.Bool("enabled", secret != ""))

	return nil
}
