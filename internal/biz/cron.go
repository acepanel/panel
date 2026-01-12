package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
)

type Cron struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null;default:'';unique" json:"name"`
	Status    bool      `gorm:"not null;default:false" json:"status"`
	Type      string    `gorm:"not null;default:''" json:"type"`
	Time      string    `gorm:"not null;default:''" json:"time"`
	Shell     string    `gorm:"not null;default:''" json:"shell"`
	Log       string    `gorm:"not null;default:''" json:"log"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CronRepo interface {
	Count() (int64, error)
	List(page, limit uint) ([]*Cron, int64, error)
	Get(id uint) (*Cron, error)
	Create(ctx context.Context, req *request.CronCreate) error
	Update(ctx context.Context, req *request.CronUpdate) error
	Delete(ctx context.Context, id uint) error
	Status(id uint, status bool) error
}
