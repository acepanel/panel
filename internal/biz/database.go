package biz

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/db"
)

type DatabaseType string

const (
	DatabaseTypeMysql         DatabaseType = "mysql"
	DatabaseTypePostgresql    DatabaseType = "postgresql"
	DatabaseTypeMongoDB       DatabaseType = "mongodb"
	DatabaseTypeClickHouse    DatabaseType = "clickhouse"
	DatabaseTypeSQLite        DatabaseType = "sqlite"
	DatabaseTypeRedis         DatabaseType = "redis"
	DatabaseTypeElasticsearch DatabaseType = "elasticsearch"
)

type Database struct {
	Type     DatabaseType `json:"type"`
	Name     string       `json:"name"`
	Server   string       `json:"server"`
	ServerID uint         `json:"server_id"`
	Encoding string       `json:"encoding"`
	Comment  string       `json:"comment"`
}

type DatabaseRepo interface {
	ListServers(typ string) ([]*DatabaseServer, error)
	DatabasesOf(ctx context.Context, server *DatabaseServer) ([]*Database, error)
	Operator(ctx context.Context, server *DatabaseServer) (db.Operator, error)
	Mongo(ctx context.Context, server *DatabaseServer) (*db.MongoDB, error)
}

// DatabaseUsecase 数据库业务用例
type DatabaseUsecase struct {
	repo   DatabaseRepo
	server DatabaseServerRepo
	user   *DatabaseUserUsecase
	t      *gotext.Locale
	log    *slog.Logger
}

func NewDatabaseUsecase(databaseUserUsecase *DatabaseUserUsecase, t *gotext.Locale, log *slog.Logger, databaseRepo DatabaseRepo, databaseServerRepo DatabaseServerRepo) *DatabaseUsecase {
	return &DatabaseUsecase{
		repo:   databaseRepo,
		server: databaseServerRepo,
		user:   databaseUserUsecase,
		t:      t,
		log:    log,
	}
}

// List 按名称或备注关键字筛选数据库，关键字为空时返回全部
func (uc *DatabaseUsecase) List(ctx context.Context, page, limit uint, typ, keyword string) ([]*Database, int64, error) {
	servers, err := uc.repo.ListServers(typ)
	if err != nil {
		return nil, 0, err
	}

	keyword = strings.ToLower(keyword)
	database := make([]*Database, 0)
	for _, server := range servers {
		databases, err := uc.repo.DatabasesOf(ctx, server)
		if err != nil {
			continue
		}
		for _, item := range databases {
			if strings.Contains(strings.ToLower(item.Name), keyword) || strings.Contains(strings.ToLower(item.Comment), keyword) {
				database = append(database, item)
			}
		}
	}

	start := min(int((page-1)*limit), len(database))
	end := min(start+int(limit), len(database))
	return database[start:end], int64(len(database)), nil
}

func (uc *DatabaseUsecase) Create(ctx context.Context, req *request.DatabaseCreate) error {
	server, err := uc.server.Get(ctx, req.ServerID)
	if err != nil {
		return err
	}

	// MongoDB 独立处理，不走 Operator 接口
	if server.Type == DatabaseTypeMongoDB {
		mongo, mongoErr := uc.repo.Mongo(ctx, server)
		if mongoErr != nil {
			return mongoErr
		}
		defer mongo.Close()
		if mongoErr = mongo.DatabaseCreate(ctx, req.Name); mongoErr != nil {
			return mongoErr
		}
		uc.log.Info(uc.t.Get("database created"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.Uint64("server_id", uint64(req.ServerID)), slog.String("server", server.Name), slog.String("server_type", string(server.Type)))
		return nil
	}

	operator, err := uc.repo.Operator(ctx, server)
	if err != nil {
		return err
	}
	defer operator.Close()

	// 建用户 → 建库 → 授权是一个整体，中途取消会留下空库或没有权限的用户
	ctx = context.WithoutCancel(ctx)

	switch server.Type {
	case DatabaseTypeMysql:
		if req.CreateUser {
			if err = uc.user.Create(ctx, &request.DatabaseUserCreate{
				ServerID: req.ServerID,
				Username: req.Username,
				Password: req.Password,
				Host:     req.Host,
			}); err != nil {
				return err
			}
		}
		if err = operator.DatabaseCreate(ctx, req.Name); err != nil {
			return err
		}
		if req.Username != "" {
			// 授权已有用户时按其实际 host 授权，MySQL 8 起 GRANT 不再隐式创建用户
			hosts := []string{req.Host}
			if !req.CreateUser {
				if hosts = uc.mysqlUserHosts(ctx, operator, req.Username); len(hosts) == 0 {
					return errors.New(uc.t.Get("database user %s does not exist", req.Username))
				}
			}
			for _, host := range hosts {
				if err = operator.PrivilegesGrant(ctx, req.Username, req.Name, host); err != nil {
					return err
				}
			}
		}
	case DatabaseTypePostgresql:
		if req.CreateUser {
			if err = uc.user.Create(ctx, &request.DatabaseUserCreate{
				ServerID: req.ServerID,
				Username: req.Username,
				Password: req.Password,
				Host:     req.Host,
			}); err != nil {
				return err
			}
		}
		if err = operator.DatabaseCreate(ctx, req.Name); err != nil {
			return err
		}
		if req.Username != "" {
			if err = operator.PrivilegesGrant(ctx, req.Username, req.Name); err != nil {
				return err
			}
		}
		pg, ok := operator.(*db.Postgres)
		if !ok {
			return errors.New(uc.t.Get("%s does not support database comment", server.Type))
		}
		if err = pg.DatabaseComment(ctx, req.Name, req.Comment); err != nil {
			return err
		}
	case DatabaseTypeClickHouse:
		if req.CreateUser {
			if err = uc.user.Create(ctx, &request.DatabaseUserCreate{
				ServerID: req.ServerID,
				Username: req.Username,
				Password: req.Password,
			}); err != nil {
				return err
			}
		}
		if err = operator.DatabaseCreate(ctx, req.Name); err != nil {
			return err
		}
		if req.Username != "" {
			if err = operator.PrivilegesGrant(ctx, req.Username, req.Name); err != nil {
				return err
			}
		}
	default:
	}

	uc.log.Info(uc.t.Get("database created"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.Uint64("server_id", uint64(req.ServerID)), slog.String("server", server.Name), slog.String("server_type", string(server.Type)), slog.String("username", req.Username))

	return nil
}

// mysqlUserHosts 查询 MySQL 用户实际存在的 host 列表
func (uc *DatabaseUsecase) mysqlUserHosts(ctx context.Context, operator db.Operator, user string) []string {
	rows, err := operator.Query(ctx, "SELECT host FROM mysql.user WHERE user = ?", user)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var hosts []string
	for rows.Next() {
		var host string
		if rows.Scan(&host) == nil {
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func (uc *DatabaseUsecase) Delete(ctx context.Context, serverID uint, name string) error {
	server, err := uc.server.Get(ctx, serverID)
	if err != nil {
		return err
	}

	// 删库是逐表进行的，中途取消会留下半删的 schema，而面板已经把它当作已删除
	ctx = context.WithoutCancel(ctx)

	switch server.Type {
	case DatabaseTypeMongoDB:
		mongo, mongoErr := uc.repo.Mongo(ctx, server)
		if mongoErr != nil {
			return mongoErr
		}
		defer mongo.Close()
		if mongoErr = mongo.DatabaseDrop(ctx, name); mongoErr != nil {
			return mongoErr
		}
	case DatabaseTypeSQLite:
		return errors.New(uc.t.Get("sqlite does not support dropping tables from here"))
	default:
		operator, opErr := uc.repo.Operator(ctx, server)
		if opErr != nil {
			return opErr
		}
		defer operator.Close()
		if opErr = operator.DatabaseDrop(ctx, name); opErr != nil {
			return opErr
		}
	}

	uc.log.Warn(uc.t.Get("database deleted"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", name), slog.Uint64("server_id", uint64(serverID)), slog.String("server", server.Name), slog.String("server_type", string(server.Type)))

	return nil
}

func (uc *DatabaseUsecase) Comment(ctx context.Context, req *request.DatabaseComment) error {
	server, err := uc.server.Get(ctx, req.ServerID)
	if err != nil {
		return err
	}

	switch server.Type {
	case DatabaseTypePostgresql:
		operator, opErr := uc.repo.Operator(ctx, server)
		if opErr != nil {
			return opErr
		}
		defer operator.Close()
		pg, ok := operator.(*db.Postgres)
		if !ok {
			return errors.New(uc.t.Get("%s does not support database comment", server.Type))
		}
		if err = pg.DatabaseComment(ctx, req.Name, req.Comment); err != nil {
			return err
		}
		uc.log.Info(uc.t.Get("database comment updated"), slog.String("type", OperationTypeDatabase), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.Uint64("server_id", uint64(req.ServerID)), slog.String("server", server.Name), slog.String("server_type", string(server.Type)), slog.String("comment", req.Comment))
		return nil
	default:
		return errors.New(uc.t.Get("%s does not support database comment", server.Type))
	}
}
