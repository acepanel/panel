package data

import (
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

func newUserSessionRepoForTest(t *testing.T) *userSessionRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	must.NoError(t, db.AutoMigrate(&biz.UserSession{}))

	conf := &config.Config{}
	conf.Session.Lifetime = 120
	return &userSessionRepo{t: gotext.NewLocale("", "en"), conf: conf, db: db}
}

// lastActive 把会话的活跃时间拨到 ago 之前
func (r *userSessionRepo) lastActive(t *testing.T, ago time.Duration) {
	t.Helper()
	must.NoError(t, r.db.Model(&biz.UserSession{}).Where("1 = 1").Update("last_active_at", time.Now().Add(-ago)).Error)
}

func TestUserSessionAuthenticate(t *testing.T) {
	repo := newUserSessionRepoForTest(t)
	token, err := repo.Create(7, "203.0.113.7", "curl", false)
	must.NoError(t, err)

	session, err := repo.Authenticate(token, "198.51.100.9")
	must.NoError(t, err)
	check.Equal(t, session.UserID, uint(7))
	check.Equal(t, session.IP, "203.0.113.7")
	check.Equal(t, session.UserAgent, "curl")

	_, err = repo.Authenticate("", "203.0.113.7")
	check.Error(t, err)
	_, err = repo.Authenticate(token+"x", "203.0.113.7")
	check.Error(t, err)
}

func TestUserSessionStoresOnlyTokenHash(t *testing.T) {
	repo := newUserSessionRepoForTest(t)
	token, err := repo.Create(7, "203.0.113.7", "curl", false)
	must.NoError(t, err)

	var stored string
	must.NoError(t, repo.db.Model(&biz.UserSession{}).Pluck("token", &stored).Error)
	check.NotEqual(t, stored, token)

	// 库里的值不能当令牌用
	_, err = repo.Authenticate(stored, "203.0.113.7")
	check.Error(t, err)
}

func TestUserSessionExpires(t *testing.T) {
	repo := newUserSessionRepoForTest(t)
	token, err := repo.Create(7, "203.0.113.7", "curl", false)
	must.NoError(t, err)

	repo.lastActive(t, 119*time.Minute)
	_, err = repo.Authenticate(token, "203.0.113.7")
	must.NoError(t, err)

	// 上一次校验刷新了活跃时间，会话从那时起重新计时
	var session biz.UserSession
	must.NoError(t, repo.db.First(&session).Error)
	check.True(t, time.Since(session.LastActiveAt) < time.Minute)

	repo.lastActive(t, 121*time.Minute)
	_, err = repo.Authenticate(token, "203.0.113.7")
	check.Error(t, err)
}

func TestUserSessionSafeLoginBindsIP(t *testing.T) {
	repo := newUserSessionRepoForTest(t)
	token, err := repo.Create(7, "203.0.113.7", "curl", true)
	must.NoError(t, err)

	_, err = repo.Authenticate(token, "203.0.113.7")
	must.NoError(t, err)

	_, err = repo.Authenticate(token, "198.51.100.9")
	check.Error(t, err)

	// 换过 IP 的会话已被删除，回到原 IP 也不能再用
	_, err = repo.Authenticate(token, "203.0.113.7")
	check.Error(t, err)
}

func TestUserSessionDelete(t *testing.T) {
	repo := newUserSessionRepoForTest(t)
	token, err := repo.Create(7, "203.0.113.7", "curl", false)
	must.NoError(t, err)
	other, err := repo.Create(7, "203.0.113.7", "curl", false)
	must.NoError(t, err)

	must.NoError(t, repo.Delete(token))
	_, err = repo.Authenticate(token, "203.0.113.7")
	check.Error(t, err)
	_, err = repo.Authenticate(other, "203.0.113.7")
	check.NoError(t, err)
}

func TestUserSessionClearExpired(t *testing.T) {
	repo := newUserSessionRepoForTest(t)
	_, err := repo.Create(7, "203.0.113.7", "curl", false)
	must.NoError(t, err)
	repo.lastActive(t, 121*time.Minute)
	alive, err := repo.Create(8, "203.0.113.8", "curl", false)
	must.NoError(t, err)

	count, err := repo.ClearExpired()
	must.NoError(t, err)
	check.Equal(t, count, int64(1))
	_, err = repo.Authenticate(alive, "203.0.113.8")
	check.NoError(t, err)
}
