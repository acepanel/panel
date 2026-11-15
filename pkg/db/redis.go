package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/samber/lo"
)

type RedisKV struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Type      string    `json:"type"`
	Size      int64     `json:"size"`
	Length    int64     `json:"length"`
	TTL       int64     `json:"ttl"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Redis struct {
	conn redis.Conn
}

func NewRedis(ctx context.Context, username, password, address string) (*Redis, error) {
	// 限制建连与读写超时，避免不可达地址阻塞调用方
	conn, err := redis.DialContext(ctx, "tcp", address,
		redis.DialUsername(username),
		redis.DialPassword(password),
		redis.DialConnectTimeout(5*time.Second),
		redis.DialReadTimeout(10*time.Second),
		redis.DialWriteTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}

	return &Redis{conn: conn}, nil
}

func (r *Redis) Close() {
	_ = r.conn.Close()
}

func (r *Redis) Exec(command string, args ...any) (any, error) {
	return r.conn.Do(command, args...)
}

// Database 获取数据库数量
func (r *Redis) Database() (int, error) {
	values, err := redis.Strings(r.conn.Do("CONFIG", "GET", "databases"))
	if err != nil {
		return 0, err
	}
	if len(values) < 2 {
		return 16, nil // 默认 16 个数据库
	}
	var count int
	if _, err = fmt.Sscanf(values[1], "%d", &count); err != nil {
		return 16, nil //nolint:nilerr
	}
	return count, nil
}

func (r *Redis) Select(db int) error {
	_, err := r.conn.Do("SELECT", db)
	return err
}

// Search 搜索匹配的 key，支持分页
func (r *Redis) Search(pattern string, page, pageSize int) ([]RedisKV, int, error) {
	start := (page - 1) * pageSize
	end := start + pageSize

	// 不过滤时总数直接取 DBSIZE，只需扫到当前页末尾；带过滤时总数只能靠扫完全库得到
	matchAll := pattern == "" || pattern == "*"
	keys, err := r.scanKeys(pattern, lo.Ternary(matchAll, end, 0))
	if err != nil {
		return nil, 0, err
	}
	total := len(keys)
	if matchAll {
		if total, err = redis.Int(r.conn.Do("DBSIZE")); err != nil {
			return nil, 0, err
		}
	}

	if start >= len(keys) {
		return []RedisKV{}, total, nil
	}
	end = min(end, len(keys))

	result := make([]RedisKV, 0, end-start)
	for _, key := range keys[start:end] {
		kv := RedisKV{Key: key}
		r.fillKeyMeta(&kv)
		if kv.Type == "" || kv.Type == "none" {
			continue
		}
		if err = r.fillKeyValue(&kv); err != nil {
			continue
		}
		result = append(result, kv)
	}

	return result, total, nil
}

// Get 获取单个 key 的完整信息
func (r *Redis) Get(key string) (*RedisKV, error) {
	keyType, err := redis.String(r.conn.Do("TYPE", key))
	if err != nil {
		return nil, fmt.Errorf("key not found: %w", err)
	}
	if keyType == "none" {
		return nil, errors.New("key not found")
	}

	kv := &RedisKV{Key: key, Type: keyType}
	r.fillKeyMeta(kv)
	if err = r.fillKeyValue(kv); err != nil {
		return nil, err
	}
	return kv, nil
}

// Del 删除 key
func (r *Redis) Del(keys ...string) error {
	args := lo.Map(keys, func(key string, _ int) any { return key })
	_, err := r.conn.Do("DEL", args...)
	return err
}

// Expire 设置 key 的 TTL，ttl 为 -1 时移除过期时间
func (r *Redis) Expire(key string, ttl int64) error {
	if ttl < 0 {
		_, err := r.conn.Do("PERSIST", key)
		return err
	}
	_, err := r.conn.Do("EXPIRE", key, ttl)
	return err
}

// Rename 重命名 key
func (r *Redis) Rename(oldKey, newKey string) error {
	_, err := r.conn.Do("RENAME", oldKey, newKey)
	return err
}

// SetKey 根据类型创建或设置 key
func (r *Redis) SetKey(key, value, keyType string, ttl int64) error {
	switch keyType {
	case "string":
		if _, err := r.conn.Do("SET", key, value); err != nil {
			return err
		}
	case "list":
		var items []string
		if err := json.Unmarshal([]byte(value), &items); err != nil {
			return fmt.Errorf("list value must be JSON array: %w", err)
		}
		_, _ = r.conn.Do("DEL", key)
		for _, item := range items {
			if _, err := r.conn.Do("RPUSH", key, item); err != nil {
				return err
			}
		}
	case "set":
		var items []string
		if err := json.Unmarshal([]byte(value), &items); err != nil {
			return fmt.Errorf("set value must be JSON array: %w", err)
		}
		_, _ = r.conn.Do("DEL", key)
		for _, item := range items {
			if _, err := r.conn.Do("SADD", key, item); err != nil {
				return err
			}
		}
	case "zset":
		var members map[string]string
		if err := json.Unmarshal([]byte(value), &members); err != nil {
			return fmt.Errorf("zset value must be JSON object {member: score}: %w", err)
		}
		_, _ = r.conn.Do("DEL", key)
		for member, score := range members {
			if _, err := r.conn.Do("ZADD", key, score, member); err != nil {
				return err
			}
		}
	case "hash":
		var fields map[string]string
		if err := json.Unmarshal([]byte(value), &fields); err != nil {
			return fmt.Errorf("hash value must be JSON object: %w", err)
		}
		_, _ = r.conn.Do("DEL", key)
		for field, val := range fields {
			if _, err := r.conn.Do("HSET", key, field, val); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported type: %s", keyType)
	}

	if ttl > 0 {
		if _, err := r.conn.Do("EXPIRE", key, ttl); err != nil {
			return err
		}
	}

	return nil
}

func (r *Redis) Clear() error {
	_, err := r.conn.Do("FLUSHDB")
	return err
}

// scanKeys 用 SCAN 遍历匹配的 key，limit 大于 0 时扫够即停
func (r *Redis) scanKeys(pattern string, limit int) ([]string, error) {
	var keys []string
	cursor := 0
	args := []any{cursor, "COUNT", 1000}
	if pattern != "" && pattern != "*" {
		args = []any{cursor, "MATCH", pattern, "COUNT", 1000}
	}
	for {
		args[0] = cursor
		values, err := redis.Values(r.conn.Do("SCAN", args...))
		if err != nil {
			return nil, fmt.Errorf("failed to SCAN: %w", err)
		}
		var batch []string
		if _, err = redis.Scan(values, &cursor, &batch); err != nil {
			return nil, fmt.Errorf("failed to parse SCAN result: %w", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 || (limit > 0 && len(keys) >= limit) {
			break
		}
	}
	return keys, nil
}

// fillKeyMeta 填充 key 的元信息（类型、TTL、大小）
func (r *Redis) fillKeyMeta(kv *RedisKV) {
	if kv.Type == "" {
		kv.Type, _ = redis.String(r.conn.Do("TYPE", kv.Key))
	}
	ttl, err := redis.Int64(r.conn.Do("TTL", kv.Key))
	if err == nil {
		kv.TTL = ttl
	}
	idleTime, err := redis.Int64(r.conn.Do("OBJECT", "IDLETIME", kv.Key))
	if err == nil {
		kv.UpdatedAt = time.Now().Add(-time.Duration(idleTime) * time.Second)
	} else {
		kv.UpdatedAt = time.Now()
	}
	memory, err := redis.Int64(r.conn.Do("MEMORY", "USAGE", kv.Key))
	if err == nil {
		kv.Size = memory
	}
}

// fillKeyValue 填充 key 的长度和值，先取长度，超长的值不取回
func (r *Redis) fillKeyValue(kv *RedisKV) error {
	// redisLengthCommands 各类型取长度的命令
	redisLengthCommands := map[string]string{
		"string": "STRLEN",
		"list":   "LLEN",
		"set":    "SCARD",
		"zset":   "ZCARD",
		"hash":   "HLEN",
	}

	lengthCommand, ok := redisLengthCommands[kv.Type]
	if !ok {
		return fmt.Errorf("unsupported type: %s", kv.Type)
	}

	var err error
	if kv.Length, err = redis.Int64(r.conn.Do(lengthCommand, kv.Key)); err != nil {
		return err
	}
	if kv.Length > 5000 {
		kv.Value = "data is too long, can't display"
		return nil
	}

	var value any
	switch kv.Type {
	case "string":
		kv.Value, err = redis.String(r.conn.Do("GET", kv.Key))
		return err
	case "list":
		value, err = redis.Strings(r.conn.Do("LRANGE", kv.Key, 0, -1))
	case "set":
		value, err = redis.Strings(r.conn.Do("SMEMBERS", kv.Key))
	case "zset":
		value, err = redis.StringMap(r.conn.Do("ZRANGE", kv.Key, 0, -1, "WITHSCORES"))
	case "hash":
		value, err = redis.StringMap(r.conn.Do("HGETALL", kv.Key))
	}
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	kv.Value = string(encoded)
	return nil
}
