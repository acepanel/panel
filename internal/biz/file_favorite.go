package biz

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
)

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
	Get(id uint) (*FileFavorite, error)
	Create(path, remark string) (*FileFavorite, error)
	UpdateRemark(id uint, remark string) error
	Delete(id uint) error
	UpdateOrder(ids []uint) error
}

type FileFavoriteUsecase struct {
	t    *gotext.Locale
	log  *slog.Logger
	repo FileFavoriteRepo
}

func NewFileFavoriteUsecase(t *gotext.Locale, log *slog.Logger, repo FileFavoriteRepo) *FileFavoriteUsecase {
	return &FileFavoriteUsecase{t: t, log: log, repo: repo}
}

func (uc *FileFavoriteUsecase) List() ([]*FileFavorite, error) {
	return uc.repo.List()
}

func (uc *FileFavoriteUsecase) Create(ctx context.Context, path, remark string) (*FileFavorite, error) {
	favorite, err := uc.repo.Create(path, remark)
	if err != nil {
		return nil, err
	}

	uc.log.Info(uc.t.Get("file favorite created"), slog.String("type", OperationTypeFile), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(favorite.ID)), slog.String("path", favorite.Path), slog.String("remark", favorite.Remark))

	return favorite, nil
}

func (uc *FileFavoriteUsecase) UpdateRemark(ctx context.Context, id uint, remark string) error {
	favorite, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdateRemark(id, remark); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("file favorite updated"), slog.String("type", OperationTypeFile), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("path", favorite.Path), slog.String("remark", remark))

	return nil
}

func (uc *FileFavoriteUsecase) Delete(ctx context.Context, id uint) error {
	favorite, err := uc.repo.Get(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("file favorite deleted"), slog.String("type", OperationTypeFile), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("path", favorite.Path))

	return nil
}

func (uc *FileFavoriteUsecase) UpdateOrder(ids []uint) error {
	return uc.repo.UpdateOrder(ids)
}
