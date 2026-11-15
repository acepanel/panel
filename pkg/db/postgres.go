package db

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	_ "github.com/lib/pq"
)

type Postgres struct {
	db       *sql.DB
	username string
}

func NewPostgres(
	ctx context.Context,
	username, password, address string,
	port uint,
	database ...string,
) (Operator, error) {
	dbname := "postgres"
	if len(database) > 0 && database[0] != "" {
		dbname = database[0]
	}
	if username == "" && password == "" {
		username = "postgres"
	}
	db, err := sql.Open("postgres", postgresDSN(username, password, address, port, dbname))
	if err != nil {
		return nil, fmt.Errorf("init postgres connection failed: %w", err)
	}
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to postgres failed: %w", err)
	}
	return &Postgres{db: db, username: username}, nil
}

func (r *Postgres) Close() {
	_ = r.db.Close()
}

func (r *Postgres) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, query, args...)
}

func (r *Postgres) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r *Postgres) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

func (r *Postgres) DatabaseCreate(ctx context.Context, name string) error {
	// postgres 不支持 CREATE DATABASE IF NOT EXISTS，但是为了保持与 MySQL 一致，先检查数据库是否存在
	exist, err := r.DatabaseExists(ctx, name)
	if err != nil {
		return err
	}
	if exist {
		return nil
	}
	name = strings.ReplaceAll(name, `"`, `""`)
	_, err = r.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, name))
	return err
}

func (r *Postgres) DatabaseDrop(ctx context.Context, name string) error {
	name = strings.ReplaceAll(name, `"`, `""`)
	_, err := r.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, name))
	return err
}

func (r *Postgres) DatabaseExists(ctx context.Context, name string) (bool, error) {
	var count int
	if err := r.QueryRow(ctx, "SELECT COUNT(*) FROM pg_database WHERE datname = $1", name).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Postgres) DatabaseComment(ctx context.Context, name, comment string) error {
	name = strings.ReplaceAll(name, `"`, `""`)
	_, err := r.Exec(ctx, fmt.Sprintf(`COMMENT ON DATABASE "%s" IS '%s'`, name, comment))
	return err
}

func (r *Postgres) UserCreate(ctx context.Context, user, password string, host ...string) error {
	user = strings.ReplaceAll(user, `"`, `""`)
	_, err := r.Exec(ctx, fmt.Sprintf(`CREATE USER "%s" WITH PASSWORD '%s'`, user, password))
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) UserDrop(ctx context.Context, user string, host ...string) error {
	// PostgreSQL 中，如果用户拥有数据库对象或权限，直接 DROP USER 会失败
	// 必须先转移所有权并撤销权限
	user = strings.ReplaceAll(user, `"`, `""`)
	username := strings.ReplaceAll(r.username, `"`, `""`)
	if _, err := r.Exec(ctx, fmt.Sprintf(`REASSIGN OWNED BY "%s" TO "%s"`, user, username)); err != nil {
		return err
	}
	if _, err := r.Exec(ctx, fmt.Sprintf(`DROP OWNED BY "%s"`, user)); err != nil {
		return err
	}
	_, err := r.Exec(ctx, fmt.Sprintf(`DROP USER IF EXISTS "%s"`, user))
	if err != nil {
		return err
	}

	return nil
}

func (r *Postgres) UserPassword(ctx context.Context, user, password string, host ...string) error {
	user = strings.ReplaceAll(user, `"`, `""`)
	_, err := r.Exec(ctx, fmt.Sprintf(`ALTER USER "%s" WITH PASSWORD '%s'`, user, password))
	return err
}

func (r *Postgres) UserPrivileges(ctx context.Context, user string, host ...string) ([]string, error) {
	query := `
        SELECT d.datname
        FROM pg_catalog.pg_database d
        JOIN pg_catalog.pg_roles r ON d.datdba = r.oid
        WHERE r.rolname = $1
        AND d.datistemplate = false
        AND d.datname NOT IN ('template0', 'template1', 'postgres')
        ORDER BY d.datname;
    `

	rows, err := r.Query(ctx, query, user)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var databases []string

	for rows.Next() {
		var dbName string
		if err = rows.Scan(&dbName); err != nil {
			return nil, err
		}
		databases = append(databases, dbName)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (r *Postgres) PrivilegesGrant(ctx context.Context, user, database string, host ...string) error {
	user = strings.ReplaceAll(user, `"`, `""`)
	database = strings.ReplaceAll(database, `"`, `""`)
	if _, err := r.Exec(ctx, fmt.Sprintf(`ALTER DATABASE "%s" OWNER TO "%s"`, database, user)); err != nil {
		return err
	}
	if _, err := r.Exec(ctx, fmt.Sprintf(`GRANT ALL PRIVILEGES ON DATABASE "%s" TO "%s"`, database, user)); err != nil {
		return err
	}

	return nil
}

func (r *Postgres) PrivilegesRevoke(ctx context.Context, user, database string, host ...string) error {
	user = strings.ReplaceAll(user, `"`, `""`)
	database = strings.ReplaceAll(database, `"`, `""`)
	_, err := r.Exec(ctx, fmt.Sprintf(`REVOKE ALL PRIVILEGES ON DATABASE "%s" FROM "%s"`, database, user))
	return err
}

func (r *Postgres) Users(ctx context.Context) ([]User, error) {
	query := `
        SELECT rolname,
               rolsuper,
               rolcreaterole,
               rolcreatedb,
               rolreplication,
               rolbypassrls
        FROM pg_roles
        WHERE rolcanlogin = true;
    `
	rows, err := r.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var users []User
	for rows.Next() {
		var user User
		var super, canCreateRole, canCreateDb, replication, bypassRls bool
		if err = rows.Scan(&user.User, &super, &canCreateRole, &canCreateDb, &replication, &bypassRls); err != nil {
			return nil, err
		}

		for _, perm := range []struct {
			name    string
			enabled bool
		}{
			{"Super", super},
			{"CreateRole", canCreateRole},
			{"CreateDB", canCreateDb},
			{"Replication", replication},
			{"BypassRLS", bypassRls},
		} {
			if perm.enabled {
				user.Grants = append(user.Grants, perm.name)
			}
		}

		if len(user.Grants) == 0 {
			user.Grants = append(user.Grants, "None")
		}

		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *Postgres) Databases(ctx context.Context) ([]Database, error) {
	query := `
        SELECT 
            d.datname, 
            pg_catalog.pg_get_userbyid(d.datdba), 
            pg_catalog.pg_encoding_to_char(d.encoding),
            COALESCE(pg_catalog.shobj_description(d.oid, 'pg_database'), '')
        FROM pg_catalog.pg_database d
        WHERE datistemplate = false;
    `
	rows, err := r.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var databases []Database
	for rows.Next() {
		var db Database
		if err := rows.Scan(&db.Name, &db.Owner, &db.CharSet, &db.Comment); err != nil {
			return nil, err
		}
		if slices.Contains([]string{"template0", "template1", "postgres"}, db.Name) {
			continue
		}
		databases = append(databases, db)
	}

	return databases, rows.Err()
}

func postgresDSN(username, password, address string, port uint, dbname string) string {
	// dsnEscaper 转义 DSN 引号值里的反斜杠和单引号
	dsnEscaper := strings.NewReplacer(`\`, `\\`, `'`, `\'`)

	// connect_timeout 限制建连耗时，避免不可达地址阻塞调用方
	dsn := fmt.Sprintf(`host=%s port=%d user='%s' dbname='%s' sslmode=disable connect_timeout=5`, address, port, dsnEscaper.Replace(username), dsnEscaper.Replace(dbname))
	if password != "" {
		dsn += ` password='` + dsnEscaper.Replace(password) + `'`
	}
	return dsn
}
