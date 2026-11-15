package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// CronCommand 计划任务命令组
func CronCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "cron",
		Usage: t.Get("Cron task"),
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  t.Get("List all cron tasks"),
				Action: cliService.CronList,
			},
			{
				Name:  "run",
				Usage: t.Get("Run a cron task immediately"),
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "id",
						Aliases:  []string{"i"},
						Usage:    t.Get("Cron task ID"),
						Required: true,
					},
				},
				Action: cliService.CronRun,
			},
			{
				Name:  "status",
				Usage: t.Get("Enable or disable a cron task"),
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "id",
						Aliases:  []string{"i"},
						Usage:    t.Get("Cron task ID"),
						Required: true,
					},
					&cli.BoolFlag{
						Name:  "off",
						Usage: t.Get("Disable the task instead of enabling it"),
					},
				},
				Action: cliService.CronStatus,
			},
			{
				Name:  "failed",
				Usage: t.Get("Report a failed cron task, called by the task wrapper script"),
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:     "id",
						Aliases:  []string{"i"},
						Usage:    t.Get("Cron task ID"),
						Required: true,
					},
					&cli.IntFlag{
						Name:    "code",
						Aliases: []string{"c"},
						Usage:   t.Get("Exit code"),
					},
				},
				Action: cliService.CronFailed,
			},
		},
	}
}
