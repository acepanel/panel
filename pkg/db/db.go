package db

import (
	"context"
	"database/sql"
)

// Operator 数据库操作句柄
// 所有方法都跟随 ctx 取消，这样 CLI 和停机路径能中断卡在锁等待上的 SQL；
// 需要整体完成的多步操作由调用方（biz 用例层）在事务边界断开取消链
type Operator interface {
	Close()

	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)

	DatabaseCreate(ctx context.Context, name string) error
	DatabaseDrop(ctx context.Context, name string) error
	DatabaseExists(ctx context.Context, name string) (bool, error)

	UserCreate(ctx context.Context, user, password string, host ...string) error
	UserDrop(ctx context.Context, user string, host ...string) error
	UserPassword(ctx context.Context, user, password string, host ...string) error
	UserPrivileges(ctx context.Context, user string, host ...string) ([]string, error)

	PrivilegesGrant(ctx context.Context, user, database string, host ...string) error
	PrivilegesRevoke(ctx context.Context, user, database string, host ...string) error

	Users(ctx context.Context) ([]User, error)
	Databases(ctx context.Context) ([]Database, error)
}
