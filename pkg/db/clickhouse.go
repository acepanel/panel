package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"

	"resty.dev/v3"
)

// ClickHouse 通过 HTTP API 操作 ClickHouse
type ClickHouse struct {
	client   *resty.Client
	username string
	password string
}

// NewClickHouse 创建 ClickHouse 连接（HTTP API）
func NewClickHouse(ctx context.Context, username, password, address string) (*ClickHouse, error) {
	client := resty.New()
	client.SetBaseURL("http://" + address)
	client.SetTimeout(10 * 1000 * 1000 * 1000) // 10s

	ch := &ClickHouse{
		client:   client,
		username: username,
		password: password,
	}

	// 测试连接
	if _, err := ch.exec(ctx, "SELECT 1"); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to clickhouse failed: %w", err)
	}

	return ch, nil
}

func (r *ClickHouse) Close() {
	_ = r.client.Close()
}

func (r *ClickHouse) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("clickhouse HTTP API does not support sql.Rows")
}

// QueryRow 返回 Scan 时报错的行：*sql.Row 只能由 database/sql 构造，借一个建连必失败的连接器产出
func (r *ClickHouse) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	db := sql.OpenDB(noRowConnector{})
	defer func(db *sql.DB) { _ = db.Close() }(db)
	return db.QueryRowContext(ctx, query)
}

func (r *ClickHouse) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	_, err := r.exec(ctx, query)
	return nil, err
}

func (r *ClickHouse) DatabaseCreate(ctx context.Context, name string) error {
	name = strings.ReplaceAll(name, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", name))
	return err
}

func (r *ClickHouse) DatabaseDrop(ctx context.Context, name string) error {
	name = strings.ReplaceAll(name, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name))
	return err
}

func (r *ClickHouse) DatabaseExists(ctx context.Context, name string) (bool, error) {
	result, err := r.exec(ctx, fmt.Sprintf("SELECT count() FROM system.databases WHERE name = '%s'", name))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(result) != "0", nil
}

func (r *ClickHouse) UserCreate(ctx context.Context, user, password string, host ...string) error {
	user = strings.ReplaceAll(user, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("CREATE USER IF NOT EXISTS `%s` IDENTIFIED BY '%s'", user, password))
	return err
}

func (r *ClickHouse) UserDrop(ctx context.Context, user string, host ...string) error {
	user = strings.ReplaceAll(user, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("DROP USER IF EXISTS `%s`", user))
	return err
}

func (r *ClickHouse) UserPassword(ctx context.Context, user, password string, host ...string) error {
	user = strings.ReplaceAll(user, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("ALTER USER `%s` IDENTIFIED BY '%s'", user, password))
	return err
}

func (r *ClickHouse) UserPrivileges(ctx context.Context, user string, host ...string) ([]string, error) {
	user = strings.ReplaceAll(user, "`", "``")
	result, err := r.exec(ctx, fmt.Sprintf("SHOW GRANTS FOR `%s`", user))
	if err != nil {
		return nil, err
	}

	var databases []string
	for line := range strings.SplitSeq(result, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 解析 GRANT ALL ON database.* TO user 格式
		if idx := strings.Index(line, " ON "); idx != -1 {
			rest := line[idx+4:]
			if before, _, ok := strings.Cut(rest, "."); ok {
				dbName := strings.Trim(before, "`")
				if dbName != "" && dbName != "*" {
					databases = append(databases, dbName)
				}
			}
		}
	}

	return databases, nil
}

func (r *ClickHouse) PrivilegesGrant(ctx context.Context, user, database string, host ...string) error {
	user = strings.ReplaceAll(user, "`", "``")
	database = strings.ReplaceAll(database, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("GRANT ALL ON `%s`.* TO `%s`", database, user))
	return err
}

func (r *ClickHouse) PrivilegesRevoke(ctx context.Context, user, database string, host ...string) error {
	user = strings.ReplaceAll(user, "`", "``")
	database = strings.ReplaceAll(database, "`", "``")
	_, err := r.exec(ctx, fmt.Sprintf("REVOKE ALL ON `%s`.* FROM `%s`", database, user))
	return err
}

func (r *ClickHouse) Users(ctx context.Context) ([]User, error) {
	result, err := r.exec(ctx, "SELECT name FROM system.users FORMAT TabSeparated")
	if err != nil {
		return nil, err
	}

	var users []User
	for line := range strings.SplitSeq(result, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		users = append(users, User{User: line})
	}

	return users, nil
}

func (r *ClickHouse) Databases(ctx context.Context) ([]Database, error) {
	result, err := r.exec(ctx, "SELECT name FROM system.databases WHERE name NOT IN ('system', 'INFORMATION_SCHEMA', 'information_schema') FORMAT TabSeparated")
	if err != nil {
		return nil, err
	}

	var databases []Database
	for line := range strings.SplitSeq(result, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		databases = append(databases, Database{Name: line})
	}

	return databases, nil
}

// exec 通过 HTTP API 执行 SQL
func (r *ClickHouse) exec(ctx context.Context, query string) (string, error) {
	resp, err := r.client.R().
		SetContext(ctx).
		SetQueryParam("user", r.username).
		SetQueryParam("password", r.password).
		SetBody(query).
		Post("/")
	if err != nil {
		return "", fmt.Errorf("clickhouse query failed: %w", err)
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("clickhouse query error: %s", strings.TrimSpace(resp.String()))
	}
	return resp.String(), nil
}

type noRowConnector struct{}

func (noRowConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("clickhouse HTTP API does not support sql.Row")
}

func (noRowConnector) Driver() driver.Driver {
	return nil
}
