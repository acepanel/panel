package nginx

import (
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/tools"
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
	r.Post("/config", s.SaveConfig)
	r.Get("/error_log", s.ErrorLog)
	r.Post("/clear_error_log", s.ClearErrorLog)

	r.Get("/stream/servers", s.ListStreamServers)
	r.Post("/stream/servers", s.CreateStreamServer)
	r.Put("/stream/servers/{name}", s.UpdateStreamServer)
	r.Delete("/stream/servers/{name}", s.DeleteStreamServer)
	r.Get("/stream/upstreams", s.ListStreamUpstreams)
	r.Post("/stream/upstreams", s.CreateStreamUpstream)
	r.Put("/stream/upstreams/{name}", s.UpdateStreamUpstream)
	r.Delete("/stream/upstreams/{name}", s.DeleteStreamUpstream)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

func (s *App) SaveConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root), req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

func (s *App) ErrorLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, fmt.Sprintf("%s/%s", app.Root, "server/nginx/nginx-error.log"))
}

func (s *App) ClearErrorLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf("cat /dev/null > %s/%s", app.Root, "server/nginx/nginx-error.log"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	client := resty.New().SetTimeout(10 * time.Second)
	resp, err := client.R().Get("http://127.0.0.1/nginx_status")
	if err != nil || !resp.IsSuccess() {
		service.Success(w, []types.NV{})
		return
	}

	raw := resp.String()
	var data []types.NV

	workers, err := shell.Execf("ps aux | grep nginx | grep 'worker process' | wc -l")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
		return
	}
	data = append(data, types.NV{
		Name:  s.t.Get("Workers"),
		Value: workers,
	})

	out, err := shell.Execf("ps aux | grep nginx | grep 'worker process' | awk '{memsum+=$6};END {print memsum}'")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
		return
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

	service.Success(w, data)
}
