package bootstrap

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sessions"
	"github.com/libtnb/sessions/driver"
)

type recordingStore struct {
	driver.Driver
	data map[string]string
}

func (s *recordingStore) Destroy(id string) error {
	delete(s.data, id)
	return nil
}

func (s *recordingStore) Write(id string, data string) error {
	s.data[id] = data
	return nil
}

func TestSessionStoreSkipsEmptySession(t *testing.T) {
	manager, err := sessions.NewManager(&sessions.ManagerOptions{
		Key:                  "0123456789abcdef0123456789abcdef",
		DisableDefaultDriver: true,
	})
	must.NoError(t, err)

	inner := &recordingStore{data: map[string]string{}}
	store := sessionStore{Driver: inner, manager: manager}

	empty, err := manager.Codec.Encode(sessions.CookieName, map[string]any{})
	must.NoError(t, err)
	filled, err := manager.Codec.Encode(sessions.CookieName, map[string]any{"verify_entrance": true})
	must.NoError(t, err)

	must.NoError(t, store.Write("anonymous", empty))
	check.Equal(t, len(inner.data), 0)

	must.NoError(t, store.Write("visitor", filled))
	check.Equal(t, inner.data["visitor"], filled)

	// 数据被清空的会话连同已有记录一并删除
	must.NoError(t, store.Write("visitor", empty))
	check.Equal(t, len(inner.data), 0)
}
