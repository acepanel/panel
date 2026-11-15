package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// HttpsCommand 面板 HTTPS 管理命令组
func HttpsCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "https",
		Usage: t.Get("Operate AcePanel HTTPS"),
		Commands: []*cli.Command{
			{
				Name:   "on",
				Usage:  t.Get("Enable HTTPS"),
				Action: cliService.HTTPSOn,
			},
			{
				Name:   "off",
				Usage:  t.Get("Disable HTTPS"),
				Action: cliService.HTTPSOff,
			},
			{
				Name:   "generate",
				Usage:  t.Get("Obtain a free certificate or generate a self-signed certificate"),
				Action: cliService.HTTPSGenerate,
			},
		},
	}
}
