package bootstrap

import (
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/command"
	"github.com/acepanel/panel/v3/internal/service"
)

func NewCli(t *gotext.Locale, cliService *service.CliService) app.CliBuilder {
	// 单趟替换按位置匹配，GLOBAL OPTIONS 不会被 OPTIONS 截走后半段
	headings := strings.NewReplacer(
		"NAME", t.Get("NAME"),
		"USAGE", t.Get("USAGE"),
		"VERSION", t.Get("VERSION"),
		"CATEGORY", t.Get("CATEGORY"),
		"DESCRIPTION", t.Get("DESCRIPTION"),
		"AUTHOR", t.Get("AUTHOR"),
		"COMMANDS", t.Get("COMMANDS"),
		"GLOBAL OPTIONS", t.Get("GLOBAL OPTIONS"),
		"OPTIONS", t.Get("OPTIONS"),
		"COPYRIGHT", t.Get("COPYRIGHT"),
	)
	for _, tpl := range []*string{&cli.RootCommandHelpTemplate, &cli.CommandHelpTemplate, &cli.SubcommandHelpTemplate} {
		*tpl = headings.Replace(*tpl)
	}

	cli.RootCommandHelpTemplate += "\n" + t.Get("Website：https://acepanel.net")
	cli.RootCommandHelpTemplate += "\n" + t.Get("Forum：https://tom.moe")
	cli.RootCommandHelpTemplate += "\n" + t.Get("QQ Group：12370907") + "\n"

	return func() *cli.Command {
		return &cli.Command{
			Name:    "acepanel",
			Usage:   t.Get("AcePanel CLI Tool"),
			Version: app.Version,
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "json",
					Usage: t.Get("Output in JSON format (list commands only)"),
				},
			},
			Commands: command.Commands(t, cliService),
		}
	}
}
