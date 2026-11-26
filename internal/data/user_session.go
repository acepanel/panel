package data

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

type userSessionRepo struct {
	t    *gotext.Locale
	conf *config.Config
	db   *gorm.DB
}

func NewUserSessionRepo(conf *config.Config, db *gorm.DB, t *gotext.Locale) biz.UserSessionRepo {
	return &userSessionRepo{
		t:    t,
		conf: conf,
		db:   db,
	}
}

func (r *userSessionRepo) Create(userID uint, ip, userAgent string, safe bool) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)

	session := &biz.UserSession{
		UserID:       userID,
		Token:        r.hash(token),
		IP:           ip,
		UserAgent:    userAgent,
		Safe:         safe,
		LastActiveAt: time.Now(),
	}
	if err := r.db.Create(session).Error; err != nil {
		return "", err
	}

	return token, nil
}

func (r *userSessionRepo) Authenticate(token, ip string) (*biz.UserSession, error) {
	session := new(biz.UserSession)
	if err := r.db.Where("token = ? AND last_active_at > ?", r.hash(token), r.deadline()).First(session).Error; err != nil {
		return nil, errors.New(r.t.Get("session expired, please login again"))
	}

	if session.Safe && session.IP != ip {
		_ = r.db.Delete(session).Error
		return nil, errors.New(r.t.Get("client ip changed, please login again"))
	}

	// 活跃时间每分钟最多落库一次，不必每个请求都写库
	if now := time.Now(); now.Sub(session.LastActiveAt) > time.Minute {
		if err := r.db.Model(session).Update("last_active_at", now).Error; err != nil {
			return nil, err
		}
	}

	return session, nil
}

func (r *userSessionRepo) Delete(token string) error {
	return r.db.Where("token = ?", r.hash(token)).Delete(&biz.UserSession{}).Error
}

func (r *userSessionRepo) ClearExpired() (int64, error) {
	result := r.db.Where("last_active_at <= ?", r.deadline()).Delete(&biz.UserSession{})
	return result.RowsAffected, result.Error
}

// deadline 返回仍然有效的最早活跃时间
func (r *userSessionRepo) deadline() time.Time {
	return time.Now().Add(-r.conf.Session.TTL())
}

// hash 库里只存令牌的哈希，数据库或备份泄露后拿不到可用的令牌
func (r *userSessionRepo) hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
