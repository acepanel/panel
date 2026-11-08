package biz

import (
	"context"
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

type DatabaseUserStatus string

const (
	DatabaseUserStatusValid   DatabaseUserStatus = "valid"
	DatabaseUserStatusInvalid DatabaseUserStatus = "invalid"
)

type DatabaseUser struct {
	ID         uint               `gorm:"primaryKey" json:"id"`
	ServerID   uint               `gorm:"not null;default:0" json:"server_id"`
	Username   string             `gorm:"not null;default:''" json:"username"`
	Password   string             `gorm:"not null;default:''" json:"password"`
	Host       string             `gorm:"not null;default:''" json:"host"` // 仅 mysql
	Status     DatabaseUserStatus `gorm:"-:all" json:"status"`             // 仅显示
	Privileges []string           `gorm:"-:all" json:"privileges"`         // 仅显示
	Remark     string             `gorm:"not null;default:''" json:"remark"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`

	Server *DatabaseServer `gorm:"foreignKey:ServerID;references:ID" json:"server"`
}

func (r *DatabaseUser) BeforeSave(tx *gorm.DB) error {
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

func (r *DatabaseUser) AfterFind(tx *gorm.DB) error {
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

type DatabaseUserRepo interface {
	Count() (int64, error)
	List(ctx context.Context, page, limit uint, typ string) ([]*DatabaseUser, int64, error)
	Get(ctx context.Context, id uint) (*DatabaseUser, error)
	UpdateRemark(ctx context.Context, req *request.DatabaseUserUpdateRemark) error
	Operator(ctx context.Context, server *DatabaseServer) (db.Operator, error)
	Upsert(user *DatabaseUser) error
	Save(user *DatabaseUser) error
	DeleteByID(id uint) error
	ListByNames(serverID uint, names []string) ([]*DatabaseUser, error)
	DeleteByServerNames(serverID uint, names []string) error
}

// DatabaseUserUsecase 数据库用户业务用例
type DatabaseUserUsecase struct {
	repo   DatabaseUserRepo
	server DatabaseServerRepo
	t      *gotext.Locale
	log    *slog.Logger
}

func NewDatabaseUserUsecase(t *gotext.Locale, log *slog.Logger, databaseServerRepo DatabaseServerRepo, databaseUserRepo DatabaseUserRepo) *DatabaseUserUsecase {
	return &DatabaseUserUsecase{
		repo:   databaseUserRepo,
		server: databaseServerRepo,
		t:      t,
		log:    log,
	}
}

func (uc *DatabaseUserUsecase) Count() (int64, error) {
	return uc.repo.Count()
}

func (uc *DatabaseUserUsecase) List(ctx context.Context, page, limit uint, typ string) ([]*DatabaseUser, int64, error) {
	return uc.repo.List(ctx, page, limit, typ)
}

func (uc *DatabaseUserUsecase) Get(ctx context.Context, id uint) (*DatabaseUser, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *DatabaseUserUsecase) Create(ctx context.Context, req *request.DatabaseUserCreate) error {
	server, err := uc.server.Get(ctx, req.ServerID)
	if err != nil {
		return err
	}

	operator, err := uc.repo.Operator(ctx, server)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 建用户 → 建库 → 授权 → 落库是一个整体，中途取消会留下没有权限的用户
	ctx = context.WithoutCancel(ctx)

	// 创建用户
	if err = operator.UserCreate(ctx, req.Username, req.Password, req.Host); err != nil {
		return err
	}

	// 创建数据库并授权
	for name := range slices.Values(req.Privileges) {
		if err = operator.DatabaseCreate(ctx, name); err != nil {
			return err
		}
		if err = operator.PrivilegesGrant(ctx, req.Username, name, req.Host); err != nil {
			return err
		}
	}

	user := &DatabaseUser{
		ServerID: req.ServerID,
		Username: req.Username,
		Host:     req.Host,
		Password: req.Password,
		Remark:   req.Remark,
	}

	if err = uc.repo.Upsert(user); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("database user created"), slog.String("type", OperationTypeDatabaseUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(user.ID)), slog.String("username", req.Username), slog.String("host", req.Host), slog.Uint64("server_id", uint64(req.ServerID)), slog.String("server", server.Name), slog.String("server_type", string(server.Type)), slog.Any("privileges", req.Privileges))

	return nil
}

func (uc *DatabaseUserUsecase) Update(ctx context.Context, req *request.DatabaseUserUpdate) error {
	user, err := uc.repo.Get(ctx, req.ID)
	if err != nil {
		return err
	}

	server, err := uc.server.Get(ctx, user.ServerID)
	if err != nil {
		return err
	}

	operator, err := uc.repo.Operator(ctx, server)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 改密码 → 撤权 → 重新授权 → 落库是一个整体，中途取消会让面板记录与实际权限不一致
	ctx = context.WithoutCancel(ctx)

	// 更新密码
	if req.Password != "" {
		if err = operator.UserPassword(ctx, user.Username, req.Password, user.Host); err != nil {
			return err
		}
		user.Password = req.Password
	}

	// 撤销被移除的权限
	currentPrivileges, _ := operator.UserPrivileges(ctx, user.Username, user.Host)
	for name := range slices.Values(currentPrivileges) {
		if !slices.Contains(req.Privileges, name) {
			_ = operator.PrivilegesRevoke(ctx, user.Username, name, user.Host)
		}
	}

	// 创建数据库并授权
	for name := range slices.Values(req.Privileges) {
		if err = operator.DatabaseCreate(ctx, name); err != nil {
			return err
		}
		if err = operator.PrivilegesGrant(ctx, user.Username, name, user.Host); err != nil {
			return err
		}
	}

	user.Remark = req.Remark

	if err = uc.repo.Save(user); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("database user updated"), slog.String("type", OperationTypeDatabaseUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(user.ID)), slog.String("username", user.Username), slog.String("host", user.Host), slog.Uint64("server_id", uint64(user.ServerID)), slog.String("server", server.Name), slog.Bool("password_changed", req.Password != ""), slog.Any("privileges", req.Privileges))

	return nil
}

func (uc *DatabaseUserUsecase) UpdateRemark(ctx context.Context, req *request.DatabaseUserUpdateRemark) error {
	user, err := uc.repo.Get(ctx, req.ID)
	if err != nil {
		return err
	}

	if err = uc.repo.UpdateRemark(ctx, req); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("database user remark updated"), slog.String("type", OperationTypeDatabaseUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(req.ID)), slog.String("username", user.Username), slog.Uint64("server_id", uint64(user.ServerID)), slog.String("remark", req.Remark))

	return nil
}

func (uc *DatabaseUserUsecase) Delete(ctx context.Context, id uint) error {
	user, err := uc.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	server, err := uc.server.Get(ctx, user.ServerID)
	if err != nil {
		return err
	}

	operator, err := uc.repo.Operator(ctx, server)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 删引擎用户 → 删面板记录是一个整体，中途取消会让面板记录已删而引擎里的用户残留
	ctx = context.WithoutCancel(ctx)

	_ = operator.UserDrop(ctx, user.Username, user.Host)

	if err = uc.repo.DeleteByID(id); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("database user deleted"), slog.String("type", OperationTypeDatabaseUser), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(id)), slog.String("username", user.Username), slog.String("host", user.Host), slog.Uint64("server_id", uint64(user.ServerID)), slog.String("server", server.Name))

	return nil
}

func (uc *DatabaseUserUsecase) DeleteByNames(ctx context.Context, serverID uint, names []string) error {
	server, err := uc.server.Get(ctx, serverID)
	if err != nil {
		return err
	}

	operator, err := uc.repo.Operator(ctx, server)
	if err != nil {
		return err
	}
	defer operator.Close()

	ctx = context.WithoutCancel(ctx)

	switch server.Type {
	case DatabaseTypeMysql:
		users, err := uc.repo.ListByNames(serverID, names)
		if err != nil {
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
			_ = operator.UserDrop(ctx, name, host)
		}
	case DatabaseTypePostgresql, DatabaseTypeClickHouse:
		for name := range slices.Values(names) {
			_ = operator.UserDrop(ctx, name)
		}
	default:
	}

	return uc.repo.DeleteByServerNames(serverID, names)
}
