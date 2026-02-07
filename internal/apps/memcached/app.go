package memcached

import (
	"bufio"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/io"
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
	status, err := systemctl.Status("memcached")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get Memcached status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	conn, err := net.Dial("tcp", "127.0.0.1:11211")
	if err != nil {
		service.Success(w, []types.NV{})
		return
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	_, err = conn.Write([]byte("stats\nquit\n"))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write to Memcached: %v", err))
		return
	}

	data := make([]types.NV, 0)
	re := regexp.MustCompile(`STAT\s(\S+)\s(\S+)`)
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) == 3 {
			data = append(data, types.NV{
				Name:  matches[1],
				Value: matches[2],
			})
		}
		if line == "END" {
			break
		}
	}

	if err = scanner.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to read from Memcached: %v", err))
		return
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read("/etc/systemd/system/memcached.service")
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

	if err = io.Write("/etc/systemd/system/memcached.service", req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("memcached"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetConfigTune 获取 Memcached 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read("/etc/systemd/system/memcached.service")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		Port:           s.getExecStartArg(config, "-p"),
		UDPPort:        s.getExecStartArg(config, "-U"),
		ListenAddress:  s.getExecStartArg(config, "-l"),
		Memory:         s.getExecStartArg(config, "-m"),
		MaxConnections: s.getExecStartArg(config, "-c"),
		Threads:        s.getExecStartArg(config, "-t"),
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Memcached 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := io.Read("/etc/systemd/system/memcached.service")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = s.setExecStartArg(config, "-p", req.Port)
	config = s.setExecStartArg(config, "-U", req.UDPPort)
	config = s.setExecStartArg(config, "-l", req.ListenAddress)
	config = s.setExecStartArg(config, "-m", req.Memory)
	config = s.setExecStartArg(config, "-c", req.MaxConnections)
	config = s.setExecStartArg(config, "-t", req.Threads)

	if err = io.Write("/etc/systemd/system/memcached.service", config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("memcached"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// getExecStartArg 从 systemd service 文件的 ExecStart 行中获取指定参数值
func (s *App) getExecStartArg(content string, flag string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ExecStart=") {
			continue
		}
		args := strings.Fields(trimmed)
		for i, arg := range args {
			if arg == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

// setExecStartArg 在 systemd service 文件的 ExecStart 行中设置指定参数值
func (s *App) setExecStartArg(content string, flag string, value string) string {
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "ExecStart=") {
			result = append(result, line)
			continue
		}
		args := strings.Fields(trimmed)
		newArgs := make([]string, 0, len(args))
		found := false
		for i := 0; i < len(args); i++ {
			if args[i] == flag && i+1 < len(args) {
				i++ // 跳过旧值
				found = true
				// 值为空时删除该参数
				if value != "" {
					newArgs = append(newArgs, flag, value)
				}
			} else {
				newArgs = append(newArgs, args[i])
			}
		}
		if !found && value != "" {
			newArgs = append(newArgs, flag, value)
		}
		result = append(result, strings.Join(newArgs, " "))
	}
	return strings.Join(result, "\n")
}
