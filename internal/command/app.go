package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// AppCommand 应用管理命令组
func AppCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "app",
		Usage: t.Get("Application management"),
		Commands: []*cli.Command{
			{
				Name:      "install",
				Usage:     t.Get("Install application"),
				Arguments: []cli.Argument{arg("slug", t.Get("Slug")), optArg("channel", t.Get("Channel"))},
				Action:    cliService.AppInstall,
			},
			{
				Name:      "uninstall",
				Usage:     t.Get("Uninstall application"),
				Arguments: []cli.Argument{arg("slug", t.Get("Slug"))},
				Action:    cliService.AppUnInstall,
			},
			{
				Name:      "update",
				Usage:     t.Get("Update application"),
				Arguments: []cli.Argument{arg("slug", t.Get("Slug"))},
				Action:    cliService.AppUpdate,
			},
			{
				Name:   "list",
				Usage:  t.Get("List installed applications"),
				Action: cliService.AppList,
			},
			{
				Name:      "write",
				Usage:     t.Get("Add panel application mark (use only under guidance)"),
				Arguments: []cli.Argument{arg("slug", t.Get("Slug")), arg("channel", t.Get("Channel")), arg("version", t.Get("Version"))},
				Hidden:    true,
				Action:    cliService.AppWrite,
			},
			{
				Name:      "remove",
				Usage:     t.Get("Remove panel application mark (use only under guidance)"),
				Arguments: []cli.Argument{arg("slug", t.Get("Slug"))},
				Hidden:    true,
				Action:    cliService.AppRemove,
			},
		},
	}
}
