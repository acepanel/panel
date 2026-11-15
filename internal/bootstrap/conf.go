package bootstrap

import (
	"errors"
	"fmt"
	"time"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/config"
)

func NewConf() (*config.Config, error) {
	conf, err := config.Load()
	if err != nil {
		return nil, err
	}

	if err = initGlobal(conf); err != nil {
		return nil, err
	}
	return conf, nil
}

// initGlobal 用配置初始化全局状态
func initGlobal(conf *config.Config) error {
	if len(conf.App.Key) != 32 {
		return errors.New("panel app key must be 32 characters")
	}
	app.Key = conf.App.Key

	app.Root = conf.App.Root
	if app.Root == "" {
		app.Root = "/opt/ace"
	}
	app.Locale = conf.App.Locale

	// 初始化时区
	loc, err := time.LoadLocation(conf.App.Timezone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %w", err)
	}
	time.Local = loc

	return nil
}
