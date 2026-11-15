package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// WebsiteCommand 网站管理命令组
func WebsiteCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "website",
		Usage: t.Get("Website management"),
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: t.Get("Create new website"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "type",
						Usage:   t.Get("Website type (proxy, static, php)"),
						Aliases: []string{"t"},
						Value:   "static",
					},
					&cli.StringFlag{
						Name:     "name",
						Usage:    t.Get("Website name"),
						Aliases:  []string{"n"},
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:     "domains",
						Usage:    t.Get("List of domains associated with the website"),
						Aliases:  []string{"d"},
						Required: true,
					},
					&cli.StringSliceFlag{
						Name:    "listens",
						Usage:   t.Get("List of listening addresses associated with the website"),
						Aliases: []string{"l"},
						Value:   []string{"80"},
					},
					&cli.StringFlag{
						Name:    "path",
						Usage:   t.Get("Path where the website is hosted (default path if not filled)"),
						Aliases: []string{"p"},
					},
					&cli.StringFlag{
						Name:  "proxy",
						Usage: t.Get("Address to proxy to (required for proxy type)"),
					},
					&cli.UintFlag{
						Name:  "php",
						Usage: t.Get("PHP version used by the website (not used if not filled, php type only)"),
					},
					&cli.StringFlag{
						Name:  "db",
						Usage: t.Get("Create a database of the given type (mysql, postgresql), not created if not filled"),
					},
					&cli.StringFlag{
						Name:  "db-name",
						Usage: t.Get("Database name (website name if not filled)"),
					},
					&cli.StringFlag{
						Name:  "db-user",
						Usage: t.Get("Database username (website name if not filled)"),
					},
					&cli.StringFlag{
						Name:  "db-password",
						Usage: t.Get("Database password (randomly generated if not filled)"),
					},
					&cli.StringFlag{
						Name:  "remark",
						Usage: t.Get("Website remark"),
					},
				},
				Action: cliService.WebsiteCreate,
			},
			{
				Name:   "rebuild",
				Usage:  t.Get("Rebuild all website configs for the current web server"),
				Action: cliService.WebsiteRebuild,
			},
			{
				Name:  "remove",
				Usage: t.Get("Remove website (keep website directory and database)"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    t.Get("Website name"),
						Aliases:  []string{"n"},
						Required: true,
					},
				},
				Action: cliService.WebsiteRemove,
			},
			{
				Name:  "delete",
				Usage: t.Get("Delete website (including website directory, database with the same name)"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    t.Get("Website name"),
						Aliases:  []string{"n"},
						Required: true,
					},
				},
				Action: cliService.WebsiteDelete,
			},
			{
				Name:   "list",
				Usage:  t.Get("List all websites"),
				Action: cliService.WebsiteList,
			},
			{
				Name:  "cert",
				Usage: t.Get("Update website certificate from files"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    t.Get("Website name"),
						Aliases:  []string{"n"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "cert",
						Usage:    t.Get("Certificate file path (fullchain)"),
						Aliases:  []string{"c"},
						Required: true,
					},
					&cli.StringFlag{
						Name:     "key",
						Usage:    t.Get("Private key file path"),
						Aliases:  []string{"k"},
						Required: true,
					},
				},
				Action: cliService.WebsiteCert,
			},
			{
				Name:   "write",
				Usage:  t.Get("Write website data to the panel database only, without creating directories and config files (use only under guidance)"),
				Hidden: true,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Usage:    t.Get("Website name"),
						Aliases:  []string{"n"},
						Required: true,
					},
					&cli.StringFlag{
						Name:    "type",
						Usage:   t.Get("Website type (proxy, static, php)"),
						Aliases: []string{"t"},
						Value:   "static",
					},
					&cli.StringFlag{
						Name:    "path",
						Usage:   t.Get("Path where the website is hosted (default path if not filled)"),
						Aliases: []string{"p"},
					},
					&cli.BoolFlag{
						Name:  "status",
						Usage: t.Get("Website status"),
						Value: true,
					},
					&cli.BoolFlag{
						Name:  "ssl",
						Usage: t.Get("Whether the website has SSL enabled"),
					},
					&cli.StringFlag{
						Name:  "remark",
						Usage: t.Get("Website remark"),
					},
				},
				Action: cliService.WebsiteWrite,
			},
		},
	}
}
