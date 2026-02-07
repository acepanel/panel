package redis

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status("redis")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get redis status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	// 检查 Redis 密码
	withPassword := ""
	config, err := io.Read(fmt.Sprintf("%s/server/redis/redis.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	re := regexp.MustCompile(`^requirepass\s+(.+)`)
	matches := re.FindStringSubmatch(config)
	if len(matches) == 2 {
		withPassword = " -a " + matches[1]
	}

	raw, err := shell.Execf("redis-cli%s info", withPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get redis info: %v", err))
		return
	}

	infoLines := strings.Split(raw, "\n")
	dataRaw := make(map[string]string)

	for _, item := range infoLines {
		parts := strings.Split(item, ":")
		if len(parts) == 2 {
			dataRaw[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	data := []types.NV{
		{Name: s.t.Get("TCP Port"), Value: dataRaw["tcp_port"]},
		{Name: s.t.Get("Uptime in Days"), Value: dataRaw["uptime_in_days"]},
		{Name: s.t.Get("Connected Clients"), Value: dataRaw["connected_clients"]},
		{Name: s.t.Get("Total Allocated Memory"), Value: dataRaw["used_memory_human"]},
		{Name: s.t.Get("Total Memory Usage"), Value: dataRaw["used_memory_rss_human"]},
		{Name: s.t.Get("Peak Memory Usage"), Value: dataRaw["used_memory_peak_human"]},
		{Name: s.t.Get("Memory Fragmentation Ratio"), Value: dataRaw["mem_fragmentation_ratio"]},
		{Name: s.t.Get("Total Connections Received"), Value: dataRaw["total_connections_received"]},
		{Name: s.t.Get("Total Commands Processed"), Value: dataRaw["total_commands_processed"]},
		{Name: s.t.Get("Commands Per Second"), Value: dataRaw["instantaneous_ops_per_sec"]},
		{Name: s.t.Get("Keyspace Hits"), Value: dataRaw["keyspace_hits"]},
		{Name: s.t.Get("Keyspace Misses"), Value: dataRaw["keyspace_misses"]},
		{Name: s.t.Get("Latest Fork Time (ms)"), Value: dataRaw["latest_fork_usec"]},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/redis/redis.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/redis/redis.conf", app.Root), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("redis"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetConfigTune 获取 Redis 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/redis/redis.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		Bind:            s.getRedisValue(config, "bind"),
		Port:            s.getRedisValue(config, "port"),
		Databases:       s.getRedisValue(config, "databases"),
		Requirepass:     s.getRedisValue(config, "requirepass"),
		Timeout:         s.getRedisValue(config, "timeout"),
		TCPKeepalive:    s.getRedisValue(config, "tcp-keepalive"),
		Maxmemory:       s.getRedisValue(config, "maxmemory"),
		MaxmemoryPolicy: s.getRedisValue(config, "maxmemory-policy"),
		Appendonly:      s.getRedisValue(config, "appendonly"),
		Appendfsync:     s.getRedisValue(config, "appendfsync"),
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Redis 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/redis/redis.conf", app.Root)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = s.setRedisValue(config, "bind", req.Bind)
	config = s.setRedisValue(config, "port", req.Port)
	config = s.setRedisValue(config, "databases", req.Databases)
	config = s.setRedisValue(config, "requirepass", req.Requirepass)
	config = s.setRedisValue(config, "timeout", req.Timeout)
	config = s.setRedisValue(config, "tcp-keepalive", req.TCPKeepalive)
	config = s.setRedisValue(config, "maxmemory", req.Maxmemory)
	config = s.setRedisValue(config, "maxmemory-policy", req.MaxmemoryPolicy)
	config = s.setRedisValue(config, "appendonly", req.Appendonly)
	config = s.setRedisValue(config, "appendfsync", req.Appendfsync)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("redis"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// getRedisValue 从 Redis 配置内容中获取指定键的值
func (s *App) getRedisValue(content string, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) >= 2 && parts[0] == key {
			return strings.Join(parts[1:], " ")
		}
	}
	return ""
}

// setRedisValue 在 Redis 配置内容中设置指定键的值
func (s *App) setRedisValue(content string, key string, value string) string {
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result = append(result, line)
			continue
		}
		checkLine := trimmed
		if strings.HasPrefix(checkLine, "#") {
			checkLine = strings.TrimSpace(checkLine[1:])
		}
		parts := strings.Fields(checkLine)
		if len(parts) >= 1 && parts[0] == key {
			if found {
				continue
			}
			found = true
			// 值为空时注释掉该配置项
			if value == "" {
				if !strings.HasPrefix(trimmed, "#") {
					result = append(result, "# "+trimmed)
				} else {
					result = append(result, line)
				}
				continue
			}
			result = append(result, key+" "+value)
		} else {
			result = append(result, line)
		}
	}
	if !found && value != "" {
		result = append(result, key+" "+value)
	}
	return strings.Join(result, "\n")
}
