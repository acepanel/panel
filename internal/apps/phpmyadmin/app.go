package phpmyadmin

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/firewall"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/systemctl"
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
	r.Get("/info", s.Info)
	r.Post("/port", s.UpdatePort)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Info(c fiber.Ctx) error {
	files, err := os.ReadDir(fmt.Sprintf("%s/server/phpmyadmin", app.Root))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("phpMyAdmin directory not found"))
	}

	var phpmyadmin string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "phpmyadmin_") {
			phpmyadmin = f.Name()
		}
	}
	if len(phpmyadmin) == 0 {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("phpMyAdmin directory not found"))
	}

	conf, err := io.Read(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	match := regexp.MustCompile(`listen\s+(\d+);`).FindStringSubmatch(conf)
	if len(match) == 0 {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("phpMyAdmin port not found"))
	}

	return service.Success(c, chix.M{
		"path": phpmyadmin,
		"port": cast.ToInt(match[1]),
	})
}

func (s *App) UpdatePort(c fiber.Ctx) error {
	req, err := service.Bind[UpdatePort](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	conf, err := io.Read(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	conf = regexp.MustCompile(`listen\s+(\d+);`).ReplaceAllString(conf, "listen "+cast.ToString(req.Port)+";")
	if err = io.Write(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root), conf, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	fw := firewall.NewFirewall()
	err = fw.Port(firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: req.Port,
		PortEnd:   req.Port,
		Direction: firewall.DirectionIn,
		Strategy:  firewall.StrategyAccept,
	}, firewall.OperationAdd)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
	}

	return service.Success(c, nil)
}

func (s *App) GetConfig(c fiber.Ctx) error {
	config, err := io.Read(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root))
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

	if err = io.Write(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
	}

	return service.Success(c, nil)
}
