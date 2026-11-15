package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// WebserverCommand Web 服务器管理命令组
func WebserverCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "webserver",
		Usage:  t.Get("Web server management"),
		Hidden: true,
		Commands: []*cli.Command{
			{
				Name:   "reload",
				Usage:  t.Get("Reload the web server (use only under guidance)"),
				Hidden: true,
				Action: cliService.ReloadWebserver,
			},
		},
	}
}
