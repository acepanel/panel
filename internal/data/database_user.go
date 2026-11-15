package data

import (
	"context"

	lop "github.com/samber/lo/parallel"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type databaseUserRepo struct {
	db *gorm.DB
}

func NewDatabaseUserRepo(db *gorm.DB) biz.DatabaseUserRepo {
	return &databaseUserRepo{
		db: db,
	}
}

func (r *databaseUserRepo) List(ctx context.Context, page, limit uint, typ string) ([]*biz.DatabaseUser, int64, error) {
	user := make([]*biz.DatabaseUser, 0)
	var total int64
	// 排序列带表名：按类型过滤时会 JOIN 服务器表，裸 id 有歧义
	query := r.db.Model(&biz.DatabaseUser{}).Preload("Server").Order("database_users.id desc")
	if typ != "" {
		query = query.Joins("JOIN database_servers ON database_servers.id = database_users.server_id").Where("database_servers.type = ?", typ)
	}
	err := query.Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&user).Error

	// 并发填充
	lop.ForEach(user, func(u *biz.DatabaseUser, _ int) {
		r.fillUser(ctx, u)
	})

	return user, total, err
}

func (r *databaseUserRepo) Get(ctx context.Context, id uint) (*biz.DatabaseUser, error) {
	user := new(biz.DatabaseUser)
	if err := r.db.Preload("Server").Where("id = ?", id).First(user).Error; err != nil {
		return nil, err
	}

	r.fillUser(ctx, user)

	return user, nil
}

// Find 只读记录，不连数据库查权限和登录状态
func (r *databaseUserRepo) Find(id uint) (*biz.DatabaseUser, error) {
	user := new(biz.DatabaseUser)
	if err := r.db.Where("id = ?", id).First(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// ListByNames 查询指定服务器下匹配用户名的用户记录
func (r *databaseUserRepo) ListByNames(serverID uint, names []string) ([]*biz.DatabaseUser, error) {
	users := make([]*biz.DatabaseUser, 0)
	if err := r.db.Where("server_id = ? AND username IN ?", serverID, names).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

// Upsert 按 (server_id, username, host) 创建或更新用户记录（FirstOrInit + Save，非原子保留）
func (r *databaseUserRepo) Upsert(user *biz.DatabaseUser) error {
	// 条件用 map：结构体条件会跳过空 host，从而匹配到同名的其他 host
	key := map[string]any{"server_id": user.ServerID, "username": user.Username, "host": user.Host}
	record := new(biz.DatabaseUser)
	if err := r.db.Where(key).FirstOrInit(record).Error; err != nil {
		return err
	}

	record.Password = user.Password
	record.Remark = user.Remark
	if err := r.db.Save(record).Error; err != nil {
		return err
	}
	user.ID = record.ID

	return nil
}

// Save 保存用户记录
func (r *databaseUserRepo) Save(user *biz.DatabaseUser) error {
	return r.db.Omit("Server").Save(user).Error
}

func (r *databaseUserRepo) UpdateRemark(req *request.DatabaseUserUpdateRemark) error {
	return r.db.Model(&biz.DatabaseUser{}).Where("id = ?", req.ID).Update("remark", req.Remark).Error
}

// DeleteByID 按 ID 删除用户记录
func (r *databaseUserRepo) DeleteByID(id uint) error {
	return r.db.Where("id = ?", id).Delete(&biz.DatabaseUser{}).Error
}

// DeleteByServerNames 按服务器与用户名删除用户记录
func (r *databaseUserRepo) DeleteByServerNames(serverID uint, names []string) error {
	return r.db.Where("server_id = ? AND username IN ?", serverID, names).Delete(&biz.DatabaseUser{}).Error
}

// Operator 获取数据库操作句柄
func (r *databaseUserRepo) Operator(ctx context.Context, server *biz.DatabaseServer) (db.Operator, error) {
	return newOperator(ctx, server)
}

func (r *databaseUserRepo) fillUser(ctx context.Context, user *biz.DatabaseUser) {
	// 初始化，防止 nil
	defer func() {
		if user.Privileges == nil {
			user.Privileges = make([]string, 0)
		}
	}()

	// 避免每行再查一次服务器记录
	server := user.Server
	if server == nil {
		var err error
		if server, err = r.loadServer(user.ServerID); err != nil {
			return
		}
	}

	operator, err := newOperator(ctx, server)
	if err != nil {
		user.Status = biz.DatabaseUserStatusInvalid
		return
	}
	defer operator.Close()

	user.Privileges, _ = operator.UserPrivileges(ctx, user.Username, user.Host)

	// 用该用户自己的账号密码登录一次，判断面板记录的密码是否还有效
	login := *server
	login.Username, login.Password = user.Username, user.Password
	user.Status = biz.DatabaseUserStatusInvalid
	if probe, err := newOperator(ctx, &login); err == nil {
		probe.Close()
		user.Status = biz.DatabaseUserStatusValid
	}
}

// loadServer 直读服务器记录，断开 repo→repo，仅取连接信息
func (r *databaseUserRepo) loadServer(id uint) (*biz.DatabaseServer, error) {
	server := new(biz.DatabaseServer)
	if err := r.db.Where("id = ?", id).First(server).Error; err != nil {
		return nil, err
	}

	return server, nil
}
