package route

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/service"
)

type Cli struct {
	t   *gotext.Locale
	cli *service.CliService
}

func NewCli(t *gotext.Locale, cli *service.CliService) *Cli {
	return &Cli{
		t:   t,
		cli: cli,
	}
}

func (route *Cli) Commands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "status",
			Usage:  route.t.Get("Get AcePanel service status"),
			Action: route.cli.Status,
		},
		{
			Name:   "restart",
			Usage:  route.t.Get("Restart AcePanel service"),
			Action: route.cli.Restart,
		},
		{
			Name:   "stop",
			Usage:  route.t.Get("Stop AcePanel service"),
			Action: route.cli.Stop,
		},
		{
			Name:   "start",
			Usage:  route.t.Get("Start AcePanel service"),
			Action: route.cli.Start,
		},
		{
			Name:   "update",
			Usage:  route.t.Get("Update AcePanel to the latest version"),
			Action: route.cli.Update,
		},
		{
			Name:   "sync",
			Usage:  route.t.Get("Sync AcePanel cached data with cloud"),
			Action: route.cli.Sync,
		},
		{
			Name:   "fix",
			Usage:  route.t.Get("Fix AcePanel upgrade issues"),
			Action: route.cli.Fix,
		},
		{
			Name:   "info",
			Usage:  route.t.Get("Output AcePanel basic information and generate new password"),
			Action: route.cli.Info,
		},
		{
			Name:  "user",
			Usage: route.t.Get("Operate AcePanel users"),
			Commands: []*cli.Command{
				{
					Name:   "list",
					Usage:  route.t.Get("List all users"),
					Action: route.cli.UserList,
				},
				{
					Name:   "username",
					Usage:  route.t.Get("Change a user's username"),
					Action: route.cli.UserName,
				},
				{
					Name:   "password",
					Usage:  route.t.Get("Change a user's password"),
					Action: route.cli.UserPassword,
				},
				{
					Name:   "2fa",
					Usage:  route.t.Get("Toggle two-factor authentication for a user"),
					Action: route.cli.UserTwoFA,
				},
				{
					Name:   "passkey",
					Usage:  route.t.Get("Clear all passkeys for a user"),
					Action: route.cli.UserPasskey,
				},
			},
		},
		{
			Name:  "https",
			Usage: route.t.Get("Operate AcePanel HTTPS"),
			Commands: []*cli.Command{
				{
					Name:   "on",
					Usage:  route.t.Get("Enable HTTPS"),
					Action: route.cli.HTTPSOn,
				},
				{
					Name:   "off",
					Usage:  route.t.Get("Disable HTTPS"),
					Action: route.cli.HTTPSOff,
				},
				{
					Name:   "generate",
					Usage:  route.t.Get("Obtain a free certificate or generate a self-signed certificate"),
					Action: route.cli.HTTPSGenerate,
				},
			},
		},
		{
			Name:  "entrance",
			Usage: route.t.Get("Operate AcePanel access entrance"),
			Commands: []*cli.Command{
				{
					Name:   "on",
					Usage:  route.t.Get("Enable access entrance"),
					Action: route.cli.EntranceOn,
				},
				{
					Name:   "off",
					Usage:  route.t.Get("Disable access entrance"),
					Action: route.cli.EntranceOff,
				},
			},
		},
		{
			Name:  "bind-domain",
			Usage: route.t.Get("Operate AcePanel domain binding"),
			Commands: []*cli.Command{
				{
					Name:   "off",
					Usage:  route.t.Get("Disable domain binding"),
					Action: route.cli.BindDomainOff,
				},
			},
		},
		{
			Name:  "bind-ip",
			Usage: route.t.Get("Operate AcePanel IP binding"),
			Commands: []*cli.Command{
				{
					Name:   "off",
					Usage:  route.t.Get("Disable IP binding"),
					Action: route.cli.BindIPOff,
				},
			},
		},
		{
			Name:  "bind-ua",
			Usage: route.t.Get("Operate AcePanel UA binding"),
			Commands: []*cli.Command{
				{
					Name:   "off",
					Usage:  route.t.Get("Disable UA binding"),
					Action: route.cli.BindUAOff,
				},
			},
		},
		{
			Name:   "port",
			Usage:  route.t.Get("Change the AcePanel listening port"),
			Action: route.cli.Port,
		},
		{
			Name:  "website",
			Usage: route.t.Get("Website management"),
			Commands: []*cli.Command{
				{
					Name:   "create",
					Usage:  route.t.Get("Create new website"),
					Action: route.cli.WebsiteCreate,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Website name"),
							Aliases:  []string{"n"},
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     "domains",
							Usage:    route.t.Get("List of domains associated with the website"),
							Aliases:  []string{"d"},
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     "listens",
							Usage:    route.t.Get("List of listening addresses associated with the website"),
							Aliases:  []string{"l"},
							Required: true,
						},
						&cli.StringFlag{
							Name:  "path",
							Usage: route.t.Get("Path where the website is hosted (default path if not filled)"),
						},
						&cli.UintFlag{
							Name:  "php",
							Usage: route.t.Get("PHP version used by the website (not used if not filled)"),
						},
					},
				},
				{
					Name:   "remove",
					Usage:  route.t.Get("Remove website"),
					Action: route.cli.WebsiteRemove,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Website name"),
							Aliases:  []string{"n"},
							Required: true,
						},
					},
				},
				{
					Name:   "delete",
					Usage:  route.t.Get("Delete website (including website directory, database with the same name)"),
					Action: route.cli.WebsiteDelete,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Website name"),
							Aliases:  []string{"n"},
							Required: true,
						},
					},
				},
				{
					Name:   "write",
					Usage:  route.t.Get("Write website data (use only under guidance)"),
					Hidden: true,
					Action: route.cli.WebsiteWrite,
				},
			},
		},
		{
			Name:  "database",
			Usage: route.t.Get("Database management"),
			Commands: []*cli.Command{
				{
					Name:   "add-server",
					Usage:  route.t.Get("Add database server"),
					Action: route.cli.DatabaseAddServer,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Usage:    route.t.Get("Server type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Server name"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "host",
							Usage:    route.t.Get("Server address"),
							Required: true,
						},
						&cli.UintFlag{
							Name:     "port",
							Usage:    route.t.Get("Server port"),
							Required: true,
						},
						&cli.StringFlag{
							Name:  "username",
							Usage: route.t.Get("Server username"),
						},
						&cli.StringFlag{
							Name:  "password",
							Usage: route.t.Get("Server password"),
						},
						&cli.StringFlag{
							Name:  "remark",
							Usage: route.t.Get("Server remark"),
						},
					},
				},
				{
					Name:   "delete-server",
					Usage:  route.t.Get("Delete database server"),
					Action: route.cli.DatabaseDeleteServer,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Server name"),
							Aliases:  []string{"n"},
							Required: true,
						},
					},
				},
			},
		},
		{
			Name:  "backup",
			Usage: route.t.Get("Data backup"),
			Commands: []*cli.Command{
				{
					Name:   "website",
					Usage:  route.t.Get("Backup website"),
					Action: route.cli.BackupWebsite,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Website name"),
							Required: true,
						},
						&cli.UintFlag{
							Name:    "storage",
							Aliases: []string{"s"},
							Usage:   route.t.Get("Storage ID (local storage if not filled)"),
						},
					},
				},
				{
					Name:   "database",
					Usage:  route.t.Get("Backup database"),
					Action: route.cli.BackupDatabase,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Usage:    route.t.Get("Database type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Database name"),
							Required: true,
						},
						&cli.UintFlag{
							Name:    "storage",
							Aliases: []string{"s"},
							Usage:   route.t.Get("Storage ID (local storage if not filled)"),
						},
					},
				},
				{
					Name:   "panel",
					Usage:  route.t.Get("Backup panel"),
					Action: route.cli.BackupPanel,
					Flags:  []cli.Flag{},
				},
				{
					Name:   "clear",
					Usage:  route.t.Get("Clear backups"),
					Action: route.cli.BackupClear,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Usage:    route.t.Get("Backup type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "file",
							Aliases:  []string{"f"},
							Usage:    route.t.Get("Backup file"),
							Required: true,
						},
						&cli.UintFlag{
							Name:     "keep",
							Aliases:  []string{"k"},
							Usage:    route.t.Get("Number of backups to keep"),
							Required: true,
						},
						&cli.UintFlag{
							Name:    "storage",
							Aliases: []string{"s"},
							Usage:   route.t.Get("Storage ID (local storage if not filled)"),
						},
					},
				},
			},
		},
		{
			Name:  "cutoff",
			Usage: route.t.Get("Log rotation"),
			Commands: []*cli.Command{
				{
					Name:   "website",
					Usage:  route.t.Get("Website"),
					Action: route.cli.CutoffWebsite,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Website name"),
							Required: true,
						},
						&cli.UintFlag{
							Name:    "storage",
							Aliases: []string{"s"},
							Usage:   route.t.Get("Storage ID (local storage if not filled)"),
						},
					},
				},
				{
					Name:   "container",
					Usage:  route.t.Get("Container"),
					Action: route.cli.CutoffContainer,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Container name"),
							Required: true,
						},
						&cli.UintFlag{
							Name:    "storage",
							Aliases: []string{"s"},
							Usage:   route.t.Get("Storage ID (local storage if not filled)"),
						},
					},
				},
				{
					Name:   "clear",
					Usage:  route.t.Get("Clear rotated logs"),
					Action: route.cli.CutoffClear,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Usage:    route.t.Get("Rotation type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Target name"),
							Required: true,
						},
						&cli.UintFlag{
							Name:     "keep",
							Aliases:  []string{"k"},
							Usage:    route.t.Get("Number of logs to keep"),
							Required: true,
						},
						&cli.UintFlag{
							Name:    "storage",
							Aliases: []string{"s"},
							Usage:   route.t.Get("Storage ID (local storage if not filled)"),
						},
					},
				},
			},
		},
		{
			Name:  "app",
			Usage: route.t.Get("Application management"),
			Commands: []*cli.Command{
				{
					Name:   "install",
					Usage:  route.t.Get("Install application"),
					Action: route.cli.AppInstall,
				},
				{
					Name:   "uninstall",
					Usage:  route.t.Get("Uninstall application"),
					Action: route.cli.AppUnInstall,
				},
				{
					Name:   "update",
					Usage:  route.t.Get("Update application"),
					Action: route.cli.AppUpdate,
				},
				{
					Name:   "write",
					Usage:  route.t.Get("Add panel application mark (use only under guidance)"),
					Hidden: true,
					Action: route.cli.AppWrite,
				},
				{
					Name:   "remove",
					Usage:  route.t.Get("Remove panel application mark (use only under guidance)"),
					Hidden: true,
					Action: route.cli.AppRemove,
				},
			},
		},
		{
			Name:   "setting",
			Usage:  route.t.Get("Setting management"),
			Hidden: true,
			Commands: []*cli.Command{
				{
					Name:   "get",
					Usage:  route.t.Get("Get panel setting (use only under guidance)"),
					Hidden: true,
					Action: route.cli.GetSetting,
				},
				{
					Name:   "write",
					Usage:  route.t.Get("Write panel setting (use only under guidance)"),
					Hidden: true,
					Action: route.cli.WriteSetting,
				},
				{
					Name:   "remove",
					Usage:  route.t.Get("Remove panel setting (use only under guidance)"),
					Hidden: true,
					Action: route.cli.RemoveSetting,
				},
			},
		},
		{
			Name:   "sync-time",
			Usage:  route.t.Get("Sync server time with NTP"),
			Action: route.cli.SyncTime,
		},
		{
			Name:   "clear-task",
			Usage:  route.t.Get("Clear all tasks in the task queue if they are stuck (use only under guidance)"),
			Hidden: true,
			Action: route.cli.ClearTask,
		},
		{
			Name:   "init",
			Usage:  route.t.Get("Initialize AcePanel (use only under guidance)"),
			Hidden: true,
			Action: route.cli.Init,
		},
	}
}
