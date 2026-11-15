package data

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
)

func newDatabaseDBForTest(t *testing.T) *gorm.DB {
	t.Helper()
	// 密码落库前会用 app.Key 加密
	app.Key = "0123456789abcdef0123456789abcdef"
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	must.NoError(t, err)
	must.NoError(t, db.AutoMigrate(&biz.DatabaseServer{}, &biz.DatabaseUser{}))
	must.NoError(t, db.Create(&biz.DatabaseServer{Name: "local_mysql", Type: biz.DatabaseTypeMysql, Password: "old"}).Error)
	return db
}

// 同一 (server_id, username, host) 重复 Upsert 只更新已有记录，不新增
func TestDatabaseUserUpsert(t *testing.T) {
	repo := &databaseUserRepo{db: newDatabaseDBForTest(t)}

	first := &biz.DatabaseUser{ServerID: 1, Username: "app", Host: "localhost", Password: "old", Remark: "a"}
	must.NoError(t, repo.Upsert(first))
	must.NotZero(t, first.ID)

	second := &biz.DatabaseUser{ServerID: 1, Username: "app", Host: "localhost", Password: "new", Remark: "b"}
	must.NoError(t, repo.Upsert(second))
	check.Equal(t, second.ID, first.ID)

	// 空 host 也是键的一部分，不能并到同名的其他 host 上
	other := &biz.DatabaseUser{ServerID: 1, Username: "app", Password: "any"}
	must.NoError(t, repo.Upsert(other))
	check.NotEqual(t, other.ID, first.ID)

	users, err := repo.ListByNames(1, []string{"app"})
	must.NoError(t, err)
	must.Len(t, users, 2)
	for _, user := range users {
		if user.ID == first.ID {
			check.Equal(t, user.Password, "new")
			check.Equal(t, user.Remark, "b")
			check.False(t, user.CreatedAt.IsZero())
		}
	}
}

// 改备注只动 remark 列，不连数据库，密码原样保留
func TestDatabaseUserUpdateRemark(t *testing.T) {
	repo := &databaseUserRepo{db: newDatabaseDBForTest(t)}

	user := &biz.DatabaseUser{ServerID: 1, Username: "app", Host: "localhost", Password: "secret", Remark: "a"}
	must.NoError(t, repo.Upsert(user))
	must.NoError(t, repo.UpdateRemark(&request.DatabaseUserUpdateRemark{ID: user.ID, Remark: "b"}))

	found, err := repo.Find(user.ID)
	must.NoError(t, err)
	check.Equal(t, found.Remark, "b")
	check.Equal(t, found.Password, "secret")
	check.Equal(t, found.Status, "")

	_, err = repo.Find(user.ID + 1)
	check.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

// 按类型过滤要 JOIN 服务器表，排序列不能有歧义
func TestDatabaseUserListByType(t *testing.T) {
	db := newDatabaseDBForTest(t)
	repo := &databaseUserRepo{db: db}

	// 用不支持用户管理的类型，填充状态时不会真的去连数据库
	server := &biz.DatabaseServer{Name: "cache", Type: biz.DatabaseTypeRedis}
	must.NoError(t, db.Create(server).Error)
	must.NoError(t, repo.Upsert(&biz.DatabaseUser{ServerID: server.ID, Username: "a", Password: "p"}))
	must.NoError(t, repo.Upsert(&biz.DatabaseUser{ServerID: server.ID, Username: "b", Password: "p"}))

	users, total, err := repo.List(t.Context(), 1, 10, string(biz.DatabaseTypeRedis))
	must.NoError(t, err)
	check.Equal(t, total, int64(2))
	must.Len(t, users, 2)
	check.Equal(t, users[0].Username, "b")
	check.Equal(t, users[0].Status, biz.DatabaseUserStatusInvalid)

	users, total, err = repo.List(t.Context(), 1, 10, string(biz.DatabaseTypeMongoDB))
	must.NoError(t, err)
	check.Equal(t, total, int64(0))
	check.Len(t, users, 0)
}

// 改密要经过加密钩子，库里不能留明文
func TestDatabaseServerUpdatePassword(t *testing.T) {
	db := newDatabaseDBForTest(t)
	repo := &databaseServerRepo{db: db}

	must.NoError(t, repo.UpdatePassword("local_mysql", "new"))

	var stored string
	must.NoError(t, db.Table("database_servers").Where("name = ?", "local_mysql").Pluck("password", &stored).Error)
	check.NotEqual(t, stored, "new")

	server, err := repo.FindByName("local_mysql")
	must.NoError(t, err)
	check.Equal(t, server.Password, "new")

	check.ErrorIs(t, repo.UpdatePassword("missing", "new"), gorm.ErrRecordNotFound)

	// 库里已有的明文密码照样能读出来
	must.NoError(t, db.Table("database_servers").Where("name = ?", "local_mysql").Update("password", "plain").Error)
	server, err = repo.FindByName("local_mysql")
	must.NoError(t, err)
	check.Equal(t, server.Password, "plain")
}
