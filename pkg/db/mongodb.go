package db

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// MongoDB 通过 mongosh CLI 操作 MongoDB
type MongoDB struct {
	username string
	password string
	address  string // host:port
}

// MongoDatabase MongoDB 数据库信息
type MongoDatabase struct {
	Name       string `json:"name"`
	SizeOnDisk int64  `json:"size_on_disk"`
}

// MongoUser MongoDB 用户信息
type MongoUser struct {
	User  string   `json:"user"`
	DB    string   `json:"db"`
	Roles []string `json:"roles"`
}

// NewMongoDB 创建 MongoDB 连接
func NewMongoDB(ctx context.Context, username, password, address string) (*MongoDB, error) {
	m := &MongoDB{
		username: username,
		password: password,
		address:  address,
	}

	if err := m.Ping(ctx); err != nil {
		return nil, fmt.Errorf("connect to mongodb failed: %w", err)
	}

	return m, nil
}

func (r *MongoDB) Close() {}

func (r *MongoDB) Ping(ctx context.Context) error {
	_, err := r.mongosh(ctx, `db.runCommand({ping:1})`)
	return err
}

// DatabaseCreate 创建数据库（MongoDB 通过创建集合来显式创建数据库）
func (r *MongoDB) DatabaseCreate(ctx context.Context, name string) error {
	_, err := r.mongosh(ctx, `db.getSiblingDB(%s).createCollection('_init')`, name)
	return err
}

// DatabaseDrop 删除数据库
func (r *MongoDB) DatabaseDrop(ctx context.Context, name string) error {
	_, err := r.mongosh(ctx, `db.getSiblingDB(%s).dropDatabase()`, name)
	return err
}

// Databases 获取数据库列表
func (r *MongoDB) Databases(ctx context.Context) ([]MongoDatabase, error) {
	raw, err := r.mongosh(ctx, `JSON.stringify(db.adminCommand({listDatabases:1,nameOnly:false}))`)
	if err != nil {
		return nil, err
	}

	var result struct {
		Databases []struct {
			Name       string `json:"name"`
			SizeOnDisk any    `json:"sizeOnDisk"`
		} `json:"databases"`
	}
	if err = json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to parse databases: %w", err)
	}

	var databases []MongoDatabase
	for _, db := range result.Databases {
		if db.Name == "admin" || db.Name == "config" || db.Name == "local" {
			continue
		}
		databases = append(databases, MongoDatabase{
			Name:       db.Name,
			SizeOnDisk: r.longToInt64(db.SizeOnDisk),
		})
	}

	return databases, nil
}

// UserCreate 创建用户
func (r *MongoDB) UserCreate(ctx context.Context, user, password, database string) error {
	_, err := r.mongosh(
		ctx,
		`db.getSiblingDB(%s).createUser({user:%s,pwd:%s,roles:[{role:'readWrite',db:%s}]})`,
		database,
		user,
		password,
		database,
	)
	return err
}

// UserDrop 删除用户
func (r *MongoDB) UserDrop(ctx context.Context, user, database string) error {
	_, err := r.mongosh(ctx, `db.getSiblingDB(%s).dropUser(%s)`, database, user)
	return err
}

// UserPassword 修改用户密码
func (r *MongoDB) UserPassword(ctx context.Context, user, password string) error {
	_, err := r.mongosh(ctx, `db.getSiblingDB('admin').changeUserPassword(%s,%s)`, user, password)
	return err
}

// Users 获取用户列表
func (r *MongoDB) Users(ctx context.Context) ([]MongoUser, error) {
	raw, err := r.mongosh(ctx, `JSON.stringify(db.getSiblingDB('admin').system.users.find({},{user:1,db:1,roles:1}).toArray())`)
	if err != nil {
		return nil, err
	}

	var result []struct {
		User  string `json:"user"`
		DB    string `json:"db"`
		Roles []struct {
			Role string `json:"role"`
			DB   string `json:"db"`
		} `json:"roles"`
	}
	if err = json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("failed to parse users: %w", err)
	}

	var users []MongoUser
	for _, u := range result {
		var roles []string
		for _, role := range u.Roles {
			roles = append(roles, fmt.Sprintf("%s@%s", role.Role, role.DB))
		}
		users = append(users, MongoUser{
			User:  u.User,
			DB:    u.DB,
			Roles: roles,
		})
	}

	return users, nil
}

// mongosh 执行 mongosh 命令，ctx 取消时终止进程；args 填进 format 的 %s
func (r *MongoDB) mongosh(ctx context.Context, format string, args ...string) (string, error) {
	eval, err := r.script(format, args...)
	if err != nil {
		return "", err
	}

	// serverSelectionTimeoutMS 限制建连耗时，避免不可达地址长时间挂起
	uri := url.URL{
		Scheme:   "mongodb",
		User:     url.UserPassword(r.username, r.password),
		Host:     r.address,
		Path:     "/admin",
		RawQuery: "serverSelectionTimeoutMS=10000",
	}
	raw, err := shell.Execf(ctx, "mongosh --quiet --eval %s %s 2>/dev/null", shell.Quote(eval), shell.Quote(uri.String()))
	if err != nil {
		return "", fmt.Errorf("mongosh error: %w", err)
	}

	return strings.TrimSpace(raw), nil
}

// script 把 args 按 JS 字符串字面量填进 format，名字与密码里的引号、反斜杠不会把脚本截断
func (r *MongoDB) script(format string, args ...string) (string, error) {
	literals := make([]any, 0, len(args))
	for _, arg := range args {
		literal, err := json.Marshal(arg)
		if err != nil {
			return "", err
		}
		literals = append(literals, string(literal))
	}

	return fmt.Sprintf(format, literals...), nil
}

// longToInt64 将 MongoDB Long 对象 {"high":0,"low":8192,"unsigned":false} 转换为 int64
func (r *MongoDB) longToInt64(v any) int64 {
	switch val := v.(type) {
	case float64:
		return int64(val)
	case map[string]any:
		high, _ := val["high"].(float64)
		low, _ := val["low"].(float64)
		// low 是有符号 32 位，最高位为 1 时是负数，要按无符号拼回去
		return int64(high)<<32 | int64(low)&0xFFFFFFFF
	default:
		return 0
	}
}
