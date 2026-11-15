package fail2ban

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/pkg/io"
)

const (
	// jailLocal 只承载 [DEFAULT] 全局项，规则一律放 jail.d
	jailLocal = "/etc/fail2ban/jail.local"
	// panelFilterPrefix 面板为网站规则生成的过滤器前缀，用于区分 fail2ban 自带的过滤器
	panelFilterPrefix = "haozi-"
)

func (s *App) jailPath(name string) string {
	// jailDDir fail2ban 原生的 drop-in 目录，.local 后缀的读取优先级最高
	const jailDDir = "/etc/fail2ban/jail.d"
	return filepath.Join(jailDDir, name+".local")
}

func (s *App) filterPath(filter string) string {
	const filterDir = "/etc/fail2ban/filter.d"
	return filepath.Join(filterDir, filter+".conf")
}

func (s *App) readJail(name string) (Jail, error) {
	raw, err := io.Read(s.jailPath(name))
	if err != nil {
		return Jail{}, err
	}

	return Jail{
		Name:     name,
		Enabled:  s.truthy(confval.SectionINI.GetIn(raw, name, "enabled")),
		MaxRetry: cast.ToInt(confval.SectionINI.GetIn(raw, name, "maxretry")),
		FindTime: cast.ToInt(confval.SectionINI.GetIn(raw, name, "findtime")),
		BanTime:  cast.ToInt(confval.SectionINI.GetIn(raw, name, "bantime")),
		Filter:   confval.SectionINI.GetIn(raw, name, "filter"),
		Port:     confval.SectionINI.GetIn(raw, name, "port"),
		LogPath:  confval.SectionINI.GetIn(raw, name, "logpath"),
	}, nil
}

// truthy fail2ban 的布尔值除了 true、1 还认 yes、on，手写的配置里常见
func (s *App) truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *App) listJails() ([]Jail, error) {
	// Glob 在目录不存在时返回空，无需额外判断
	paths, err := filepath.Glob(s.jailPath("*"))
	if err != nil {
		return nil, err
	}

	jails := make([]Jail, 0, len(paths))
	for _, path := range paths {
		jail, err := s.readJail(strings.TrimSuffix(filepath.Base(path), ".local"))
		if err != nil {
			return nil, err
		}

		jails = append(jails, jail)
	}

	return jails, nil
}

func (s *App) writeJail(jail Jail) error {
	config := fmt.Sprintf(
		"[%s]\nenabled = %t\nfilter = %s\nport = %s\nmaxretry = %d\nfindtime = %d\nbantime = %d\n",
		jail.Name, jail.Enabled, jail.Filter, jail.Port, jail.MaxRetry, jail.FindTime, jail.BanTime,
	)
	if jail.LogPath != "" {
		config += "backend = auto\nlogpath = " + jail.LogPath + "\n"
	}

	return io.Write(s.jailPath(jail.Name), config, 0644)
}
