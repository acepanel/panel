package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// BackupCommand 数据备份命令组
func BackupCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "backup",
		Usage: t.Get("Data backup"),
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: t.Get("List backup files"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Usage:    t.Get("Backup type (website, path, panel, mysql, postgresql, clickhouse, redis, valkey)"),
						Required: true,
					},
				},
				Action: cliService.BackupList,
			},
			{
				Name:  "website",
				Usage: t.Get("Backup website"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    t.Get("Website name"),
						Required: true,
					},
					&cli.UintFlag{
						Name:    "storage",
						Aliases: []string{"s"},
						Usage:   t.Get("Storage ID (local storage if not filled)"),
					},
				},
				Action: cliService.BackupWebsite,
			},
			{
				Name:  "database",
				Usage: t.Get("Backup database"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Usage:    t.Get("Database type (mysql, postgresql, clickhouse, redis, valkey)"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    t.Get("Database name"),
						Required: true,
					},
					&cli.UintFlag{
						Name:    "storage",
						Aliases: []string{"s"},
						Usage:   t.Get("Storage ID (local storage if not filled)"),
					},
				},
				Action: cliService.BackupDatabase,
			},
			{
				Name:  "path",
				Usage: t.Get("Backup directory"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "path",
						Aliases:  []string{"p"},
						Usage:    t.Get("Directory path"),
						Required: true,
					},
					&cli.UintFlag{
						Name:    "storage",
						Aliases: []string{"s"},
						Usage:   t.Get("Storage ID (local storage if not filled)"),
					},
				},
				Action: cliService.BackupPath,
			},
			{
				Name:   "panel",
				Usage:  t.Get("Backup panel"),
				Flags:  []cli.Flag{},
				Action: cliService.BackupPanel,
			},
			{
				Name:  "clear",
				Usage: t.Get("Clear backups"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Usage:    t.Get("Backup type (website, path, mysql, postgresql, clickhouse, redis, valkey)"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "file",
						Aliases:  []string{"f"},
						Usage:    t.Get("Backup file prefix"),
						Required: true,
					},
					&cli.UintFlag{
						Name:     "keep",
						Aliases:  []string{"k"},
						Usage:    t.Get("Number of backups to keep"),
						Required: true,
					},
					&cli.UintFlag{
						Name:    "storage",
						Aliases: []string{"s"},
						Usage:   t.Get("Storage ID (local storage if not filled)"),
					},
				},
				Action: cliService.BackupClear,
			},
		},
	}
}
