package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLite 通过 database/sql 操作 SQLite 文件
type SQLite struct {
	db *sql.DB
}

// NewSQLite 打开 SQLite 数据库文件
func NewSQLite(ctx context.Context, path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite failed: %w", err)
	}
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to sqlite failed: %w", err)
	}
	return &SQLite{db: db}, nil
}

func (r *SQLite) Close() {
	_ = r.db.Close()
}

// Tables 获取所有表
func (r *SQLite) Tables(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) { _ = rows.Close() }(rows)

	var tables []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}
