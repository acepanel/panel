package biz

import (
	"context"

	"github.com/acepanel/panel/internal/http/request"
)

type DatabaseType string

const (
	DatabaseTypeMysql      DatabaseType = "mysql"
	DatabaseTypePostgresql DatabaseType = "postgresql"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
	DatabaseSQLite         DatabaseType = "sqlite"
	DatabaseTypeRedis      DatabaseType = "redis"
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
	List(page, limit uint) ([]*Database, int64, error)
	Create(ctx context.Context, req *request.DatabaseCreate) error
	Delete(ctx context.Context, serverID uint, name string) error
	Comment(req *request.DatabaseComment) error
}
