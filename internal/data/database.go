package data

import (
	"context"
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

type databaseRepo struct {
	t      *gotext.Locale
	db     *gorm.DB
	log    *slog.Logger
	server biz.DatabaseServerRepo
	user   biz.DatabaseUserRepo
}

func NewDatabaseRepo(t *gotext.Locale, db *gorm.DB, log *slog.Logger, server biz.DatabaseServerRepo, user biz.DatabaseUserRepo) biz.DatabaseRepo {
	return &databaseRepo{
		t:      t,
		db:     db,
		log:    log,
		server: server,
		user:   user,
	}
}

func (r *databaseRepo) List(page, limit uint) ([]*biz.Database, int64, error) {
	var databaseServer []*biz.DatabaseServer
	if err := r.db.Model(&biz.DatabaseServer{}).Order("id desc").Find(&databaseServer).Error; err != nil {
		return nil, 0, err
	}

	database := make([]*biz.Database, 0)
	for _, server := range databaseServer {
		operator, err := r.getOperator(server)
		if err != nil {
			continue
		}

		if databases, err := operator.Databases(); err == nil {
			for item := range slices.Values(databases) {
				database = append(database, &biz.Database{
					Type:     server.Type,
					Name:     item.Name,
					Server:   server.Name,
					ServerID: server.ID,
					Encoding: item.CharSet,
					Comment:  item.Comment,
				})
			}
		}

		operator.Close()
	}

	if len(database) < int((page-1)*limit) {
		return []*biz.Database{}, int64(len(database)), nil
	}

	return database[(page-1)*limit:], int64(len(database)), nil
}

func (r *databaseRepo) Create(ctx context.Context, req *request.DatabaseCreate) error {
	server, err := r.server.Get(req.ServerID)
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
		if req.CreateUser {
			if err = r.user.Create(ctx, &request.DatabaseUserCreate{
				ServerID: req.ServerID,
				Username: req.Username,
				Password: req.Password,
				Host:     req.Host,
			}); err != nil {
				return err
			}
		}
		if err = operator.DatabaseCreate(req.Name); err != nil {
			return err
		}
		if req.Username != "" {
			if err = operator.PrivilegesGrant(req.Username, req.Name, req.Host); err != nil {
				return err
			}
		}
	case biz.DatabaseTypePostgresql:
		if req.CreateUser {
			if err = r.user.Create(ctx, &request.DatabaseUserCreate{
				ServerID: req.ServerID,
				Username: req.Username,
				Password: req.Password,
				Host:     req.Host,
			}); err != nil {
				return err
			}
		}
		if err = operator.DatabaseCreate(req.Name); err != nil {
			return err
		}
		if req.Username != "" {
			if err = operator.PrivilegesGrant(req.Username, req.Name); err != nil {
				return err
			}
		}
		if err = operator.(*db.Postgres).DatabaseComment(req.Name, req.Comment); err != nil {
			return err
		}
	}

	// 记录日志
	r.log.Info("database created", slog.String("type", biz.OperationTypeDatabase), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("name", req.Name), slog.Uint64("server_id", uint64(req.ServerID)))

	return nil
}

func (r *databaseRepo) Delete(ctx context.Context, serverID uint, name string) error {
	server, err := r.server.Get(serverID)
	if err != nil {
		return err
	}

	operator, err := r.getOperator(server)
	if err != nil {
		return err
	}
	defer operator.Close()

	if err = operator.DatabaseDrop(name); err != nil {
		return err
	}

	// 记录日志
	r.log.Info("database deleted", slog.String("type", biz.OperationTypeDatabase), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("name", name), slog.Uint64("server_id", uint64(serverID)))

	return nil
}

func (r *databaseRepo) Comment(req *request.DatabaseComment) error {
	server, err := r.server.Get(req.ServerID)
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
		return errors.New(r.t.Get("mysql not support database comment"))
	case biz.DatabaseTypePostgresql:
		return operator.(*db.Postgres).DatabaseComment(req.Name, req.Comment)
	}

	return nil
}

func (r *databaseRepo) getOperator(server *biz.DatabaseServer) (db.Operator, error) {
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
