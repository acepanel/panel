package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// UpdateCommand 更新面板
func UpdateCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "update",
		Usage:  t.Get("Update AcePanel to the latest version"),
		Action: cliService.Update,
	}
}

// SyncCommand 同步云端缓存数据
func SyncCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "sync",
		Usage:  t.Get("Sync AcePanel cached data with cloud"),
		Action: cliService.Sync,
	}
}

// FixCommand 修复升级问题
func FixCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "fix",
		Usage:  t.Get("Fix AcePanel upgrade issues"),
		Action: cliService.Fix,
	}
}

// InfoCommand 输出面板基础信息
func InfoCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "info",
		Usage: t.Get("Output AcePanel basic information"),
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   t.Get("Force reset password"),
			},
			&cli.StringFlag{
				Name:    "username",
				Aliases: []string{"u"},
				Usage:   t.Get("Target username (the first user if not filled)"),
			},
		},
		Action: cliService.Info,
	}
}

// PortCommand 修改监听端口
func PortCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "port",
		Usage: t.Get("Change the AcePanel listening port"),
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:     "port",
				Aliases:  []string{"p"},
				Usage:    t.Get("Listening port"),
				Required: true,
			},
		},
		Action: cliService.Port,
	}
}

// SyncTimeCommand 通过 NTP 同步系统时间
func SyncTimeCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "sync-time",
		Usage:  t.Get("Sync server time with NTP"),
		Action: cliService.SyncTime,
	}
}

// ClearTaskCommand 清理卡住的任务队列
func ClearTaskCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "clear-task",
		Usage:  t.Get("Clear all tasks in the task queue if they are stuck (use only under guidance)"),
		Hidden: true,
		Action: cliService.ClearTask,
	}
}

// InitCommand 初始化面板
func InitCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "init",
		Usage:  t.Get("Initialize AcePanel (use only under guidance)"),
		Hidden: true,
		Action: cliService.Init,
	}
}
