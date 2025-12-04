package db

import "database/sql"

type Operator interface {
	Close()
	Ping() error

	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)

	DatabaseCreate(name string) error
	DatabaseDrop(name string) error
	DatabaseExists(name string) (bool, error)
	DatabaseSize(name string) (int64, error)

	UserCreate(user, password string, host ...string) error
	UserDrop(user string, host ...string) error
	UserPassword(user, password string, host ...string) error
	UserPrivileges(user string, host ...string) ([]string, error)

	PrivilegesGrant(user, database string, host ...string) error
	PrivilegesRevoke(user, database string, host ...string) error

	Users() ([]User, error)
	Databases() ([]Database, error)
}
