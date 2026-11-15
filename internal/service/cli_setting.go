package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

func (s *CliService) GetSetting(ctx context.Context, cmd *cli.Command) error {
	key := cmd.StringArg("key")
	if key == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	setting := new(biz.Setting)
	if err := s.db.Where("key", key).First(setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Setting not exists"))
		}
		return errors.New(s.t.Get("Failed to get setting: %v", err))
	}

	fmt.Print(setting.Value)
	return nil
}

func (s *CliService) WriteSetting(ctx context.Context, cmd *cli.Command) error {
	key := cmd.StringArg("key")
	value := cmd.StringArg("value")
	if key == "" || value == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.settingRepo.Set(biz.SettingKey(key), value); err != nil {
		return errors.New(s.t.Get("Failed to save setting: %v", err))
	}

	return nil
}

func (s *CliService) RemoveSetting(ctx context.Context, cmd *cli.Command) error {
	key := cmd.StringArg("key")
	if key == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.settingRepo.Delete(biz.SettingKey(key)); err != nil {
		return errors.New(s.t.Get("Failed to delete setting: %v", err))
	}

	return nil
}
