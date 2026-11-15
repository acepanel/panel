package bootstrap

import (
	"testing"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/pkg/embed"
)

func TestNewCliTranslatesHelpHeadings(t *testing.T) {
	root, command, subcommand := cli.RootCommandHelpTemplate, cli.CommandHelpTemplate, cli.SubcommandHelpTemplate
	t.Cleanup(func() {
		cli.RootCommandHelpTemplate, cli.CommandHelpTemplate, cli.SubcommandHelpTemplate = root, command, subcommand
	})

	locale := gotext.NewLocaleFSWithPath("zh_CN", embed.LocalesFS, "locales")
	locale.AddDomain("backend")
	global, description := locale.Get("GLOBAL OPTIONS"), locale.Get("DESCRIPTION")
	must.NotEqual(t, global, "GLOBAL OPTIONS")
	must.NotEqual(t, description, "DESCRIPTION")

	NewCli(locale, nil)

	for _, tpl := range []string{cli.RootCommandHelpTemplate, cli.CommandHelpTemplate, cli.SubcommandHelpTemplate} {
		check.Contains(t, tpl, "\n"+global+":")
		check.Contains(t, tpl, "\n"+description+":")
		check.NotContains(t, tpl, "GLOBAL")
	}
}
