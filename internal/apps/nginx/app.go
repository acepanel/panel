package nginx

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/go-resty/resty/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/systemctl"
	"github.com/tnborg/panel/pkg/tools"
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
	r.Post("/config", s.SaveConfig)
	r.Get("/error_log", s.ErrorLog)
	r.Post("/clear_error_log", s.ClearErrorLog)
}

func (s *App) GetConfig(c fiber.Ctx) error {
	config, err := io.Read(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) SaveConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
	}

	return service.Success(c, nil)
}

func (s *App) ErrorLog(c fiber.Ctx) error {
	return service.Success(c, fmt.Sprintf("%s/%s", app.Root, "wwwlogs/nginx-error.log"))
}

func (s *App) ClearErrorLog(c fiber.Ctx) error {
	if _, err := shell.Execf("cat /dev/null > %s/%s", app.Root, "wwwlogs/nginx-error.log"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) Load(c fiber.Ctx) error {
	client := resty.New().SetTimeout(10 * time.Second)
	resp, err := client.R().Get("http://127.0.0.1/nginx_status")
	if err != nil || !resp.IsSuccess() {
		return service.Success(c, []types.NV{})
	}

	raw := resp.String()
	var data []types.NV

	workers, err := shell.Execf("ps aux | grep nginx | grep 'worker process' | wc -l")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
	}
	data = append(data, types.NV{
		Name:  s.t.Get("Workers"),
		Value: workers,
	})

	out, err := shell.Execf("ps aux | grep nginx | grep 'worker process' | awk '{memsum+=$6};END {print memsum}'")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
	}
	mem := tools.FormatBytes(cast.ToFloat64(out))
	data = append(data, types.NV{
		Name:  s.t.Get("Memory"),
		Value: mem,
	})

	match := regexp.MustCompile(`Active connections:\s+(\d+)`).FindStringSubmatch(raw)
	if len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Active connections"),
			Value: match[1],
		})
	}

	match = regexp.MustCompile(`server accepts handled requests\s+(\d+)\s+(\d+)\s+(\d+)`).FindStringSubmatch(raw)
	if len(match) == 4 {
		data = append(data, types.NV{
			Name:  s.t.Get("Total connections"),
			Value: match[1],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Total handshakes"),
			Value: match[2],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Total requests"),
			Value: match[3],
		})
	}

	match = regexp.MustCompile(`Reading:\s+(\d+)\s+Writing:\s+(\d+)\s+Waiting:\s+(\d+)`).FindStringSubmatch(raw)
	if len(match) == 4 {
		data = append(data, types.NV{
			Name:  s.t.Get("Reading"),
			Value: match[1],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Writing"),
			Value: match[2],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Waiting"),
			Value: match[3],
		})
	}

	return service.Success(c, data)
}
