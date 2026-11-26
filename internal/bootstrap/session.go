package bootstrap

import (
	"log/slog"

	"github.com/libtnb/gormstore"
	"github.com/libtnb/sessions"
	"github.com/libtnb/sessions/driver"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/pkg/config"
)

type sessionStore struct {
	driver.Driver
	manager *sessions.Manager
}

func (s sessionStore) Write(id string, data string) error {
	var attributes map[string]any
	if _, err := s.manager.Codec.Decode(sessions.CookieName, data, &attributes); err == nil && len(attributes) == 0 {
		return s.Destroy(id)
	}

	return s.Driver.Write(id, data)
}

func NewSession(conf *config.Config, db *gorm.DB, log *slog.Logger) (*sessions.Manager, error) {
	// initialize session manager
	manager, err := sessions.NewManager(&sessions.ManagerOptions{
		Key:                  conf.App.Key,
		Lifetime:             int(conf.Session.Lifetime),
		GcInterval:           5,
		DisableDefaultDriver: true,
		Logger:               log,
	})
	if err != nil {
		return nil, err
	}

	// extend gorm store driver
	store := sessionStore{Driver: gormstore.New(db), manager: manager}
	if err = manager.Extend("default", store); err != nil {
		return nil, err
	}

	return manager, nil
}
