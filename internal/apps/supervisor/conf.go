package supervisor

import (
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/pkg/os"
)

// mainConfPath supervisor 主配置，RHEL 系与 Debian 系的路径不同
func (s *App) mainConfPath() string {
	return lo.Ternary(os.IsRHEL(), "/etc/supervisord.conf", "/etc/supervisor/supervisord.conf")
}

func (s *App) confPath(name string) string {
	return lo.Ternary(os.IsRHEL(), "/etc/supervisord.d/", "/etc/supervisor/conf.d/") + name + ".conf"
}

func (s *App) settingFields(setting *ProcessSetting) []confval.Field {
	return []confval.Field{
		{Key: "command", Value: &setting.Command},
		{Key: "directory", Value: &setting.Directory},
		{Key: "user", Value: &setting.User},
		{Key: "numprocs", Value: &setting.NumProcs},
		{Key: "priority", Value: &setting.Priority},
		{Key: "autostart", Value: &setting.AutoStart},
		{Key: "autorestart", Value: &setting.AutoRestart},
		{Key: "startsecs", Value: &setting.StartSecs},
		{Key: "startretries", Value: &setting.StartRetries},
		{Key: "stopwaitsecs", Value: &setting.StopWaitSecs},
		{Key: "stopasgroup", Value: &setting.StopAsGroup},
		{Key: "killasgroup", Value: &setting.KillAsGroup},
		{Key: "redirect_stderr", Value: &setting.RedirectStderr},
		{Key: "stdout_logfile", Value: &setting.StdoutLogfile},
		{Key: "stdout_logfile_maxbytes", Value: &setting.StdoutLogfileMaxBytes},
		{Key: "stdout_logfile_backups", Value: &setting.StdoutLogfileBackups},
		{Key: "environment", Value: &setting.Environment},
	}
}

// writeSetting numprocs 大于 1 时 process_name 必须带上 process_num，否则 supervisor 会拒绝加载
func (s *App) writeSetting(config string, setting *ProcessSetting) string {
	config = confval.Supervisor.Write(config, s.settingFields(setting))

	return confval.Supervisor.Set(config, "process_name", s.processNameExpr(setting.NumProcs))
}

func (s *App) processNameExpr(numProcs string) string {
	return lo.Ternary(numProcs == "" || numProcs == "1", `%(program_name)s`, `%(program_name)s_%(process_num)02d`)
}
