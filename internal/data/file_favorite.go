package data

import (
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

type fileFavoriteRepo struct {
	db *gorm.DB
}

func NewFileFavoriteRepo(db *gorm.DB) biz.FileFavoriteRepo {
	return &fileFavoriteRepo{db: db}
}

func (r *fileFavoriteRepo) List() ([]*biz.FileFavorite, error) {
	favorites := make([]*biz.FileFavorite, 0)
	err := r.db.Order("sort asc, id asc").Find(&favorites).Error
	return favorites, err
}

func (r *fileFavoriteRepo) Get(id uint) (*biz.FileFavorite, error) {
	favorite := new(biz.FileFavorite)
	if err := r.db.First(favorite, id).Error; err != nil {
		return nil, err
	}
	return favorite, nil
}

// Create 新收藏追加到末尾
func (r *fileFavoriteRepo) Create(path, remark string) (*biz.FileFavorite, error) {
	var maxSort int
	if err := r.db.Model(&biz.FileFavorite{}).Select("COALESCE(MAX(sort), -1)").Scan(&maxSort).Error; err != nil {
		return nil, err
	}

	favorite := &biz.FileFavorite{Path: path, Remark: remark, Sort: maxSort + 1}
	if err := r.db.Create(favorite).Error; err != nil {
		return nil, err
	}
	return favorite, nil
}

func (r *fileFavoriteRepo) UpdateRemark(id uint, remark string) error {
	return r.db.Model(&biz.FileFavorite{}).Where("id = ?", id).Update("remark", remark).Error
}

func (r *fileFavoriteRepo) Delete(id uint) error {
	return r.db.Delete(&biz.FileFavorite{}, id).Error
}

func (r *fileFavoriteRepo) UpdateOrder(ids []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&biz.FileFavorite{}).Where("id = ?", id).Update("sort", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
