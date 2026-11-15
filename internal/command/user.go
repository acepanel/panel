package command

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

// UserCommand 用户管理命令组
func UserCommand(t *gotext.Locale, cliService *service.CliService) *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: t.Get("Operate AcePanel users"),
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  t.Get("List all users"),
				Action: cliService.UserList,
			},
			{
				Name:  "create",
				Usage: t.Get("Create a new user (password is read from the ACEPANEL_PASSWORD environment variable or entered interactively if omitted)"),
				Arguments: []cli.Argument{
					arg("username", t.Get("Username")),
					optArg("password", t.Get("Password")),
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "email",
						Usage:   t.Get("User email (generated from the username if not filled)"),
						Aliases: []string{"e"},
					},
				},
				Action: cliService.UserCreate,
			},
			{
				Name:      "delete",
				Usage:     t.Get("Delete a user"),
				Arguments: []cli.Argument{arg("username", t.Get("Username"))},
				Action:    cliService.UserDelete,
			},
			{
				Name:  "username",
				Usage: t.Get("Change a user's username"),
				Arguments: []cli.Argument{
					arg("old-username", t.Get("Old username")),
					arg("new-username", t.Get("New username")),
				},
				Action: cliService.UserName,
			},
			{
				Name:  "password",
				Usage: t.Get("Change a user's password (password is read from the ACEPANEL_PASSWORD environment variable or entered interactively if omitted)"),
				Arguments: []cli.Argument{
					arg("username", t.Get("Username")),
					optArg("password", t.Get("Password")),
				},
				Action: cliService.UserPassword,
			},
			{
				Name:      "2fa",
				Usage:     t.Get("Toggle two-factor authentication for a user"),
				Arguments: []cli.Argument{arg("username", t.Get("Username"))},
				Action:    cliService.UserTwoFA,
			},
			{
				Name:      "passkey",
				Usage:     t.Get("Clear all passkeys for a user"),
				Arguments: []cli.Argument{arg("username", t.Get("Username"))},
				Action:    cliService.UserPasskey,
			},
		},
	}
}
