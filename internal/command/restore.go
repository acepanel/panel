package command

import (
	"context"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/do/v2"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// RestoreCommand 数据恢复命令组
func RestoreCommand(i do.Injector) (*cli.Command, error) {
	t := do.MustInvoke[*gotext.Locale](i)
	return &cli.Command{
		Name:  "restore",
		Usage: t.Get("Data restore"),
		Commands: []*cli.Command{
			{
				Name:  "website",
				Usage: t.Get("Restore website backup"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    t.Get("Website name"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "file",
						Aliases:  []string{"f"},
						Usage:    t.Get("Backup file (absolute path or filename under default backup path)"),
						Required: true,
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return do.MustInvoke[*service.CliService](i).RestoreWebsite(ctx, cmd)
				},
			},
			{
				Name:  "database",
				Usage: t.Get("Restore database backup"),
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "type",
						Aliases:  []string{"t"},
						Usage:    t.Get("Database type"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Usage:    t.Get("Database name"),
						Required: true,
					},
					&cli.StringFlag{
						Name:     "file",
						Aliases:  []string{"f"},
						Usage:    t.Get("Backup file (absolute path or filename under default backup path)"),
						Required: true,
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return do.MustInvoke[*service.CliService](i).RestoreDatabase(ctx, cmd)
				},
			},
		},
	}, nil
}
