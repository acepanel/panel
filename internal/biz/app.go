package biz

import (
	"time"

	"github.com/acepanel/panel/pkg/api"
	"github.com/acepanel/panel/pkg/types"
)

type App struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Slug      string    `gorm:"not null;default:'';unique" json:"slug"`
	Channel   string    `gorm:"not null;default:''" json:"channel"`
	Version   string    `gorm:"not null;default:''" json:"version"`
	Show      bool      `gorm:"not null;default:false" json:"show"`
	ShowOrder int       `gorm:"not null;default:0" json:"show_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AppRepo interface {
	Categories() []types.LV
	All() api.Apps
	Get(slug string) (*api.App, error)
	UpdateExist(slug string) bool
	Installed() ([]*App, error)
	GetInstalled(slug string) (*App, error)
	GetInstalledAll(query string, cond ...string) ([]*App, error)
	GetHomeShow() ([]map[string]string, error)
	IsInstalled(query string, cond ...any) (bool, error)
	Install(channel, slug string) error
	UnInstall(slug string) error
	Update(slug string) error
	UpdateShow(slug string, show bool) error
	UpdateOrder(slugs []string) error
}
