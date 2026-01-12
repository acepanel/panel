package data

import (
	"context"
	"errors"
	"image"
	"log/slog"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/hash"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
)

type userRepo struct {
	t      *gotext.Locale
	db     *gorm.DB
	log    *slog.Logger
	hasher hash.Hasher
}

func NewUserRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.UserRepo {
	return &userRepo{
		t:      t,
		db:     db,
		log:    log,
		hasher: hash.NewArgon2id(),
	}
}

func (r *userRepo) List(page, limit uint) ([]*biz.User, int64, error) {
	users := make([]*biz.User, 0)
	var total int64
	err := r.db.Model(&biz.User{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&users).Error
	return users, total, err
}

func (r *userRepo) Get(id uint) (*biz.User, error) {
	user := new(biz.User)
	if err := r.db.First(user, id).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepo) Create(ctx context.Context, username, password, email string) (*biz.User, error) {
	value, err := r.hasher.Make(password)
	if err != nil {
		return nil, err
	}

	user := &biz.User{
		Username: username,
		Password: value,
		Email:    email,
	}
	if err = r.db.Create(user).Error; err != nil {
		return nil, err
	}

	// 记录日志
	r.log.Info("user created", slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(user.ID)), slog.String("username", username))

	return user, nil
}

func (r *userRepo) UpdateUsername(ctx context.Context, id uint, username string) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	user.Username = username
	if err = r.db.Save(user).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("user username updated", slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", username))

	return nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, id uint, password string) error {
	value, err := r.hasher.Make(password)
	if err != nil {
		return err
	}

	user, err := r.Get(id)
	if err != nil {
		return err
	}

	user.Password = value
	if err = r.db.Save(user).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("user password updated", slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)))

	return nil
}

func (r *userRepo) UpdateEmail(ctx context.Context, id uint, email string) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	user.Email = email
	if err = r.db.Save(user).Error; err != nil {
		return err
	}

	// 记录日志
	r.log.Info("user email updated", slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("email", email))

	return nil
}

func (r *userRepo) Delete(ctx context.Context, id uint) error {
	var count int64
	if err := r.db.Model(&biz.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count <= 1 {
		return errors.New(r.t.Get("please don't do this"))
	}

	user := new(biz.User)
	if err := r.db.Preload("Tokens").First(user, id).Error; err != nil {
		return err
	}

	username := user.Username
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Association("Tokens").Delete(); err != nil {
			return err
		}
		return tx.Delete(&user).Error
	}); err != nil {
		return err
	}

	// 记录日志
	r.log.Info("user deleted", slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", getOperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", username))

	return nil
}

func (r *userRepo) CheckPassword(username, password string) (*biz.User, error) {
	user := new(biz.User)
	if err := r.db.Where("username = ?", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(r.t.Get("username or password error"))
		} else {
			return nil, err
		}
	}

	if !r.hasher.Check(password, user.Password) {
		return nil, errors.New(r.t.Get("username or password error"))
	}

	return user, nil
}

func (r *userRepo) IsTwoFA(username string) (bool, error) {
	user := new(biz.User)
	if err := r.db.Where("username = ?", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errors.New(r.t.Get("username or password error"))
		} else {
			return false, err
		}
	}

	return user.TwoFA != "", nil
}

func (r *userRepo) GenerateTwoFA(id uint) (image.Image, string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "AcePanel",
		AccountName: cast.ToString(id),
		SecretSize:  32,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, "", "", err
	}

	img, err := key.Image(200, 200)
	if err != nil {
		return nil, "", "", err
	}

	return img, key.URL(), key.Secret(), nil
}

func (r *userRepo) UpdateTwoFA(id uint, code, secret string) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	// 保存前先验证一次，防止错误开启
	if secret != "" {
		if valid := totp.Validate(code, secret); !valid {
			return errors.New(r.t.Get("invalid 2FA code"))
		}
	}

	user.TwoFA = secret
	return r.db.Save(user).Error
}

func (r *userRepo) CheckTwoFA(id uint, code string) (bool, error) {
	user, err := r.Get(id)
	if err != nil {
		return false, err
	}

	if user.TwoFA == "" {
		return true, nil // 未开启2FA，无需验证
	}

	if valid := totp.Validate(code, user.TwoFA); !valid {
		return false, errors.New(r.t.Get("invalid 2FA code"))
	}

	return true, nil
}
