package service

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/webserver"
)

// ReloadWebserver 重载当前 Web 服务器，未运行时忽略，供安装脚本调用
func (s *CliService) ReloadWebserver(ctx context.Context, cmd *cli.Command) error {
	webServer, err := s.settingRepo.Get(biz.SettingKeyWebserver)
	if err != nil {
		return err
	}
	d, err := webserver.Get(webserver.Type(webServer))
	if err != nil {
		return err
	}

	return d.ReloadIfRunning(ctx)
}
