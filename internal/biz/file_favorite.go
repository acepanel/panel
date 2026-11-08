package biz

import "time"

type FileFavorite struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"not null;unique" json:"path"`
	Remark    string    `gorm:"not null;default:''" json:"remark"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FileFavoriteRepo interface {
	List() ([]*FileFavorite, error)
	Create(path, remark string) (*FileFavorite, error)
	UpdateRemark(id uint, remark string) error
	Delete(id uint) error
	UpdateOrder(ids []uint) error
}

type FileFavoriteUsecase struct {
	repo FileFavoriteRepo
}

func NewFileFavoriteUsecase(repo FileFavoriteRepo) *FileFavoriteUsecase {
	return &FileFavoriteUsecase{repo: repo}
}

func (uc *FileFavoriteUsecase) List() ([]*FileFavorite, error) {
	return uc.repo.List()
}

func (uc *FileFavoriteUsecase) Create(path, remark string) (*FileFavorite, error) {
	return uc.repo.Create(path, remark)
}

func (uc *FileFavoriteUsecase) UpdateRemark(id uint, remark string) error {
	return uc.repo.UpdateRemark(id, remark)
}

func (uc *FileFavoriteUsecase) Delete(id uint) error {
	return uc.repo.Delete(id)
}

func (uc *FileFavoriteUsecase) UpdateOrder(ids []uint) error {
	return uc.repo.UpdateOrder(ids)
}
