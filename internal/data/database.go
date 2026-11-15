package data

import (
	"context"
	"fmt"
	"slices"

	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/db"
)

type databaseRepo struct {
	db *gorm.DB
}

func NewDatabaseRepo(db *gorm.DB) biz.DatabaseRepo {
	return &databaseRepo{
		db: db,
	}
}

// ListServers 列出数据库服务器
func (r *databaseRepo) ListServers(typ string) ([]*biz.DatabaseServer, error) {
	var databaseServer []*biz.DatabaseServer
	query := r.db.Model(&biz.DatabaseServer{}).Order("id desc")
	if typ != "" {
		query = query.Where("type = ?", typ)
	}
	if err := query.Find(&databaseServer).Error; err != nil {
		return nil, err
	}
	return databaseServer, nil
}

// DatabasesOf 列出单个服务器上的数据库
func (r *databaseRepo) DatabasesOf(ctx context.Context, server *biz.DatabaseServer) ([]*biz.Database, error) {
	database := make([]*biz.Database, 0)
	switch server.Type {
	case biz.DatabaseTypeMongoDB:
		mongo, err := db.NewMongoDB(ctx, server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
		if err != nil {
			return nil, err
		}
		if databases, err := mongo.Databases(ctx); err == nil {
			for item := range slices.Values(databases) {
				database = append(database, &biz.Database{
					Type:     server.Type,
					Name:     item.Name,
					Server:   server.Name,
					ServerID: server.ID,
				})
			}
		}
		mongo.Close()
	case biz.DatabaseTypeSQLite:
		sqlite, err := db.NewSQLite(ctx, server.Host)
		if err != nil {
			return nil, err
		}
		if tables, err := sqlite.Tables(ctx); err == nil {
			for table := range slices.Values(tables) {
				database = append(database, &biz.Database{
					Type:     server.Type,
					Name:     table,
					Server:   server.Name,
					ServerID: server.ID,
				})
			}
		}
		sqlite.Close()
	default:
		operator, err := r.Operator(ctx, server)
		if err != nil {
			return nil, err
		}
		if databases, err := operator.Databases(ctx); err == nil {
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
	return database, nil
}

func (r *databaseRepo) Operator(ctx context.Context, server *biz.DatabaseServer) (db.Operator, error) {
	return newOperator(ctx, server)
}

// Mongo 构建 MongoDB 客户端
func (r *databaseRepo) Mongo(ctx context.Context, server *biz.DatabaseServer) (*db.MongoDB, error) {
	return db.NewMongoDB(ctx, server.Username, server.Password, fmt.Sprintf("%s:%d", server.Host, server.Port))
}
