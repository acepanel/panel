package data

import (
	"context"
	"fmt"

	lop "github.com/samber/lo/parallel"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type databaseServerRepo struct {
	db *gorm.DB
}

func NewDatabaseServerRepo(db *gorm.DB) biz.DatabaseServerRepo {
	return &databaseServerRepo{
		db: db,
	}
}

func (r *databaseServerRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.DatabaseServer{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func (r *databaseServerRepo) List(ctx context.Context, page, limit uint, typ string) ([]*biz.DatabaseServer, int64, error) {
	databaseServer := make([]*biz.DatabaseServer, 0)
	var total int64
	query := r.db.Model(&biz.DatabaseServer{}).Order("id desc")
	if typ != "" {
		query = query.Where("type = ?", typ)
	}
	err := query.Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&databaseServer).Error

	// 并发探测
	lop.ForEach(databaseServer, func(server *biz.DatabaseServer, _ int) {
		r.CheckServer(ctx, server)
	})

	return databaseServer, total, err
}

func (r *databaseServerRepo) Get(ctx context.Context, id uint) (*biz.DatabaseServer, error) {
	databaseServer, err := r.Find(id)
	if err != nil {
		return nil, err
	}

	r.CheckServer(ctx, databaseServer)

	return databaseServer, nil
}

func (r *databaseServerRepo) Find(id uint) (*biz.DatabaseServer, error) {
	server := new(biz.DatabaseServer)
	if err := r.db.Where("id = ?", id).First(server).Error; err != nil {
		return nil, err
	}

	return server, nil
}

func (r *databaseServerRepo) GetByName(ctx context.Context, name string) (*biz.DatabaseServer, error) {
	databaseServer := new(biz.DatabaseServer)
	if err := r.db.Where("name = ?", name).First(databaseServer).Error; err != nil {
		return nil, err
	}

	r.CheckServer(ctx, databaseServer)

	return databaseServer, nil
}

// Create 创建服务器记录
func (r *databaseServerRepo) Create(server *biz.DatabaseServer) error {
	return r.db.Create(server).Error
}

// Save 保存服务器记录
func (r *databaseServerRepo) Save(server *biz.DatabaseServer) error {
	return r.db.Save(server).Error
}

func (r *databaseServerRepo) UpdateRemark(req *request.DatabaseServerUpdateRemark) error {
	return r.db.Model(&biz.DatabaseServer{}).Where("id = ?", req.ID).Update("remark", req.Remark).Error
}

func (r *databaseServerRepo) UpdatePassword(name string, password string) error {
	return r.db.Model(&biz.DatabaseServer{}).Where("name = ?", name).Update("password", password).Error
}

func (r *databaseServerRepo) UpdatePort(name string, port uint) error {
	return r.db.Model(&biz.DatabaseServer{}).Where("name = ?", name).Update("port", port).Error
}

func (r *databaseServerRepo) Delete(id uint) error {
	if err := r.ClearUsers(id); err != nil {
		return err
	}

	return r.db.Where("id = ?", id).Delete(&biz.DatabaseServer{}).Error
}

// ClearUsers 删除指定服务器的所有用户，只是删除面板记录，不会实际删除
func (r *databaseServerRepo) ClearUsers(serverID uint) error {
	return r.db.Where("server_id = ?", serverID).Delete(&biz.DatabaseUser{}).Error
}

// ListUsers 查询指定服务器的本地用户记录
func (r *databaseServerRepo) ListUsers(serverID uint) ([]*biz.DatabaseUser, error) {
	users := make([]*biz.DatabaseUser, 0)
	if err := r.db.Where("server_id = ?", serverID).Find(&users).Error; err != nil {
		return nil, err
	}

	return users, nil
}

// CreateUser 创建同步用户记录
func (r *databaseServerRepo) CreateUser(user *biz.DatabaseUser) error {
	return r.db.Create(user).Error
}

// CheckServer 检查服务器连接，ctx 取消时立即放弃探测
func (r *databaseServerRepo) CheckServer(ctx context.Context, server *biz.DatabaseServer) bool {
	switch server.Type {
	case biz.DatabaseTypeMysql, biz.DatabaseTypePostgresql, biz.DatabaseTypeClickHouse:
		operator, err := r.Operator(ctx, server)
		if err == nil {
			operator.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	case biz.DatabaseTypeRedis:
		redis, err := db.NewRedis(ctx, server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err == nil {
			redis.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	case biz.DatabaseTypeMongoDB:
		mongo, err := db.NewMongoDB(ctx, server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err == nil {
			mongo.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	case biz.DatabaseTypeSQLite:
		sqlite, err := db.NewSQLite(ctx, server.Host)
		if err == nil {
			sqlite.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	case biz.DatabaseTypeElasticsearch:
		es, err := db.NewElasticsearch(ctx, fmt.Sprintf("%s:%d", server.Host, server.Port), server.Username, server.Password)
		if err == nil {
			es.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	}

	server.Status = biz.DatabaseServerStatusInvalid
	return false
}

// Operator 获取数据库操作句柄
func (r *databaseServerRepo) Operator(ctx context.Context, server *biz.DatabaseServer) (db.Operator, error) {
	switch server.Type {
	case biz.DatabaseTypeMysql:
		return newMySQLOperator(ctx, server.Username, server.Password, server.Host, server.Port)
	case biz.DatabaseTypePostgresql:
		postgres, err := db.NewPostgres(ctx, server.Username, server.Password, server.Host, server.Port)
		if err != nil {
			return nil, err
		}
		return postgres, nil
	case biz.DatabaseTypeClickHouse:
		clickhouse, err := db.NewClickHouse(ctx, server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err != nil {
			return nil, err
		}
		return clickhouse, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", server.Type)
	}
}

// newMySQLOperator 构建 MySQL 操作句柄
// 本地 MySQL 优先使用 unix socket 连接：开启 skip-name-resolve 后 TCP 127.0.0.1 无法反解为 localhost，默认的 root@localhost 账户会匹配失败
func newMySQLOperator(ctx context.Context, username, password, host string, port uint) (db.Operator, error) {
	if sock := localMySQLSocket(host); sock != "" {
		if mysql, err := db.NewMySQL(ctx, username, password, sock, "unix"); err == nil {
			return mysql, nil
		}
	}

	return db.NewMySQL(ctx, username, password, fmt.Sprintf("%s:%d", host, port))
}

// localMySQLSocket 返回本地 MySQL 的 unix socket 路径，非本地或未探测到返回空
func localMySQLSocket(host string) string {
	if host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return ""
	}
	return db.MySQLSocket(app.Root)
}
