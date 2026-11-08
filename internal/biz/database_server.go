package biz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/crypt"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type DatabaseServerStatus string

const (
	DatabaseServerStatusValid   DatabaseServerStatus = "valid"
	DatabaseServerStatusInvalid DatabaseServerStatus = "invalid"
)

type DatabaseServer struct {
	ID        uint                 `gorm:"primaryKey" json:"id"`
	Name      string               `gorm:"not null;default:'';unique" json:"name"`
	Type      DatabaseType         `gorm:"not null;default:''" json:"type"`
	Host      string               `gorm:"not null;default:''" json:"host"`
	Port      uint                 `gorm:"not null;default:0" json:"port"`
	Username  string               `gorm:"not null;default:''" json:"username"`
	Password  string               `gorm:"not null;default:''" json:"password"`
	Status    DatabaseServerStatus `gorm:"-:all" json:"status"`
	Remark    string               `gorm:"not null;default:''" json:"remark"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
}

func (r *DatabaseServer) BeforeSave(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	r.Password, err = crypter.Encrypt([]byte(r.Password))
	if err != nil {
		return err
	}

	return nil
}

func (r *DatabaseServer) AfterFind(tx *gorm.DB) error {
	crypter, err := crypt.NewXChacha20Poly1305([]byte(app.Key))
	if err != nil {
		return err
	}

	password, err := crypter.Decrypt(r.Password)
	if err == nil {
		r.Password = string(password)
	}

	return nil
}

type DatabaseServerRepo interface {
	Count() (int64, error)
	List(ctx context.Context, page, limit uint, typ string) ([]*DatabaseServer, int64, error)
	Get(ctx context.Context, id uint) (*DatabaseServer, error)
	Find(id uint) (*DatabaseServer, error)
	GetByName(ctx context.Context, name string) (*DatabaseServer, error)
	Create(server *DatabaseServer) error
	Save(server *DatabaseServer) error
	UpdateRemark(req *request.DatabaseServerUpdateRemark) error
	UpdatePassword(name string, password string) error
	UpdatePort(name string, port uint) error
	Delete(id uint) error
	ClearUsers(id uint) error
	ListUsers(serverID uint) ([]*DatabaseUser, error)
	CreateUser(user *DatabaseUser) error
	Operator(ctx context.Context, server *DatabaseServer) (db.Operator, error)
	CheckServer(ctx context.Context, server *DatabaseServer) bool
}

// DatabaseServerUsecase 数据库服务器业务用例
type DatabaseServerUsecase struct {
	repo DatabaseServerRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewDatabaseServerUsecase(t *gotext.Locale, log *slog.Logger, databaseServerRepo DatabaseServerRepo) *DatabaseServerUsecase {
	return &DatabaseServerUsecase{
		repo: databaseServerRepo,
		t:    t,
		log:  log,
	}
}

func (uc *DatabaseServerUsecase) Count() (int64, error) {
	return uc.repo.Count()
}

func (uc *DatabaseServerUsecase) List(ctx context.Context, page, limit uint, typ string) ([]*DatabaseServer, int64, error) {
	return uc.repo.List(ctx, page, limit, typ)
}

func (uc *DatabaseServerUsecase) Get(ctx context.Context, id uint) (*DatabaseServer, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *DatabaseServerUsecase) GetByName(ctx context.Context, name string) (*DatabaseServer, error) {
	return uc.repo.GetByName(ctx, name)
}

func (uc *DatabaseServerUsecase) Create(ctx context.Context, req *request.DatabaseServerCreate) error {
	databaseServer := &DatabaseServer{
		Name:     req.Name,
		Type:     DatabaseType(req.Type),
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Remark:   req.Remark,
	}

	if !uc.repo.CheckServer(ctx, databaseServer) {
		return errors.New(uc.t.Get("check server connection failed"))
	}

	if err := uc.repo.Create(databaseServer); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("database server created"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(databaseServer.ID)), slog.String("name", databaseServer.Name), slog.String("server_type", string(databaseServer.Type)), slog.String("host", databaseServer.Host), slog.Uint64("port", uint64(databaseServer.Port)), slog.String("username", databaseServer.Username))

	return nil
}

func (uc *DatabaseServerUsecase) Update(ctx context.Context, req *request.DatabaseServerUpdate) error {
	server, err := uc.repo.Get(ctx, req.ID)
	if err != nil {
		return err
	}

	server.Name = req.Name
	server.Host = req.Host
	server.Port = req.Port
	server.Username = req.Username
	server.Password = req.Password
	server.Remark = req.Remark

	if !uc.repo.CheckServer(ctx, server) {
		return errors.New(uc.t.Get("check server connection failed"))
	}

	if err = uc.repo.Save(server); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("database server updated"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(server.ID)), slog.String("name", server.Name), slog.String("server_type", string(server.Type)), slog.String("host", server.Host), slog.Uint64("port", uint64(server.Port)), slog.String("username", server.Username))

	return nil
}

func (uc *DatabaseServerUsecase) UpdateRemark(ctx context.Context, req *request.DatabaseServerUpdateRemark) error {
	server, err := uc.repo.Find(req.ID)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdateRemark(req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("database server remark updated"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("name", server.Name), slog.String("remark", req.Remark))

	return nil
}

func (uc *DatabaseServerUsecase) UpdatePassword(name string, password string) error {
	return uc.repo.UpdatePassword(name, password)
}

func (uc *DatabaseServerUsecase) UpdatePort(name string, port uint) error {
	return uc.repo.UpdatePort(name, port)
}

func (uc *DatabaseServerUsecase) Delete(ctx context.Context, id uint) error {
	server, err := uc.repo.Find(id)
	if err != nil {
		return err
	}

	if err = uc.repo.Delete(id); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("database server deleted"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("name", server.Name), slog.String("server_type", string(server.Type)), slog.String("host", server.Host), slog.Uint64("port", uint64(server.Port)))

	return nil
}

func (uc *DatabaseServerUsecase) ClearUsers(id uint) error {
	return uc.repo.ClearUsers(id)
}

func (uc *DatabaseServerUsecase) Sync(ctx context.Context, id uint) error {
	server, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// 非 Operator 类型不支持用户同步
	switch server.Type {
	case DatabaseTypeMysql, DatabaseTypePostgresql, DatabaseTypeClickHouse:
	default:
		return fmt.Errorf("sync is not supported for %s", server.Type)
	}

	users, err := uc.repo.ListUsers(id)
	if err != nil {
		return err
	}

	operator, err := uc.repo.Operator(ctx, server)
	if err != nil {
		return err
	}
	defer operator.Close()

	synced := 0
	switch server.Type {
	case DatabaseTypeMysql:
		allUsers, err := operator.Users(ctx)
		if err != nil {
			return err
		}
		for user := range slices.Values(allUsers) {
			if !slices.ContainsFunc(users, func(a *DatabaseUser) bool {
				return a.Username == user.User && a.Host == user.Host
			}) && !slices.Contains([]string{"root", "mysql.sys", "mysql.session", "mysql.infoschema"}, user.User) {
				newUser := &DatabaseUser{
					ServerID: id,
					Username: user.User,
					Host:     user.Host,
					Remark:   uc.t.Get("sync from server %s", server.Name),
				}
				if err = uc.repo.CreateUser(newUser); err != nil {
					uc.log.Warn(uc.t.Get("failed to sync database users"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(server.ID)), slog.String("name", server.Name), slog.String("server_type", string(server.Type)), slog.String("username", user.User), slog.Any("err", err))
					continue
				}
				synced++
			}
		}
	case DatabaseTypePostgresql:
		allUsers, err := operator.Users(ctx)
		if err != nil {
			return err
		}
		for user := range slices.Values(allUsers) {
			if !slices.ContainsFunc(users, func(a *DatabaseUser) bool {
				return a.Username == user.User
			}) && !slices.Contains([]string{"postgres"}, user.User) {
				newUser := &DatabaseUser{
					ServerID: id,
					Username: user.User,
					Remark:   uc.t.Get("sync from server %s", server.Name),
				}
				if err = uc.repo.CreateUser(newUser); err != nil {
					uc.log.Warn(uc.t.Get("failed to sync database users"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(server.ID)), slog.String("name", server.Name), slog.String("server_type", string(server.Type)), slog.String("username", user.User), slog.Any("err", err))
					continue
				}
				synced++
			}
		}
	case DatabaseTypeClickHouse:
		allUsers, err := operator.Users(ctx)
		if err != nil {
			return err
		}
		for user := range slices.Values(allUsers) {
			if !slices.ContainsFunc(users, func(a *DatabaseUser) bool {
				return a.Username == user.User
			}) && !slices.Contains([]string{"default"}, user.User) {
				newUser := &DatabaseUser{
					ServerID: id,
					Username: user.User,
					Remark:   uc.t.Get("sync from server %s", server.Name),
				}
				if err = uc.repo.CreateUser(newUser); err != nil {
					uc.log.Warn(uc.t.Get("failed to sync database users"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(server.ID)), slog.String("name", server.Name), slog.String("server_type", string(server.Type)), slog.String("username", user.User), slog.Any("err", err))
					continue
				}
				synced++
			}
		}
	default:
	}

	uc.log.Info(uc.t.Get("database server users synced"), slog.String("type", OperationTypeDatabaseServer), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(server.ID)), slog.String("name", server.Name), slog.String("server_type", string(server.Type)), slog.Int("synced", synced))

	return nil
}
