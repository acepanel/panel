package service

import (
	"sync"
	"time"

	"github.com/libtnb/utils/str"
)

const (
	// pendingTTL 暂存项的有效期
	pendingTTL = 5 * time.Minute
	// pendingLimit 暂存项的数量上限
	pendingLimit = 10000
)

type pendingEntry[T any] struct {
	value   T
	expires time.Time
}

// pendingStore 登录过程中一次性使用的内存暂存区
type pendingStore[T any] struct {
	mu      sync.Mutex
	entries map[string]pendingEntry[T]
}

func newPendingStore[T any]() *pendingStore[T] {
	return &pendingStore[T]{entries: make(map[string]pendingEntry[T])}
}

// Put 存入一项并返回取回它的句柄，存满时返回 false
func (s *pendingStore[T]) Put(value T) (string, bool) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.entries {
		if now.After(entry.expires) {
			delete(s.entries, id)
		}
	}
	if len(s.entries) >= pendingLimit {
		return "", false
	}

	id := str.Random(16)
	s.entries[id] = pendingEntry[T]{value: value, expires: now.Add(pendingTTL)}
	return id, true
}

// Take 取出并删除一项，不存在或已过期时返回 false
func (s *pendingStore[T]) Take(id string) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[id]
	delete(s.entries, id)
	if !ok || time.Now().After(entry.expires) {
		var zero T
		return zero, false
	}

	return entry.value, true
}
