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

type databaseUserRepo struct {
	t      *gotext.Locale
	db     *gorm.DB
	server biz.DatabaseServerRepo
}

func NewDatabaseUserRepo(t *gotext.Locale, db *gorm.DB, server biz.DatabaseServerRepo) biz.DatabaseUserRepo {
	return &databaseUserRepo{
		t:      t,
		db:     db,
		server: server,
	}
}

func (r *databaseUserRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.DatabaseUser{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func (r *databaseUserRepo) List(page, limit uint) ([]*biz.DatabaseUser, int64, error) {
	user := make([]*biz.DatabaseUser, 0)
	var total int64
	err := r.db.Model(&biz.DatabaseUser{}).Preload("Server").Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&user).Error

	for u := range slices.Values(user) {
		r.fillUser(u)
	}

	return user, total, err
}

func (r *databaseUserRepo) Get(id uint) (*biz.DatabaseUser, error) {
	user := new(biz.DatabaseUser)
	if err := r.db.Preload("Server").Where("id = ?", id).First(user).Error; err != nil {
		return nil, err
	}

	r.fillUser(user)

	return user, nil
}

func (r *databaseUserRepo) Create(req *request.DatabaseUserCreate) error {
	server, err := r.server.Get(req.ServerID)
	if err != nil {
		return err
	}

	operator, err := r.getOperator(server)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 创建用户
	if err = operator.UserCreate(req.Username, req.Password); err != nil {
		return err
	}

	// 创建数据库并授权
	for name := range slices.Values(req.Privileges) {
		if err = operator.DatabaseCreate(name); err != nil {
			return err
		}
		if err = operator.PrivilegesGrant(req.Username, name); err != nil {
			return err
		}
	}

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

func (r *databaseUserRepo) Update(req *request.DatabaseUserUpdate) error {
	user, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	server, err := r.server.Get(user.ServerID)
	if err != nil {
		return err
	}

	operator, err := r.getOperator(server)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 更新密码
	if req.Password != "" {
		if err = operator.UserPassword(user.Username, req.Password); err != nil {
			return err
		}
	}

	// 创建数据库并授权
	for name := range slices.Values(req.Privileges) {
		if err = operator.DatabaseCreate(name); err != nil {
			return err
		}
		if err = operator.PrivilegesGrant(user.Username, name); err != nil {
			return err
		}
	}

	user.Password = req.Password
	user.Remark = req.Remark

	return r.db.Save(user).Error
}

func (r *databaseUserRepo) UpdateRemark(req *request.DatabaseUserUpdateRemark) error {
	user, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	user.Remark = req.Remark

	return r.db.Save(user).Error
}

func (r *databaseUserRepo) Delete(id uint) error {
	user, err := r.Get(id)
	if err != nil {
		return err
	}

	server, err := r.server.Get(user.ServerID)
	if err != nil {
		return err
	}

	operator, err := r.getOperator(server)
	if err != nil {
		return err
	}
	defer operator.Close()

	_ = operator.UserDrop(user.Username)

	return r.db.Where("id = ?", id).Delete(&biz.DatabaseUser{}).Error
}

func (r *databaseUserRepo) DeleteByNames(serverID uint, names []string) error {
	server, err := r.server.Get(serverID)
	if err != nil {
		return err
	}

	operator, err := r.getOperator(server)
	if err != nil {
		return err
	}
	defer operator.Close()

	switch server.Type {
	case biz.DatabaseTypeMysql:
		users := make([]*biz.DatabaseUser, 0)
		if err = r.db.Where("server_id = ? AND username IN ?", serverID, names).Find(&users).Error; err != nil {
			return err
		}
		for name := range slices.Values(names) {
			host := "localhost"
			for u := range slices.Values(users) {
				if u.Username == name {
					host = u.Host
					break
				}
			}
			_ = operator.UserDrop(name, host)
		}
	case biz.DatabaseTypePostgresql:
		for name := range slices.Values(names) {
			_ = operator.UserDrop(name)
		}
	}

	return r.db.Where("server_id = ? AND username IN ?", serverID, names).Delete(&biz.DatabaseUser{}).Error
}

func (r *databaseUserRepo) fillUser(user *biz.DatabaseUser) {
	server, err := r.server.Get(user.ServerID)
	if err == nil {
		operator, err := r.getOperator(server)
		if err == nil {
			defer operator.Close()
			switch server.Type {
			case biz.DatabaseTypeMysql:
				privileges, _ := operator.UserPrivileges(user.Username, user.Host)
				user.Privileges = privileges
				if mysql2, err := db.NewMySQL(user.Username, user.Password, fmt.Sprintf("%s:%d", server.Host, server.Port)); err == nil {
					mysql2.Close()
					user.Status = biz.DatabaseUserStatusValid
				} else {
					user.Status = biz.DatabaseUserStatusInvalid
				}
			case biz.DatabaseTypePostgresql:
				privileges, _ := operator.UserPrivileges(user.Username)
				user.Privileges = privileges
				if postgres2, err := db.NewPostgres(user.Username, user.Password, server.Host, server.Port); err == nil {
					postgres2.Close()
					user.Status = biz.DatabaseUserStatusValid
				} else {
					user.Status = biz.DatabaseUserStatusInvalid
				}
			}
		}
	}
	// 初始化，防止 nil
	if user.Privileges == nil {
		user.Privileges = make([]string, 0)
	}
}

func (r *databaseUserRepo) getOperator(server *biz.DatabaseServer) (db.Operator, error) {
	switch server.Type {
	case biz.DatabaseTypeMysql:
		mysql, err := db.NewMySQL(server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err != nil {
			return nil, err
		}
		return mysql, nil
	case biz.DatabaseTypePostgresql:
		postgres, err := db.NewPostgres(server.Username, server.Password, server.Host, server.Port)
		if err != nil {
			return nil, err
		}
		return postgres, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", server.Type)
	}
}
