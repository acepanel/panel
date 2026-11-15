package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// DatabaseCommand 数据库管理命令组
func DatabaseCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "database",
		Usage: t.Get("Database management"),
		Commands: []*cli.Command{
			{
				Name:  "add-server",
				Usage: t.Get("Add database server"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Usage:    t.Get("Server type (mysql, postgresql, mongodb, clickhouse, redis, elasticsearch)"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    t.Get("Server name"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "host",
						Usage:    t.Get("Server address"),
						Required: true,
					},
					&cli.UintFlag{
						Name:     "port",
						Usage:    t.Get("Server port"),
						Required: true,
					},
					&cli.StringFlag{
						Name:  "username",
						Usage: t.Get("Server username"),
					},
					&cli.StringFlag{
						Name:  "password",
						Usage: t.Get("Server password"),
					},
					&cli.StringFlag{
						Name:  "remark",
						Usage: t.Get("Server remark"),
					},
				},
				Action: cliService.DatabaseAddServer,
			},
			{
				Name:  "delete-server",
				Usage: t.Get("Delete database server"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    t.Get("Server name"),
						Aliases:  []string{"n"},
						Required: true,
					},
				},
				Action: cliService.DatabaseDeleteServer,
			},
			{
				Name:   "list-server",
				Usage:  t.Get("List all database servers"),
				Action: cliService.DatabaseListServer,
			},
		},
	}
}
