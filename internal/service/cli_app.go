package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

func (s *CliService) AppInstall(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.StringArg("slug")
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}
	// 未指定通道时跟随面板通道
	channel := cmd.StringArg("channel")
	if channel == "" {
		channel, _ = s.settingRepo.Get(biz.SettingKeyChannel)
		channel = cmp.Or(channel, "stable")
	}

	if err := s.appRepo.Install(ctx, channel, slug); err != nil {
		return errors.New(s.t.Get("App install failed: %v", err))
	}

	fmt.Println(s.t.Get("App %s installed successfully", slug))
	return nil
}

func (s *CliService) AppUnInstall(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.StringArg("slug")
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.appRepo.UnInstall(ctx, slug); err != nil {
		return errors.New(s.t.Get("App uninstall failed: %v", err))
	}

	fmt.Println(s.t.Get("App %s uninstalled successfully", slug))
	return nil
}

func (s *CliService) AppUpdate(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.StringArg("slug")
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.appRepo.Update(ctx, slug); err != nil {
		return errors.New(s.t.Get("App update failed: %v", err))
	}

	fmt.Println(s.t.Get("App %s updated successfully", slug))
	return nil
}

func (s *CliService) AppList(ctx context.Context, cmd *cli.Command) error {
	apps, err := s.appRepo.Installed()
	if err != nil {
		return errors.New(s.t.Get("Failed to get app list: %v", err))
	}

	return s.printList(cmd, apps, func() {
		for _, item := range apps {
			fmt.Println(s.t.Get("Slug: %s, Channel: %s, Version: %s", item.Slug, item.Channel, item.Version))
		}
	})
}

func (s *CliService) AppWrite(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.StringArg("slug")
	channel := cmd.StringArg("channel")
	version := cmd.StringArg("version")
	if slug == "" || channel == "" || version == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	newApp := new(biz.App)
	if err := s.db.Where("slug", slug).First(newApp).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Failed to get app: %v", err))
		}
	}
	newApp.Slug = slug
	newApp.Channel = channel
	newApp.Version = version
	if err := s.db.Save(newApp).Error; err != nil {
		return errors.New(s.t.Get("Failed to save app: %v", err))
	}

	return nil
}

func (s *CliService) AppRemove(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.StringArg("slug")
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.db.Where("slug", slug).Delete(&biz.App{}).Error; err != nil {
		return errors.New(s.t.Get("Failed to delete app: %v", err))
	}

	return nil
}
