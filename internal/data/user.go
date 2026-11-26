package data

import (
	"errors"
	"image"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/hash"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

type userRepo struct {
	t      *gotext.Locale
	db     *gorm.DB
	hasher hash.Hasher
}

func NewUserRepo(db *gorm.DB, t *gotext.Locale) biz.UserRepo {
	return &userRepo{
		t:      t,
		db:     db,
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

func (r *userRepo) GetByUsername(username string) (*biz.User, error) {
	user := new(biz.User)
	if err := r.db.Where("username = ?", username).First(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepo) First() (*biz.User, error) {
	user := new(biz.User)
	if err := r.db.First(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *userRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userRepo) Create(username, password, email string) (*biz.User, error) {
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

	return user, nil
}

func (r *userRepo) UpdateUsername(id uint, username string) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	user.Username = username
	if err = r.db.Save(user).Error; err != nil {
		return err
	}

	return nil
}

func (r *userRepo) UpdatePassword(id uint, password string) error {
	value, err := r.hasher.Make(password)
	if err != nil {
		return err
	}

	user, err := r.Get(id)
	if err != nil {
		return err
	}

	// 改密码后该用户的全部会话随之失效
	user.Password = value
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&biz.UserSession{}).Error; err != nil {
			return err
		}
		return tx.Save(user).Error
	})
}

func (r *userRepo) UpdateEmail(id uint, email string) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	user.Email = email
	if err = r.db.Save(user).Error; err != nil {
		return err
	}

	return nil
}

func (r *userRepo) Delete(id uint) (string, error) {
	user, err := r.Get(id)
	if err != nil {
		return "", err
	}

	if err = r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&biz.UserToken{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&biz.UserPasskey{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&biz.UserSession{}).Error; err != nil {
			return err
		}
		return tx.Delete(user).Error
	}); err != nil {
		return "", err
	}

	return user.Username, nil
}

func (r *userRepo) CheckPassword(username, password string) (*biz.User, error) {
	user, err := r.GetByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(r.t.Get("username or password error"))
		}
		return nil, err
	}

	if !r.hasher.Check(password, user.Password) {
		return nil, errors.New(r.t.Get("username or password error"))
	}

	return user, nil
}

func (r *userRepo) IsTwoFA(username string) (bool, error) {
	user, err := r.GetByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errors.New(r.t.Get("username or password error"))
		}
		return false, err
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
