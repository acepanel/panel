package data

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

func newUserRepoForTest(t *testing.T) biz.UserRepo {
	t.Helper()
	// 2FA 密钥落库前会用 app.Key 加密
	app.Key = "0123456789abcdef0123456789abcdef"
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	must.NoError(t, db.AutoMigrate(&biz.User{}))
	return NewUserRepo(db, gotext.NewLocale("", "en"))
}

func TestUserLookup(t *testing.T) {
	repo := newUserRepoForTest(t)

	// CLI 靠 gorm.ErrRecordNotFound 区分“用户不存在”与查询失败
	_, err := repo.First()
	check.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = repo.GetByUsername("admin")
	check.ErrorIs(t, err, gorm.ErrRecordNotFound)

	admin, err := repo.Create("admin", "secret-1", "admin@example.com")
	must.NoError(t, err)
	second, err := repo.Create("second", "secret-2", "second@example.com")
	must.NoError(t, err)

	first, err := repo.First()
	must.NoError(t, err)
	check.Equal(t, first.ID, admin.ID)

	got, err := repo.GetByUsername("second")
	must.NoError(t, err)
	check.Equal(t, got.ID, second.ID)
}

// Create 接收明文并自行哈希，调用方预先哈希会让真实密码变成哈希串
func TestUserCreateHashesPlainPassword(t *testing.T) {
	repo := newUserRepoForTest(t)

	_, err := repo.Create("admin", "secret-1", "admin@example.com")
	must.NoError(t, err)

	user, err := repo.CheckPassword("admin", "secret-1")
	must.NoError(t, err)
	check.NotEqual(t, user.Password, "secret-1")

	_, err = repo.CheckPassword("admin", user.Password)
	check.Error(t, err)
}

func TestUserTwoFAEncryptedAtRest(t *testing.T) {
	repo := newUserRepoForTest(t)
	user, err := repo.Create("admin", "secret-1", "admin@example.com")
	must.NoError(t, err)

	_, _, secret, err := repo.GenerateTwoFA(user.ID)
	must.NoError(t, err)
	code, err := totp.GenerateCode(secret, time.Now())
	must.NoError(t, err)
	must.NoError(t, repo.UpdateTwoFA(user.ID, code, secret))

	impl, ok := repo.(*userRepo)
	must.True(t, ok)
	var stored string
	must.NoError(t, impl.db.Table("users").Where("id = ?", user.ID).Pluck("two_fa", &stored).Error)
	check.True(t, stored != "" && stored != secret)

	got, err := repo.Get(user.ID)
	must.NoError(t, err)
	check.Equal(t, got.TwoFA, secret)
	check.True(t, got.TwoFAEnabled)

	// 关闭后读回为空
	must.NoError(t, repo.UpdateTwoFA(user.ID, "", ""))
	got, err = repo.Get(user.ID)
	must.NoError(t, err)
	check.Equal(t, got.TwoFA, "")
	check.False(t, got.TwoFAEnabled)
}

// 加密上线前落库的明文密钥仍然可用
func TestUserTwoFALegacyPlaintext(t *testing.T) {
	repo := newUserRepoForTest(t)
	user, err := repo.Create("admin", "secret-1", "admin@example.com")
	must.NoError(t, err)
	impl, ok := repo.(*userRepo)
	must.True(t, ok)
	must.NoError(t, impl.db.Table("users").Where("id = ?", user.ID).Update("two_fa", "LEGACYSECRET").Error)

	got, err := repo.Get(user.ID)
	must.NoError(t, err)
	check.Equal(t, got.TwoFA, "LEGACYSECRET")
	check.True(t, got.TwoFAEnabled)
}

func TestUserJSONHidesSecrets(t *testing.T) {
	repo := newUserRepoForTest(t)
	user, err := repo.Create("admin", "secret-1", "admin@example.com")
	must.NoError(t, err)

	got, err := repo.Get(user.ID)
	must.NoError(t, err)
	raw, err := json.Marshal(got)
	must.NoError(t, err)

	var fields map[string]any
	must.NoError(t, json.Unmarshal(raw, &fields))
	_, hasPassword := fields["password"]
	check.False(t, hasPassword)
	check.Equal(t, fields["two_fa"], any(false))
	check.False(t, strings.Contains(string(raw), got.Password))
}

// 会话指纹只随密码变化，改密后旧会话才会失效
func TestUserSessionStampFollowsPassword(t *testing.T) {
	repo := newUserRepoForTest(t)
	user, err := repo.Create("admin", "secret-1", "admin@example.com")
	must.NoError(t, err)

	before, err := repo.Get(user.ID)
	must.NoError(t, err)
	must.NoError(t, repo.UpdateEmail(user.ID, "new@example.com"))
	same, err := repo.Get(user.ID)
	must.NoError(t, err)
	check.Equal(t, same.SessionStamp(), before.SessionStamp())

	must.NoError(t, repo.UpdatePassword(user.ID, "secret-2"))
	after, err := repo.Get(user.ID)
	must.NoError(t, err)
	check.NotEqual(t, after.SessionStamp(), before.SessionStamp())
}
