package db

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
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
	conn     redis.Conn
	username string
	password string
	address  string
}

func NewRedis(username, password, address string) (*Redis, error) {
	conn, err := redis.Dial("tcp", address, redis.DialUsername(username), redis.DialPassword(password))
	if err != nil {
		return nil, err
	}

	return &Redis{
		conn:     conn,
		username: username,
		password: password,
		address:  address,
	}, nil
}

func (r *Redis) Close() {
	_ = r.conn.Close()
}

func (r *Redis) Exec(command string, args ...any) (any, error) {
	return r.conn.Do(command, args...)
}

func (r *Redis) Database() (int, error) {
	return redis.Int(r.conn.Do("CONFIG", "GET", "databases"))
}

func (r *Redis) Select(db int) error {
	_, err := r.conn.Do("SELECT", db)
	return err
}

func (r *Redis) Size() (int, error) {
	return redis.Int(r.conn.Do("DBSIZE"))
}

func (r *Redis) Data(page, pageSize int) ([]RedisKV, error) {
	result := make([]RedisKV, 0)
	cursor := 0
	var keys []string
	for {
		values, err := redis.Values(r.conn.Do("SCAN", cursor, "COUNT", 100))
		if err != nil {
			return nil, fmt.Errorf("failed to SCAN: %v", err)
		}
		var batch []string
		_, err = redis.Scan(values, &cursor, &batch)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SCAN result: %v", err)
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(keys) {
		return []RedisKV{}, nil
	}
	if end > len(keys) {
		end = len(keys)
	}

	paged := keys[start:end]
	for _, key := range paged {
		kv := RedisKV{
			Key: key,
		}

		keyType, err := redis.String(r.conn.Do("TYPE", key))
		if err != nil {
			continue
		}
		kv.Type = keyType

		ttl, err := redis.Int64(r.conn.Do("TTL", key))
		if err == nil {
			kv.TTL = ttl
		}
		idleTime, err := redis.Int64(r.conn.Do("OBJECT", "IDLETIME", key))
		if err == nil {
			kv.UpdatedAt = time.Now().Add(-time.Duration(idleTime) * time.Second)
		} else {
			kv.UpdatedAt = time.Now()
		}
		memory, err := redis.Int64(r.conn.Do("MEMORY", "USAGE", key))
		if err == nil {
			kv.Size = memory
		}

		var value any
		switch keyType {
		case "string":
			if value, err = redis.String(r.conn.Do("GET", key)); err == nil {
				kv.Length = int64(len(value.(string)))
			}
		case "list":
			if value, err = redis.Strings(r.conn.Do("LRANGE", key, 0, -1)); err == nil {
				kv.Length, _ = redis.Int64(r.conn.Do("LLEN", key))
			}
		case "set":
			if value, err = redis.Strings(r.conn.Do("SMEMBERS", key)); err == nil {
				kv.Length, _ = redis.Int64(r.conn.Do("SCARD", key))
			}
		case "zset":
			if members, err := redis.Strings(r.conn.Do("ZRANGE", key, 0, -1, "WITHSCORES")); err == nil {
				kv.Length, _ = redis.Int64(r.conn.Do("ZCARD", key))
				zsetMap := make(map[string]string)
				for i := 0; i < len(members); i += 2 {
					zsetMap[members[i]] = members[i+1]
				}
				value = zsetMap
			}
		case "hash":
			if value, err = redis.StringMap(r.conn.Do("HGETALL", key)); err == nil {
				kv.Length, _ = redis.Int64(r.conn.Do("HLEN", key))
			}
		default:
			continue
		}

		if err != nil {
			continue
		}
		if kv.Length > 500 {
			value = "data is too long, can't display"
		}

		if str, ok := value.(string); ok {
			kv.Value = str
		} else {
			encoded, err := json.Marshal(value)
			if err != nil {
				continue
			}
			kv.Value = string(encoded)
		}

		result = append(result, kv)
	}

	return result, nil
}

func (r *Redis) Clear() error {
	_, err := r.conn.Do("FLUSHDB")
	return err
}
