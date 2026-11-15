package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// StatusCommand 查询服务状态
func StatusCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "status",
		Usage:  t.Get("Get AcePanel service status"),
		Action: cliService.Status,
	}
}

// RestartCommand 重启服务
func RestartCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "restart",
		Usage:  t.Get("Restart AcePanel service"),
		Action: cliService.Restart,
	}
}

// StopCommand 停止服务
func StopCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "stop",
		Usage:  t.Get("Stop AcePanel service"),
		Action: cliService.Stop,
	}
}

// StartCommand 启动服务
func StartCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "start",
		Usage:  t.Get("Start AcePanel service"),
		Action: cliService.Start,
	}
}
