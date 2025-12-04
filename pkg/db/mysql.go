package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"slices"

	_ "github.com/go-sql-driver/mysql"
)

type MySQL struct {
	db       *sql.DB
	username string
	password string
	address  string
}

func NewMySQL(username, password, address string, typ ...string) (Operator, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", username, password, address)
	if len(typ) > 0 && typ[0] == "unix" {
		dsn = fmt.Sprintf("%s:%s@unix(%s)/", username, password, address)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("init mysql connection failed: %w", err)
	}
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("connect to mysql failed: %w", err)
	}
	return &MySQL{
		db:       db,
		username: username,
		password: password,
		address:  address,
	}, nil
}

func (r *MySQL) Close() {
	_ = r.db.Close()
}

func (r *MySQL) Ping() error {
	return r.db.Ping()
}

func (r *MySQL) Query(query string, args ...any) (*sql.Rows, error) {
	return r.db.Query(query, args...)
}

func (r *MySQL) QueryRow(query string, args ...any) *sql.Row {
	return r.db.QueryRow(query, args...)
}

func (r *MySQL) Exec(query string, args ...any) (sql.Result, error) {
	return r.db.Exec(query, args...)
}

func (r *MySQL) Prepare(query string) (*sql.Stmt, error) {
	return r.db.Prepare(query)
}

func (r *MySQL) DatabaseCreate(name string) error {
	_, err := r.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", name))
	r.flushPrivileges()
	return err
}

func (r *MySQL) DatabaseDrop(name string) error {
	_, err := r.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", name))
	r.flushPrivileges()
	return err
}

func (r *MySQL) DatabaseExists(name string) (bool, error) {
	rows, err := r.Query("SHOW DATABASES")
	if err != nil {
		return false, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			continue
		}
		if database == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *MySQL) DatabaseSize(name string) (int64, error) {
	var size int64
	err := r.QueryRow(fmt.Sprintf("SELECT COALESCE(SUM(data_length) + SUM(index_length), 0) FROM information_schema.tables WHERE table_schema = '%s'", name)).Scan(&size)
	return size, err
}

func (r *MySQL) UserCreate(user, password string, host ...string) error {
	if len(host) == 0 {
		host = append(host, "%")
	}
	_, err := r.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", user, host[0], password))
	r.flushPrivileges()
	return err
}

func (r *MySQL) UserDrop(user string, host ...string) error {
	if len(host) == 0 {
		host = append(host, "%")
	}
	_, err := r.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s'", user, host[0]))
	r.flushPrivileges()
	return err
}

func (r *MySQL) UserPassword(user, password string, host ...string) error {
	if len(host) == 0 {
		host = append(host, "%")
	}
	_, err := r.Exec(fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", user, host[0], password))
	r.flushPrivileges()
	return err
}

func (r *MySQL) UserPrivileges(user string, host ...string) ([]string, error) {
	if len(host) == 0 {
		host = append(host, "%")
	}

	rows, err := r.Query(fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host[0]))
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

func (r *MySQL) PrivilegesGrant(user, database string, host ...string) error {
	if len(host) == 0 {
		host = append(host, "%")
	}
	_, err := r.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%s'", database, user, host[0]))
	r.flushPrivileges()
	return err
}

func (r *MySQL) PrivilegesRevoke(user, database string, host ...string) error {
	if len(host) == 0 {
		host = append(host, "%")
	}
	_, err := r.Exec(fmt.Sprintf("REVOKE ALL PRIVILEGES ON %s.* FROM '%s'@'%s'", database, user, host[0]))
	r.flushPrivileges()
	return err
}

func (r *MySQL) Users() ([]User, error) {
	rows, err := r.Query("SELECT user, host FROM mysql.user")
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
		grants, err := r.userGrants(user, host)
		if err != nil {
			continue
		}

		users = append(users, User{
			User:   user,
			Host:   host,
			Grants: grants,
		})
	}

	return users, nil
}

func (r *MySQL) Databases() ([]Database, error) {
	query := `
        SELECT 
            SCHEMA_NAME,
            DEFAULT_CHARACTER_SET_NAME,
            DEFAULT_COLLATION_NAME
        FROM INFORMATION_SCHEMA.SCHEMATA
        WHERE SCHEMA_NAME NOT IN ('information_schema', 'performance_schema', 'mysql', 'sys')
    `

	rows, err := r.Query(query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

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

func (r *MySQL) userGrants(user, host string) ([]string, error) {
	rows, err := r.Query(fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host))
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var grants []string
	for rows.Next() {
		var grant string
		if err := rows.Scan(&grant); err != nil {
			continue
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func (r *MySQL) flushPrivileges() {
	_, _ = r.Exec("FLUSH PRIVILEGES")
}
