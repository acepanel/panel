package service

import (
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestPendingStoreTakesOnce(t *testing.T) {
	store := newPendingStore[string]()
	id, ok := store.Put("value")
	must.True(t, ok)

	value, ok := store.Take(id)
	check.True(t, ok)
	check.Equal(t, value, "value")

	_, ok = store.Take(id)
	check.False(t, ok)
	_, ok = store.Take("")
	check.False(t, ok)
}

func TestPendingStoreExpires(t *testing.T) {
	store := newPendingStore[string]()
	id, ok := store.Put("value")
	must.True(t, ok)

	entry := store.entries[id]
	entry.expires = time.Now().Add(-time.Second)
	store.entries[id] = entry

	_, ok = store.Take(id)
	check.False(t, ok)
}

func TestPendingStoreLimit(t *testing.T) {
	store := newPendingStore[struct{}]()
	for range pendingLimit {
		_, ok := store.Put(struct{}{})
		must.True(t, ok)
	}
	_, ok := store.Put(struct{}{})
	check.False(t, ok)

	// 过期项在下次存入时清掉，腾出位置
	for id, entry := range store.entries {
		entry.expires = time.Now().Add(-time.Second)
		store.entries[id] = entry
	}
	_, ok = store.Put(struct{}{})
	check.True(t, ok)
	check.Equal(t, len(store.entries), 1)
}
