package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// SettingCommand 面板设置管理命令组
func SettingCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:   "setting",
		Usage:  t.Get("Setting management"),
		Hidden: true,
		Commands: []*cli.Command{
			{
				Name:      "get",
				Usage:     t.Get("Get panel setting (use only under guidance)"),
				Arguments: []cli.Argument{arg("key", t.Get("Key"))},
				Hidden:    true,
				Action:    cliService.GetSetting,
			},
			{
				Name:      "write",
				Usage:     t.Get("Write panel setting (use only under guidance)"),
				Arguments: []cli.Argument{arg("key", t.Get("Key")), arg("value", t.Get("Value"))},
				Hidden:    true,
				Action:    cliService.WriteSetting,
			},
			{
				Name:      "remove",
				Usage:     t.Get("Remove panel setting (use only under guidance)"),
				Arguments: []cli.Argument{arg("key", t.Get("Key"))},
				Hidden:    true,
				Action:    cliService.RemoveSetting,
			},
		},
	}
}
