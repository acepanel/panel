package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// CertCommand 证书管理命令组
func CertCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "cert",
		Usage: t.Get("Certificate management"),
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  t.Get("List all certificates"),
				Action: cliService.CertList,
			},
			{
				Name:  "renew",
				Usage: t.Get("Renew certificate"),
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:    "id",
						Usage:   t.Get("Certificate ID"),
						Aliases: []string{"i"},
					},
					&cli.BoolFlag{
						Name:  "all",
						Usage: t.Get("Renew all certificates with auto renewal enabled"),
					},
				},
				Action: cliService.CertRenew,
			},
		},
	}
}
