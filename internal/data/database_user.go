// Package data 提供数据访问层实现
//
// 本模块负责实现业务层定义的数据访问接口，主要包括：
// - 数据库用户的 CRUD 操作
// - 跨数据库类型的统一操作抽象（MySQL/PostgreSQL）
// - 用户权限和数据库管理
//
// 依赖：
// - internal/biz: 业务逻辑层接口定义
// - pkg/db: 数据库客户端封装
// - gorm.io/gorm: ORM 框架
//
// 设计模式：
// - Repository 模式：实现 biz.DatabaseUserRepo 接口
// - 适配器模式：统一 MySQL 和 PostgreSQL 的操作接口
//
// 作者: Catsayer
// 创建时间: 2025-12-04
// 最后修改: 2025-12-04
package data

import (
	"fmt"
	"slices"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/db"
)

// databaseOperator 统一的数据库操作接口
//
// 该接口抽象了 MySQL 和 PostgreSQL 的差异，提供统一的用户和权限管理操作。
// 使用适配器模式将不同数据库的 API 统一为相同的接口。
type databaseOperator interface {
	// 用户操作
	createUser(username, password string) error
	dropUser(username string) error
	updatePassword(username, password string) error

	// 权限操作
	createDatabase(name string) error
	grantPrivileges(username, database string) error
	getUserPrivileges(username string) ([]string, error)

	// 连接验证
	testConnection(username, password string) error

	// 连接管理
	Close() error
}

// mysqlOperator MySQL 数据库操作适配器
//
// 适配 MySQL 特定的 API，主要差异：
// - 用户操作需要 host 参数（如 'user'@'localhost'）
// - 权限控制粒度更细（支持 host 级别的权限）
type mysqlOperator struct {
	*db.MySQL
	host   string // MySQL 用户的 host 字段（如 localhost, %, 192.168.1.%）
	server *biz.DatabaseServer
}

// createUser 创建 MySQL 用户
//
// 参数：
//
//	username: 用户名
//	password: 密码
//
// 返回：
//
//	error: 创建失败时返回错误
func (m *mysqlOperator) createUser(username, password string) error {
	return m.MySQL.UserCreate(username, password, m.host)
}

// dropUser 删除 MySQL 用户
func (m *mysqlOperator) dropUser(username string) error {
	return m.MySQL.UserDrop(username, m.host)
}

// updatePassword 修改 MySQL 用户密码
func (m *mysqlOperator) updatePassword(username, password string) error {
	return m.MySQL.UserPassword(username, password, m.host)
}

// createDatabase 创建 MySQL 数据库
func (m *mysqlOperator) createDatabase(name string) error {
	return m.MySQL.DatabaseCreate(name)
}

// grantPrivileges 授予 MySQL 用户对指定数据库的所有权限
func (m *mysqlOperator) grantPrivileges(username, database string) error {
	return m.MySQL.PrivilegesGrant(username, database, m.host)
}

// getUserPrivileges 获取 MySQL 用户拥有权限的数据库列表
func (m *mysqlOperator) getUserPrivileges(username string) ([]string, error) {
	return m.MySQL.UserPrivileges(username, m.host)
}

// testConnection 测试 MySQL 用户连接是否有效
//
// 通过尝试使用用户凭据建立新连接来验证用户状态
func (m *mysqlOperator) testConnection(username, password string) error {
	conn, err := db.NewMySQL(username, password, fmt.Sprintf("%s:%d", m.server.Host, m.server.Port))
	if err != nil {
		return err
	}
	return conn.Close()
}

// postgresOperator PostgreSQL 数据库操作适配器
//
// 适配 PostgreSQL 特定的 API，主要差异：
// - 用户操作不需要 host 参数（通过 pg_hba.conf 控制访问）
// - 权限通过 OWNER 和 GRANT 管理
type postgresOperator struct {
	*db.Postgres
	server *biz.DatabaseServer
}

// createUser 创建 PostgreSQL 用户
func (p *postgresOperator) createUser(username, password string) error {
	return p.Postgres.UserCreate(username, password)
}

// dropUser 删除 PostgreSQL 用户
func (p *postgresOperator) dropUser(username string) error {
	return p.Postgres.UserDrop(username)
}

// updatePassword 修改 PostgreSQL 用户密码
func (p *postgresOperator) updatePassword(username, password string) error {
	return p.Postgres.UserPassword(username, password)
}

// createDatabase 创建 PostgreSQL 数据库
func (p *postgresOperator) createDatabase(name string) error {
	return p.Postgres.DatabaseCreate(name)
}

// grantPrivileges 授予 PostgreSQL 用户对指定数据库的所有权限
func (p *postgresOperator) grantPrivileges(username, database string) error {
	return p.Postgres.PrivilegesGrant(username, database)
}

// getUserPrivileges 获取 PostgreSQL 用户拥有权限的数据库列表
func (p *postgresOperator) getUserPrivileges(username string) ([]string, error) {
	return p.Postgres.UserPrivileges(username)
}

// testConnection 测试 PostgreSQL 用户连接是否有效
func (p *postgresOperator) testConnection(username, password string) error {
	conn, err := db.NewPostgres(username, password, p.server.Host, p.server.Port)
	if err != nil {
		return err
	}
	return conn.Close()
}

// databaseUserRepo 数据库用户仓库实现
type databaseUserRepo struct {
	t      *gotext.Locale
	db     *gorm.DB
	server biz.DatabaseServerRepo
}

// NewDatabaseUserRepo 创建数据库用户仓库实例
//
// 参数：
//
//	t: 国际化翻译器
//	db: GORM 数据库连接
//	server: 数据库服务器仓库接口
//
// 返回：
//
//	biz.DatabaseUserRepo: 数据库用户仓库接口实例
func NewDatabaseUserRepo(t *gotext.Locale, db *gorm.DB, server biz.DatabaseServerRepo) biz.DatabaseUserRepo {
	return &databaseUserRepo{
		t:      t,
		db:     db,
		server: server,
	}
}

// getOperator 获取数据库操作器（工厂方法）
//
// 参数：
//
//	server: 数据库服务器配置
//	host: MySQL 用户的 host 字段（PostgreSQL 时忽略）
//
// 返回：
//
//	databaseOperator: 数据库操作接口实例
//	error: 连接失败或不支持的数据库类型时返回错误
//
// 注意事项：
//  1. 调用者负责调用返回的 operator.Close() 释放连接
//  2. 不支持的数据库类型会返回错误
func (r *databaseUserRepo) getOperator(server *biz.DatabaseServer, host string) (databaseOperator, error) {
	switch server.Type {
	case biz.DatabaseTypeMysql:
		mysql, err := db.NewMySQL(server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err != nil {
			return nil, err
		}
		return &mysqlOperator{MySQL: mysql, host: host, server: server}, nil
	case biz.DatabaseTypePostgresql:
		postgres, err := db.NewPostgres(server.Username, server.Password, server.Host, server.Port)
		if err != nil {
			return nil, err
		}
		return &postgresOperator{Postgres: postgres, server: server}, nil
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", server.Type)
	}
}

// Count 统计数据库用户总数
//
// 返回：
//
//	int64: 用户总数
//	error: 查询失败时返回错误
func (r databaseUserRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.DatabaseUser{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

// List 分页获取数据库用户列表
//
// 参数：
//
//	page: 页码（从 1 开始）
//	limit: 每页数量
//
// 返回：
//
//	[]*biz.DatabaseUser: 用户列表（包含权限和状态信息）
//	int64: 总记录数
//	error: 查询失败时返回错误
func (r databaseUserRepo) List(page, limit uint) ([]*biz.DatabaseUser, int64, error) {
	user := make([]*biz.DatabaseUser, 0)
	var total int64
	err := r.db.Model(&biz.DatabaseUser{}).Preload("Server").Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&user).Error

	for u := range slices.Values(user) {
		r.fillUser(u)
	}

	return user, total, err
}

// Get 根据 ID 获取数据库用户详情
//
// 参数：
//
//	id: 用户 ID
//
// 返回：
//
//	*biz.DatabaseUser: 用户详情（包含权限和状态信息）
//	error: 用户不存在或查询失败时返回错误
func (r databaseUserRepo) Get(id uint) (*biz.DatabaseUser, error) {
	user := new(biz.DatabaseUser)
	if err := r.db.Preload("Server").Where("id = ?", id).First(user).Error; err != nil {
		return nil, err
	}

	r.fillUser(user)

	return user, nil
}

// Create 创建数据库用户并授予指定数据库权限
//
// 参数：
//
//	req: 创建用户请求（包含用户名、密码、Host、权限列表等）
//
// 返回：
//
//	error: 创建失败时返回错误
//
// 注意事项：
//  1. 会自动创建请求中指定的数据库（如果不存在）
//  2. MySQL 用户需要指定 Host，PostgreSQL 不需要
//  3. 创建成功后会将用户信息保存到本地数据库
func (r databaseUserRepo) Create(req *request.DatabaseUserCreate) error {
	server, err := r.server.Get(req.ServerID)
	if err != nil {
		return err
	}

	// 获取数据库操作器
	operator, err := r.getOperator(server, req.Host)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 创建用户
	if err = operator.createUser(req.Username, req.Password); err != nil {
		return err
	}

	// 创建数据库并授权
	for name := range slices.Values(req.Privileges) {
		if err = operator.createDatabase(name); err != nil {
			return err
		}
		if err = operator.grantPrivileges(req.Username, name); err != nil {
			return err
		}
	}

	// 保存到本地数据库
	user := &biz.DatabaseUser{
		ServerID: req.ServerID,
		Username: req.Username,
		Host:     req.Host,
		Password: req.Password,
		Remark:   req.Remark,
	}

	if err = r.db.FirstOrInit(user, user).Error; err != nil {
		return err
	}

	return r.db.Save(user).Error
}

// Update 更新数据库用户密码和权限
//
// 参数：
//
//	req: 更新用户请求（包含用户 ID、新密码、新权限列表等）
//
// 返回：
//
//	error: 更新失败时返回错误
//
// 注意事项：
//  1. 密码为空时不更新密码
//  2. 会自动创建请求中指定的数据库（如果不存在）
//  3. 不会撤销未在请求中列出的旧权限（仅授予新权限）
func (r databaseUserRepo) Update(req *request.DatabaseUserUpdate) error {
	user, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	server, err := r.server.Get(user.ServerID)
	if err != nil {
		return err
	}

	// 获取数据库操作器
	operator, err := r.getOperator(server, user.Host)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 更新密码
	if req.Password != "" {
		if err = operator.updatePassword(user.Username, req.Password); err != nil {
			return err
		}
	}

	// 创建数据库并授权
	for name := range slices.Values(req.Privileges) {
		if err = operator.createDatabase(name); err != nil {
			return err
		}
		if err = operator.grantPrivileges(user.Username, name); err != nil {
			return err
		}
	}

	// 更新本地数据库记录
	user.Password = req.Password
	user.Remark = req.Remark

	return r.db.Save(user).Error
}

// UpdateRemark 更新数据库用户备注
//
// 参数：
//
//	req: 更新备注请求（包含用户 ID 和新备注）
//
// 返回：
//
//	error: 更新失败时返回错误
func (r databaseUserRepo) UpdateRemark(req *request.DatabaseUserUpdateRemark) error {
	user, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	user.Remark = req.Remark

	return r.db.Save(user).Error
}

// Delete 删除数据库用户
//
// 参数：
//
//	id: 用户 ID
//
// 返回：
//
//	error: 删除失败时返回错误
//
// 注意事项：
//  1. 会同时删除远程数据库中的用户
//  2. 删除失败不会影响本地数据库记录的删除（使用 _ 忽略删除错误）
func (r databaseUserRepo) Delete(id uint) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	server, err := r.server.Get(user.ServerID)
	if err != nil {
		return err
	}

	// 获取数据库操作器并删除远程用户
	operator, err := r.getOperator(server, user.Host)
	if err != nil {
		return err
	}
	defer operator.Close()

	_ = operator.dropUser(user.Username)

	return r.db.Where("id = ?", id).Delete(&biz.DatabaseUser{}).Error
}

// DeleteByNames 批量删除指定服务器上的数据库用户
//
// 参数：
//
//	serverID: 数据库服务器 ID
//	names: 用户名列表
//
// 返回：
//
//	error: 删除失败时返回错误
//
// 注意事项：
//  1. MySQL 需要从本地数据库查询用户的 Host 信息
//  2. 远程删除失败不会影响本地数据库记录的删除
func (r databaseUserRepo) DeleteByNames(serverID uint, names []string) error {
	server, err := r.server.Get(serverID)
	if err != nil {
		return err
	}

	// 处理 MySQL 的特殊逻辑：需要获取每个用户的 Host
	if server.Type == biz.DatabaseTypeMysql {
		mysql, err := db.NewMySQL(server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err != nil {
			return err
		}
		defer mysql.Close()

		// 查询用户列表获取 Host 信息
		users := make([]*biz.DatabaseUser, 0)
		if err = r.db.Where("server_id = ? AND username IN ?", serverID, names).Find(&users).Error; err != nil {
			return err
		}

		// 删除每个用户
		for name := range slices.Values(names) {
			host := "localhost" // 默认 host
			for u := range slices.Values(users) {
				if u.Username == name {
					host = u.Host
					break
				}
			}
			_ = mysql.UserDrop(name, host)
		}
	} else if server.Type == biz.DatabaseTypePostgresql {
		postgres, err := db.NewPostgres(server.Username, server.Password, server.Host, server.Port)
		if err != nil {
			return err
		}
		defer postgres.Close()

		// PostgreSQL 不需要 Host，直接删除
		for name := range slices.Values(names) {
			_ = postgres.UserDrop(name)
		}
	}

	return r.db.Where("server_id = ? AND username IN ?", serverID, names).Delete(&biz.DatabaseUser{}).Error
}

// fillUser 填充数据库用户的权限和状态信息
//
// 参数：
//
//	user: 待填充的用户对象
//
// 注意事项：
//  1. 会查询远程数据库获取用户的实时权限列表
//  2. 会尝试使用用户凭据连接数据库以验证状态（Valid/Invalid）
//  3. 查询失败时会初始化空权限列表，避免 nil
func (r databaseUserRepo) fillUser(user *biz.DatabaseUser) {
	server, err := r.server.Get(user.ServerID)
	if err != nil {
		// 无法获取服务器信息，初始化空权限列表
		if user.Privileges == nil {
			user.Privileges = make([]string, 0)
		}
		return
	}

	// 获取数据库操作器
	operator, err := r.getOperator(server, user.Host)
	if err == nil {
		defer operator.Close()

		// 获取用户权限
		privileges, _ := operator.getUserPrivileges(user.Username)
		user.Privileges = privileges

		// 测试用户连接状态
		if err := operator.testConnection(user.Username, user.Password); err == nil {
			user.Status = biz.DatabaseUserStatusValid
		} else {
			user.Status = biz.DatabaseUserStatusInvalid
		}
	}

	// 初始化，防止 nil
	if user.Privileges == nil {
		user.Privileges = make([]string, 0)
	}
}
