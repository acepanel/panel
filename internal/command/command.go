package command

import (
	"github.com/samber/do/v2"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/registry"
)

const Prefix = "commands:"

var Package = do.Package(
	do.LazyNamed(Prefix+"status", StatusCommand), do.LazyNamed(Prefix+"restart", RestartCommand),
	do.LazyNamed(Prefix+"stop", StopCommand), do.LazyNamed(Prefix+"start", StartCommand),
	do.LazyNamed(Prefix+"update", UpdateCommand), do.LazyNamed(Prefix+"sync", SyncCommand),
	do.LazyNamed(Prefix+"fix", FixCommand), do.LazyNamed(Prefix+"info", InfoCommand),
	do.LazyNamed(Prefix+"port", PortCommand), do.LazyNamed(Prefix+"sync-time", SyncTimeCommand),
	do.LazyNamed(Prefix+"clear-task", ClearTaskCommand), do.LazyNamed(Prefix+"init", InitCommand),
	do.LazyNamed(Prefix+"user", UserCommand), do.LazyNamed(Prefix+"https", HttpsCommand),
	do.LazyNamed(Prefix+"entrance", EntranceCommand), do.LazyNamed(Prefix+"bind-domain", BindDomainCommand),
	do.LazyNamed(Prefix+"bind-ip", BindIPCommand), do.LazyNamed(Prefix+"bind-ua", BindUACommand),
	do.LazyNamed(Prefix+"website", WebsiteCommand), do.LazyNamed(Prefix+"database", DatabaseCommand),
	do.LazyNamed(Prefix+"backup", BackupCommand), do.LazyNamed(Prefix+"restore", RestoreCommand),
	do.LazyNamed(Prefix+"cutoff", CutoffCommand),
	do.LazyNamed(Prefix+"app", AppCommand), do.LazyNamed(Prefix+"setting", SettingCommand),
)

// Commands 收集全部命令贡献。
func Commands(i do.Injector) ([]*cli.Command, error) {
	return registry.Collect[*cli.Command](i, Prefix)
}
