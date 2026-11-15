package db

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/spf13/cast"
)

// fakeRedisConn 只实现浏览 key 用到的命令，并记下执行过的命令名
type fakeRedisConn struct {
	keys   []string // SCAN 的返回顺序
	values map[string]any
	calls  []string
}

func newFakeRedisConn(values map[string]any, keys ...string) *fakeRedisConn {
	conn := &fakeRedisConn{keys: keys, values: values}
	if conn.values == nil {
		conn.values = make(map[string]any)
	}
	for _, key := range keys {
		if _, ok := conn.values[key]; !ok {
			conn.values[key] = "v"
		}
	}
	return conn
}

func (c *fakeRedisConn) Close() error              { return nil }
func (c *fakeRedisConn) Err() error                { return nil }
func (c *fakeRedisConn) Send(string, ...any) error { return nil }
func (c *fakeRedisConn) Flush() error              { return nil }
func (c *fakeRedisConn) Receive() (any, error)     { return nil, nil }

func (c *fakeRedisConn) Do(command string, args ...any) (any, error) {
	c.calls = append(c.calls, command)

	if command == "SCAN" {
		return c.scan(args), nil
	}
	if command == "DBSIZE" {
		return int64(len(c.keys)), nil
	}
	if command == "OBJECT" || command == "MEMORY" {
		return int64(0), nil
	}

	value := c.values[cast.ToString(args[0])]
	str, isString := value.(string)
	list, _ := value.([]string)
	switch command {
	case "TYPE":
		if isString {
			return "string", nil
		}
		return "list", nil
	case "TTL":
		return int64(-1), nil
	case "STRLEN":
		return int64(len(str)), nil
	case "GET":
		return []byte(str), nil
	case "LLEN":
		return int64(len(list)), nil
	case "LRANGE":
		reply := make([]any, 0, len(list))
		for _, item := range list {
			reply = append(reply, []byte(item))
		}
		return reply, nil
	default:
		return nil, fmt.Errorf("unexpected command %s", command)
	}
}

func (c *fakeRedisConn) scan(args []any) any {
	cursor := cast.ToInt(args[0])
	pattern, count := "*", 10
	for i := 1; i+1 < len(args); i += 2 {
		switch args[i] {
		case "MATCH":
			pattern = cast.ToString(args[i+1])
		case "COUNT":
			count = cast.ToInt(args[i+1])
		}
	}

	end := min(cursor+count, len(c.keys))
	batch := make([]any, 0, count)
	for _, key := range c.keys[cursor:end] {
		if ok, _ := path.Match(pattern, key); ok {
			batch = append(batch, []byte(key))
		}
	}
	if end == len(c.keys) {
		end = 0
	}
	return []any{[]byte(strconv.Itoa(end)), batch}
}

func (c *fakeRedisConn) count(command string) int {
	n := 0
	for _, call := range c.calls {
		if call == command {
			n++
		}
	}
	return n
}

func fakeRedisKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("k%04d", i)
		if i%10 == 0 {
			keys[i] = fmt.Sprintf("user:%04d", i)
		}
	}
	return keys
}

// 不过滤时总数取 DBSIZE，只扫到当前页末尾
func TestRedisSearchStopsAtPageEnd(t *testing.T) {
	conn := newFakeRedisConn(nil, fakeRedisKeys(2500)...)
	r := &Redis{conn: conn}

	items, total, err := r.Search("*", 1, 20)
	must.NoError(t, err)
	check.Equal(t, total, 2500)
	must.Len(t, items, 20)
	check.Equal(t, items[0].Key, "user:0000")
	check.Equal(t, items[19].Key, "k0019")
	check.Equal(t, conn.count("SCAN"), 1)

	conn.calls = nil
	items, total, err = r.Search("", 60, 20)
	must.NoError(t, err)
	check.Equal(t, total, 2500)
	must.Len(t, items, 20)
	check.Equal(t, items[0].Key, "user:1180")
	check.Equal(t, conn.count("SCAN"), 2)

	items, total, err = r.Search("*", 126, 20)
	must.NoError(t, err)
	check.Equal(t, total, 2500)
	check.Len(t, items, 0)
}

// 带过滤时总数只能靠扫完全库得到
func TestRedisSearchWithPattern(t *testing.T) {
	conn := newFakeRedisConn(nil, fakeRedisKeys(2500)...)
	r := &Redis{conn: conn}

	items, total, err := r.Search("user:*", 2, 20)
	must.NoError(t, err)
	check.Equal(t, total, 250)
	must.Len(t, items, 20)
	check.Equal(t, items[0].Key, "user:0200")
	check.Equal(t, conn.count("SCAN"), 3)
	check.Equal(t, conn.count("DBSIZE"), 0)
}

// 超长的值只取长度，不把整个值传回来
func TestRedisLongValueNotFetched(t *testing.T) {
	conn := newFakeRedisConn(map[string]any{
		"big-list":   make([]string, 5001),
		"big-string": strings.Repeat("x", 5001),
		"small-list": []string{"a", "b"},
		"small":      "hello",
	})
	r := &Redis{conn: conn}

	for _, key := range []string{"big-list", "big-string"} {
		kv, err := r.Get(key)
		must.NoError(t, err)
		check.Equal(t, kv.Length, int64(5001))
		check.Equal(t, kv.Value, "data is too long, can't display")
	}
	check.Equal(t, conn.count("LRANGE"), 0)
	check.Equal(t, conn.count("GET"), 0)

	kv, err := r.Get("small-list")
	must.NoError(t, err)
	check.Equal(t, kv.Length, int64(2))
	check.Equal(t, kv.Value, `["a","b"]`)

	kv, err = r.Get("small")
	must.NoError(t, err)
	check.Equal(t, kv.Length, int64(5))
	check.Equal(t, kv.Value, "hello")
}
