package data

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/db"
)

type databaseServerRepo struct {
	t   *gotext.Locale
	db  *gorm.DB
	log *slog.Logger
}

func NewDatabaseServerRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger) biz.DatabaseServerRepo {
	return &databaseServerRepo{
		t:   t,
		db:  db,
		log: log,
	}
}

func (r *databaseServerRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.DatabaseServer{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func (r *databaseServerRepo) List(page, limit uint) ([]*biz.DatabaseServer, int64, error) {
	databaseServer := make([]*biz.DatabaseServer, 0)
	var total int64
	err := r.db.Model(&biz.DatabaseServer{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&databaseServer).Error

	for server := range slices.Values(databaseServer) {
		r.checkServer(server)
	}

	return databaseServer, total, err
}

func (r *databaseServerRepo) Get(id uint) (*biz.DatabaseServer, error) {
	databaseServer := new(biz.DatabaseServer)
	if err := r.db.Where("id = ?", id).First(databaseServer).Error; err != nil {
		return nil, err
	}

	r.checkServer(databaseServer)

	return databaseServer, nil
}

func (r *databaseServerRepo) GetByName(name string) (*biz.DatabaseServer, error) {
	databaseServer := new(biz.DatabaseServer)
	if err := r.db.Where("name = ?", name).First(databaseServer).Error; err != nil {
		return nil, err
	}

	r.checkServer(databaseServer)

	return databaseServer, nil
}

func (r *databaseServerRepo) Create(req *request.DatabaseServerCreate) error {
	databaseServer := &biz.DatabaseServer{
		Name:     req.Name,
		Type:     biz.DatabaseType(req.Type),
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Remark:   req.Remark,
	}

	if !r.checkServer(databaseServer) {
		return errors.New(r.t.Get("check server connection failed"))
	}

	return r.db.Create(databaseServer).Error
}

func (r *databaseServerRepo) Update(req *request.DatabaseServerUpdate) error {
	server, err := r.Get(req.ID)
	if err != nil {
		return err
	}

	server.Name = req.Name
	server.Host = req.Host
	server.Port = req.Port
	server.Username = req.Username
	server.Password = req.Password
	server.Remark = req.Remark

	if !r.checkServer(server) {
		return errors.New(r.t.Get("check server connection failed"))
	}

	return r.db.Save(server).Error
}

func (r *databaseServerRepo) UpdateRemark(req *request.DatabaseServerUpdateRemark) error {
	return r.db.Model(&biz.DatabaseServer{}).Where("id = ?", req.ID).Update("remark", req.Remark).Error
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

func (r *databaseServerRepo) Sync(id uint) error {
	server, err := r.Get(id)
	if err != nil {
		return err
	}

	users := make([]*biz.DatabaseUser, 0)
	if err = r.db.Where("server_id = ?", id).Find(&users).Error; err != nil {
		return err
	}

	operator, err := r.getOperator(server)
	if err != nil {
		return err
	}
	defer operator.Close()

	switch server.Type {
	case biz.DatabaseTypeMysql:
		allUsers, err := operator.Users()
		if err != nil {
			return err
		}
		for user := range slices.Values(allUsers) {
			if !slices.ContainsFunc(users, func(a *biz.DatabaseUser) bool {
				return a.Username == user.User && a.Host == user.Host
			}) && !slices.Contains([]string{"root", "mysql.sys", "mysql.session", "mysql.infoschema"}, user.User) {
				newUser := &biz.DatabaseUser{
					ServerID: id,
					Username: user.User,
					Host:     user.Host,
					Remark:   r.t.Get("sync from server %s", server.Name),
				}
				if err = r.db.Create(newUser).Error; err != nil {
					r.log.Warn("sync mysql database user failed", slog.String("type", biz.OperationTypeDatabaseServer), slog.Uint64("operator_id", 0), slog.Any("err", err))
				}
			}
		}
	case biz.DatabaseTypePostgresql:
		allUsers, err := operator.Users()
		if err != nil {
			return err
		}
		for user := range slices.Values(allUsers) {
			if !slices.ContainsFunc(users, func(a *biz.DatabaseUser) bool {
				return a.Username == user.User
			}) && !slices.Contains([]string{"postgres"}, user.User) {
				newUser := &biz.DatabaseUser{
					ServerID: id,
					Username: user.User,
					Remark:   r.t.Get("sync from server %s", server.Name),
				}
				if err = r.db.Create(newUser).Error; err != nil {
					r.log.Warn("sync postgresql database user failed", slog.String("type", biz.OperationTypeDatabaseServer), slog.Uint64("operator_id", 0), slog.Any("err", err))
				}
			}
		}
	}

	return nil
}

// checkServer 检查服务器连接
func (r *databaseServerRepo) checkServer(server *biz.DatabaseServer) bool {
	switch server.Type {
	case biz.DatabaseTypeMysql, biz.DatabaseTypePostgresql:
		operator, err := r.getOperator(server)
		if err == nil {
			operator.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	case biz.DatabaseTypeRedis:
		redis, err := db.NewRedis(server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err == nil {
			redis.Close()
			server.Status = biz.DatabaseServerStatusValid
			return true
		}
	}

	server.Status = biz.DatabaseServerStatusInvalid
	return false
}

func (r *databaseServerRepo) getOperator(server *biz.DatabaseServer) (db.Operator, error) {
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
