package biz

import (
	"time"
)

type UserSession struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	Token        string    `gorm:"not null;uniqueIndex" json:"-"`         // 令牌的 SHA-256，原值只在 Cookie 里
	IP           string    `gorm:"not null;default:''" json:"ip"`         // 登录时的 IP
	UserAgent    string    `gorm:"not null;default:''" json:"user_agent"` // 登录时的 UA
	Safe         bool      `gorm:"not null;default:false" json:"safe"`    // 安全登录，只接受登录时的 IP
	LastActiveAt time.Time `gorm:"not null;index" json:"last_active_at"`  // 超过登录超时未活跃即过期
	CreatedAt    time.Time `json:"created_at"`
}

type UserSessionRepo interface {
	Create(userID uint, ip, userAgent string, safe bool) (string, error)
	Authenticate(token, ip string) (*UserSession, error)
	Delete(token string) error
	ClearExpired() (int64, error)
}

type UserSessionUsecase struct {
	repo UserSessionRepo
}

func NewUserSessionUsecase(repo UserSessionRepo) *UserSessionUsecase {
	return &UserSessionUsecase{
		repo: repo,
	}
}

// Create 为登录成功的用户创建会话，返回写进 Cookie 的令牌
func (uc *UserSessionUsecase) Create(userID uint, ip, userAgent string, safe bool) (string, error) {
	return uc.repo.Create(userID, ip, userAgent, safe)
}

// Authenticate 校验令牌并刷新活跃时间，过期或来源 IP 不符的会话视为无效
func (uc *UserSessionUsecase) Authenticate(token, ip string) (*UserSession, error) {
	return uc.repo.Authenticate(token, ip)
}

func (uc *UserSessionUsecase) Delete(token string) error {
	return uc.repo.Delete(token)
}

func (uc *UserSessionUsecase) ClearExpired() (int64, error) {
	return uc.repo.ClearExpired()
}
