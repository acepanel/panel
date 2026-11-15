package data

import (
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

func newUserRepoForTest(t *testing.T) biz.UserRepo {
	t.Helper()
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
