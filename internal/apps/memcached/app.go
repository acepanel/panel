package memcached

import (
	"bufio"
	"net"
	"net/http"
	"regexp"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/systemctl"
	"github.com/tnborg/panel/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Load(c fiber.Ctx) error {
	status, err := systemctl.Status("memcached")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get Memcached status: %v", err))
	}
	if !status {
		return service.Success(c, []types.NV{})
	}

	conn, err := net.Dial("tcp", "127.0.0.1:11211")
	if err != nil {
		return service.Success(c, []types.NV{})
	}
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	_, err = conn.Write([]byte("stats\nquit\n"))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to write to Memcached: %v", err))
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
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to read from Memcached: %v", err))
	}

	return service.Success(c, data)
}

func (s *App) GetConfig(c fiber.Ctx) error {
	config, err := io.Read("/etc/systemd/system/memcached.service")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write("/etc/systemd/system/memcached.service", req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("memcached"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}
