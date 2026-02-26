package data

import (
	"time"

	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

type userPasskeyRepo struct {
	db *gorm.DB
}

func NewUserPasskeyRepo(db *gorm.DB) biz.UserPasskeyRepo {
	return &userPasskeyRepo{db: db}
}

func (r userPasskeyRepo) List(userID uint) ([]*biz.UserPasskey, error) {
	var passkeys []*biz.UserPasskey
	if err := r.db.Where("user_id = ?", userID).Order("id desc").Find(&passkeys).Error; err != nil {
		return nil, err
	}
	return passkeys, nil
}

func (r userPasskeyRepo) Create(passkey *biz.UserPasskey) error {
	return r.db.Create(passkey).Error
}

func (r userPasskeyRepo) UpdateSignCount(credentialID []byte, signCount uint32) error {
	return r.db.Model(&biz.UserPasskey{}).Where("credential_id = ?", credentialID).Update("sign_count", signCount).Error
}

func (r userPasskeyRepo) UpdateLastUsed(credentialID []byte) error {
	now := time.Now()
	return r.db.Model(&biz.UserPasskey{}).Where("credential_id = ?", credentialID).Update("last_used_at", &now).Error
}

func (r userPasskeyRepo) Delete(userID, id uint) error {
	return r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&biz.UserPasskey{}).Error
}

func (r userPasskeyRepo) GetByCredentialID(credentialID []byte) (*biz.UserPasskey, *biz.User, error) {
	passkey := new(biz.UserPasskey)
	if err := r.db.Where("credential_id = ?", credentialID).First(passkey).Error; err != nil {
		return nil, nil, err
	}

	user := new(biz.User)
	if err := r.db.First(user, passkey.UserID).Error; err != nil {
		return nil, nil, err
	}

	return passkey, user, nil
}

func (r userPasskeyRepo) HasPasskey(userID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&biz.UserPasskey{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r userPasskeyRepo) HasAny() (bool, error) {
	var count int64
	if err := r.db.Model(&biz.UserPasskey{}).Limit(1).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r userPasskeyRepo) DeleteAllByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&biz.UserPasskey{}).Error
}
