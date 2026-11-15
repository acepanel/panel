package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// FirewallCommand 防火墙管理命令组
func FirewallCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "firewall",
		Usage: t.Get("Firewall management"),
		Commands: []*cli.Command{
			{
				Name:   "status",
				Usage:  t.Get("Get firewall status"),
				Action: cliService.FirewallStatus,
			},
			{
				Name:   "on",
				Usage:  t.Get("Enable firewall"),
				Action: cliService.FirewallOn,
			},
			{
				Name:   "off",
				Usage:  t.Get("Disable firewall"),
				Action: cliService.FirewallOff,
			},
			{
				Name:   "list",
				Usage:  t.Get("List all firewall rules"),
				Action: cliService.FirewallList,
			},
			{
				Name:      "port",
				Usage:     t.Get("Allow or remove a port rule"),
				Arguments: []cli.Argument{arg("port", t.Get("Port or port range, e.g. 8888 or 8000-9000"))},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "protocol",
						Usage:   t.Get("Protocol (tcp, udp, tcp/udp)"),
						Aliases: []string{"p"},
						Value:   "tcp/udp",
					},
					&cli.BoolFlag{
						Name:  "remove",
						Usage: t.Get("Remove the rule instead of adding it"),
					},
				},
				Action: cliService.FirewallPort,
			},
		},
	}
}
