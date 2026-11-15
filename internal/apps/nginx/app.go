package nginx

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	webserverNginx "github.com/acepanel/panel/v3/pkg/webserver/nginx"
)

type App struct {
	*common.WebServer
	t    *gotext.Locale
	log  *slog.Logger
	slug string
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return New("nginx", t, log)
}

func New(slug string, t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		WebServer: common.NewWebServer(t, log, slug, webserver.TypeNginx, configPath(), app.Root+"/server/nginx/nginx-error.log"),
		t:         t,
		log:       log,
		slug:      slug,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.SaveConfig)
	r.Get("/error_log", s.ErrorLog)
	r.Post("/clear_error_log", s.ClearErrorLog)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)

	r.Get("/stream/servers", s.ListStreamServers)
	r.Post("/stream/servers", s.CreateStreamServer)
	r.Put("/stream/servers/{name}", s.UpdateStreamServer)
	r.Delete("/stream/servers/{name}", s.DeleteStreamServer)
	r.Get("/stream/upstreams", s.ListStreamUpstreams)
	r.Post("/stream/upstreams", s.CreateStreamUpstream)
	r.Put("/stream/upstreams/{name}", s.UpdateStreamUpstream)
	r.Delete("/stream/upstreams/{name}", s.DeleteStreamUpstream)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "nginx")
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	resp, err := client.R().Get("http://127.0.0.1/nginx_status")
	if err != nil || !resp.IsStatusSuccess() {
		service.Success(w, []types.NV{})
		return
	}

	raw := resp.String()
	var data []types.NV

	workers, err := shell.Execf(r.Context(), "ps aux | grep nginx | grep 'worker process' | wc -l")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
		return
	}
	data = append(data, types.NV{
		Name:  s.t.Get("Workers"),
		Value: workers,
	})

	out, err := shell.Execf(r.Context(), "ps aux | grep nginx | grep 'worker process' | awk '{memsum+=$6};END {print memsum}'")
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

// GetConfigTune 获取 Nginx 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	confPath := configPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := &ConfigTune{Reuseport: webserverNginx.DefaultSiteReuseport()}
	confval.Nginx.Read(config, s.tuneFields(tune))

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", confPath))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Nginx 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := configPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = confval.Nginx.Write(config, s.tuneFields(req))

	if err = io.Write(confPath, config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if req.Reuseport != webserverNginx.DefaultSiteReuseport() {
		if err = webserverNginx.SetDefaultSiteReuseport(req.Reuseport); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = s.Reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", s.slug),
		slog.String("path", confPath),
		slog.String("worker_processes", req.WorkerProcesses),
		slog.String("worker_connections", req.WorkerConnections),
		slog.String("keepalive_timeout", req.KeepaliveTimeout),
		slog.String("client_max_body_size", req.ClientMaxBodySize),
		slog.Bool("reuseport", req.Reuseport),
		slog.String("gzip", req.Gzip),
		slog.String("brotli", req.Brotli),
		slog.String("zstd", req.Zstd),
	)

	service.Success(w, nil)
}

func (s *App) tuneFields(t *ConfigTune) []confval.Field {
	return []confval.Field{
		// 常规设置
		{Key: "worker_processes", Value: &t.WorkerProcesses},
		{Key: "worker_connections", Value: &t.WorkerConnections},
		{Key: "keepalive_timeout", Value: &t.KeepaliveTimeout},
		{Key: "client_max_body_size", Value: &t.ClientMaxBodySize},
		{Key: "client_body_buffer_size", Value: &t.ClientBodyBufferSize},
		{Key: "client_header_buffer_size", Value: &t.ClientHeaderBufferSize},
		{Key: "server_names_hash_bucket_size", Value: &t.ServerNamesHashBucketSize},
		{Key: "server_tokens", Value: &t.ServerTokens},
		// Gzip 压缩
		{Key: "gzip", Value: &t.Gzip},
		{Key: "gzip_min_length", Value: &t.GzipMinLength},
		{Key: "gzip_comp_level", Value: &t.GzipCompLevel},
		{Key: "gzip_types", Value: &t.GzipTypes},
		{Key: "gzip_vary", Value: &t.GzipVary},
		{Key: "gzip_proxied", Value: &t.GzipProxied},
		// Brotli 压缩
		{Key: "brotli", Value: &t.Brotli},
		{Key: "brotli_min_length", Value: &t.BrotliMinLength},
		{Key: "brotli_comp_level", Value: &t.BrotliCompLevel},
		{Key: "brotli_types", Value: &t.BrotliTypes},
		{Key: "brotli_static", Value: &t.BrotliStatic},
		// Zstd 压缩
		{Key: "zstd", Value: &t.Zstd},
		{Key: "zstd_min_length", Value: &t.ZstdMinLength},
		{Key: "zstd_comp_level", Value: &t.ZstdCompLevel},
		{Key: "zstd_types", Value: &t.ZstdTypes},
		{Key: "zstd_static", Value: &t.ZstdStatic},
	}
}

func configPath() string {
	return app.Root + "/server/nginx/conf/nginx.conf"
}
