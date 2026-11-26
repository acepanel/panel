package data

import (
	"errors"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/str"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

type fileShareRepo struct {
	t  *gotext.Locale
	db *gorm.DB
}

func NewFileShareRepo(db *gorm.DB, t *gotext.Locale) biz.FileShareRepo {
	return &fileShareRepo{
		t:  t,
		db: db,
	}
}

func (r *fileShareRepo) List() ([]*biz.FileShare, error) {
	shares := make([]*biz.FileShare, 0)
	err := r.db.Order("id desc").Find(&shares).Error
	return shares, err
}

func (r *fileShareRepo) Get(id uint) (*biz.FileShare, error) {
	share := new(biz.FileShare)
	if err := r.db.Where("id = ?", id).First(share).Error; err != nil {
		return nil, err
	}
	return share, nil
}

func (r *fileShareRepo) Create(path string, maxDownloads uint, expiredAt time.Time) (*biz.FileShare, error) {
	share := &biz.FileShare{
		Token:        str.Random(32),
		Path:         path,
		MaxDownloads: maxDownloads,
		ExpiredAt:    expiredAt,
	}
	if err := r.db.Create(share).Error; err != nil {
		return nil, err
	}
	return share, nil
}

func (r *fileShareRepo) Delete(id uint) error {
	return r.db.Delete(&biz.FileShare{}, id).Error
}

func (r *fileShareRepo) Consume(token string, count bool) (*biz.FileShare, error) {
	share := new(biz.FileShare)
	if err := r.db.Where("token = ?", token).First(share).Error; err != nil {
		return nil, biz.ErrFileShareNotFound
	}
	if time.Now().After(share.ExpiredAt) {
		return nil, errors.New(r.t.Get("share link has expired"))
	}

	if !count {
		if share.MaxDownloads > 0 && share.Downloads >= share.MaxDownloads {
			return nil, errors.New(r.t.Get("share link download limit reached"))
		}
		return share, nil
	}

	// 原子计数，条件不满足时零行更新即视为次数用尽
	result := r.db.Model(&biz.FileShare{}).
		Where("token = ? AND (max_downloads = 0 OR downloads < max_downloads)", token).
		Update("downloads", gorm.Expr("downloads + 1"))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New(r.t.Get("share link download limit reached"))
	}

	return share, nil
}

func (r *fileShareRepo) ClearExpired() (int64, error) {
	result := r.db.Where("expired_at <= ?", time.Now()).Delete(&biz.FileShare{})
	return result.RowsAffected, result.Error
}
