package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/types"
)

type Project struct {
	ID        uint              `gorm:"primaryKey" json:"id"`
	Name      string            `gorm:"not null;unique" json:"name"`                  // 项目名称
	Type      types.ProjectType `gorm:"not null;index;default:'general'" json:"type"` // 项目类型
	Path      string            `gorm:"not null;default:''" json:"path"`              // 项目路径
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type ProjectRepo interface {
	List(typ types.ProjectType, page, limit uint) ([]*types.ProjectDetail, int64, error)
	Get(id uint) (*types.ProjectDetail, error)
	Create(ctx context.Context, req *request.ProjectCreate) (*types.ProjectDetail, error)
	Update(ctx context.Context, req *request.ProjectUpdate) error
	Delete(ctx context.Context, id uint) error
}
