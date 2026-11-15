package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// EntranceCommand 访问入口管理命令组
func EntranceCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "entrance",
		Usage: t.Get("Operate AcePanel access entrance"),
		Commands: []*cli.Command{
			{
				Name:      "on",
				Usage:     t.Get("Enable access entrance"),
				Arguments: []cli.Argument{optArg("entrance", t.Get("Entrance, e.g. /mypanel, random if omitted"))},
				Action:    cliService.EntranceOn,
			},
			{
				Name:   "off",
				Usage:  t.Get("Disable access entrance"),
				Action: cliService.EntranceOff,
			},
			{
				Name:      "error",
				Usage:     t.Get("Set the error page for wrong entrance"),
				Arguments: []cli.Argument{arg("type", t.Get("Error page type (418, nginx, close)"))},
				Action:    cliService.EntranceError,
			},
		},
	}
}

// BindDomainCommand 域名绑定管理命令组
func BindDomainCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "bind-domain",
		Usage: t.Get("Operate AcePanel domain binding"),
		Commands: []*cli.Command{
			{
				Name:      "on",
				Usage:     t.Get("Enable domain binding"),
				Arguments: []cli.Argument{multiArg("domain", t.Get("Domain"))},
				Action:    cliService.BindDomainOn,
			},
			{
				Name:   "off",
				Usage:  t.Get("Disable domain binding"),
				Action: cliService.BindDomainOff,
			},
		},
	}
}

// BindIPCommand IP 绑定管理命令组
func BindIPCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "bind-ip",
		Usage: t.Get("Operate AcePanel IP binding"),
		Commands: []*cli.Command{
			{
				Name:      "on",
				Usage:     t.Get("Enable IP binding"),
				Arguments: []cli.Argument{multiArg("ip", t.Get("IP"))},
				Action:    cliService.BindIPOn,
			},
			{
				Name:   "off",
				Usage:  t.Get("Disable IP binding"),
				Action: cliService.BindIPOff,
			},
		},
	}
}

// BindUACommand UA 绑定管理命令组
func BindUACommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "bind-ua",
		Usage: t.Get("Operate AcePanel UA binding"),
		Commands: []*cli.Command{
			{
				Name:      "on",
				Usage:     t.Get("Enable UA binding"),
				Arguments: []cli.Argument{multiArg("user-agent", t.Get("User-Agent"))},
				Action:    cliService.BindUAOn,
			},
			{
				Name:   "off",
				Usage:  t.Get("Disable UA binding"),
				Action: cliService.BindUAOff,
			},
		},
	}
}
