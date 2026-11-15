package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"slices"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	db *sql.DB
}

func NewMySQL(ctx context.Context, username, password, address string, typ ...string) (Operator, error) {
	// 限制建连与读写超时，避免不可达地址阻塞调用方
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/?timeout=5s&readTimeout=10s&writeTimeout=10s", username, password, address)
	if len(typ) > 0 && typ[0] == "unix" {
		dsn = fmt.Sprintf("%s:%s@unix(%s)/?timeout=5s&readTimeout=10s&writeTimeout=10s", username, password, address)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("init mysql connection failed: %w", err)
	}
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to mysql failed: %w", err)
	}
	return &MySQL{db: db}, nil
}

func (r *MySQL) Close() {
	_ = r.db.Close()
}

func (r *MySQL) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}

func (r *MySQL) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *MySQL) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

func (r *MySQL) DatabaseCreate(ctx context.Context, name string) error {
	name = strings.ReplaceAll(name, "`", "``")
	_, err := r.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", name))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) DatabaseDrop(ctx context.Context, name string) error {
	name = strings.ReplaceAll(name, "`", "``")
	_, err := r.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) DatabaseExists(ctx context.Context, name string) (bool, error) {
	rows, err := r.Query(ctx, "SHOW DATABASES")
	if err != nil {
		return false, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			continue
		}
		if database == name {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (r *MySQL) UserCreate(ctx context.Context, user, password string, host ...string) error {
	if len(host) == 0 || host[0] == "" {
		host = []string{"%"}
	}
	_, err := r.Exec(ctx, fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", user, host[0], password))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) UserDrop(ctx context.Context, user string, host ...string) error {
	if len(host) == 0 || host[0] == "" {
		host = []string{"%"}
	}
	_, err := r.Exec(ctx, fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s'", user, host[0]))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) UserPassword(ctx context.Context, user, password string, host ...string) error {
	if len(host) == 0 || host[0] == "" {
		host = []string{"%"}
	}
	_, err := r.Exec(ctx, fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", user, host[0], password))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) UserPrivileges(ctx context.Context, user string, host ...string) ([]string, error) {
	if len(host) == 0 || host[0] == "" {
		host = []string{"%"}
	}

	rows, err := r.Query(ctx, fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host[0]))
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	re := regexp.MustCompile(`GRANT\s+ALL PRIVILEGES\s+ON\s+[\x60'"]?([^\s\x60'"]+)[\x60'"]?\.\*\s+TO\s+`)
	var databases []string
	for rows.Next() {
		var grant string
		if err = rows.Scan(&grant); err != nil {
			return nil, err
		}

		// 使用正则表达式匹配
		matches := re.FindStringSubmatch(grant)
		if len(matches) == 2 {
			dbName := matches[1]
			if dbName != "*" {
				databases = append(databases, dbName)
			}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	slices.Sort(databases)
	return slices.Compact(databases), nil
}

func (r *MySQL) PrivilegesGrant(ctx context.Context, user, database string, host ...string) error {
	if len(host) == 0 || host[0] == "" {
		host = []string{"%"}
	}
	database = strings.ReplaceAll(database, "`", "``")
	_, err := r.Exec(ctx, fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s'", database, user, host[0]))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) PrivilegesRevoke(ctx context.Context, user, database string, host ...string) error {
	if len(host) == 0 || host[0] == "" {
		host = []string{"%"}
	}
	database = strings.ReplaceAll(database, "`", "``")
	_, err := r.Exec(ctx, fmt.Sprintf("REVOKE ALL PRIVILEGES ON `%s`.* FROM '%s'@'%s'", database, user, host[0]))
	r.flushPrivileges(ctx)
	return err
}

func (r *MySQL) Users(ctx context.Context) ([]User, error) {
	rows, err := r.Query(ctx, "SELECT user, host FROM mysql.user")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var users []User
	for rows.Next() {
		var user, host string
		if err := rows.Scan(&user, &host); err != nil {
			continue
		}
		grants, err := r.userGrants(ctx, user, host)
		if err != nil {
			continue
		}

		users = append(users, User{
			User:   user,
			Host:   host,
			Grants: grants,
		})
	}

	return users, rows.Err()
}

func (r *MySQL) Databases(ctx context.Context) ([]Database, error) {
	query := `
        SELECT 
            SCHEMA_NAME,
            DEFAULT_CHARACTER_SET_NAME,
            DEFAULT_COLLATION_NAME
        FROM INFORMATION_SCHEMA.SCHEMATA
        WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
    `

	rows, err := r.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var databases []Database
	for rows.Next() {
		var db Database
		if err = rows.Scan(&db.Name, &db.CharSet, &db.Collation); err != nil {
			return nil, err
		}
		databases = append(databases, db)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (r *MySQL) userGrants(ctx context.Context, user, host string) ([]string, error) {
	rows, err := r.Query(ctx, fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host))
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var grants []string
	for rows.Next() {
		var grant string
		if err := rows.Scan(&grant); err != nil {
			continue
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (r *MySQL) flushPrivileges(ctx context.Context) {
	_, _ = r.Exec(ctx, "FLUSH PRIVILEGES")
}
