package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
)

type WebHook struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"not null;default:''" json:"name"`      // 钩子名称
	Key        string    `gorm:"not null;uniqueIndex" json:"key"`      // 唯一标识（用于 URL）
	Script     string    `gorm:"not null;default:''" json:"script"`    // 脚本内容
	Raw        bool      `gorm:"not null;default:false" json:"raw"`    // 是否以原始格式返回输出
	User       string    `gorm:"not null;default:''" json:"user"`      // 以哪个用户身份执行脚本
	Status     bool      `gorm:"not null;default:true" json:"status"`  // 启用状态
	CallCount  uint      `gorm:"not null;default:0" json:"call_count"` // 调用次数
	LastCallAt time.Time `json:"last_call_at"`                         // 上次调用时间
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type WebHookRepo interface {
	List(page, limit uint) ([]*WebHook, int64, error)
	Get(id uint) (*WebHook, error)
	GetByKey(key string) (*WebHook, error)
	Create(ctx context.Context, req *request.WebHookCreate) (*WebHook, error)
	Update(ctx context.Context, req *request.WebHookUpdate) error
	Delete(ctx context.Context, id uint) error
	Call(key string) (string, error)
}
