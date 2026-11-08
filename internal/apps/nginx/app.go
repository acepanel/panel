package nginx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
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
	t    *gotext.Locale
	log  *slog.Logger
	slug string
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return New("nginx", t, log)
}

func New(slug string, t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		t:    t,
		log:  log,
		slug: slug,
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

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	path := app.Root + "/server/nginx/conf/nginx.conf"
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", path))

	service.Success(w, config)
}

func (s *App) SaveConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := app.Root + "/server/nginx/conf/nginx.conf"
	if err = io.Write(path, req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", path))

	service.Success(w, nil)
}

func (s *App) ErrorLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, fmt.Sprintf("%s/%s", app.Root, "server/nginx/nginx-error.log"))
}

func (s *App) ClearErrorLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf(r.Context(), "cat /dev/null > %s/%s", app.Root, "server/nginx/nginx-error.log"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app error log cleared"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", app.Root+"/server/nginx/nginx-error.log"))

	service.Success(w, nil)
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
	confPath := app.Root + "/server/nginx/conf/nginx.conf"
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		// 常规设置
		WorkerProcesses:           confval.Nginx.Get(config, "worker_processes"),
		WorkerConnections:         confval.Nginx.Get(config, "worker_connections"),
		KeepaliveTimeout:          confval.Nginx.Get(config, "keepalive_timeout"),
		ClientMaxBodySize:         confval.Nginx.Get(config, "client_max_body_size"),
		ClientBodyBufferSize:      confval.Nginx.Get(config, "client_body_buffer_size"),
		ClientHeaderBufferSize:    confval.Nginx.Get(config, "client_header_buffer_size"),
		ServerNamesHashBucketSize: confval.Nginx.Get(config, "server_names_hash_bucket_size"),
		ServerTokens:              confval.Nginx.Get(config, "server_tokens"),
		Reuseport:                 webserverNginx.DefaultSiteReuseport(),
		// Gzip 压缩
		Gzip:          confval.Nginx.Get(config, "gzip"),
		GzipMinLength: confval.Nginx.Get(config, "gzip_min_length"),
		GzipCompLevel: confval.Nginx.Get(config, "gzip_comp_level"),
		GzipTypes:     confval.Nginx.Get(config, "gzip_types"),
		GzipVary:      confval.Nginx.Get(config, "gzip_vary"),
		GzipProxied:   confval.Nginx.Get(config, "gzip_proxied"),
		// Brotli 压缩
		Brotli:          confval.Nginx.Get(config, "brotli"),
		BrotliMinLength: confval.Nginx.Get(config, "brotli_min_length"),
		BrotliCompLevel: confval.Nginx.Get(config, "brotli_comp_level"),
		BrotliTypes:     confval.Nginx.Get(config, "brotli_types"),
		BrotliStatic:    confval.Nginx.Get(config, "brotli_static"),
		// Zstd 压缩
		Zstd:          confval.Nginx.Get(config, "zstd"),
		ZstdMinLength: confval.Nginx.Get(config, "zstd_min_length"),
		ZstdCompLevel: confval.Nginx.Get(config, "zstd_comp_level"),
		ZstdTypes:     confval.Nginx.Get(config, "zstd_types"),
		ZstdStatic:    confval.Nginx.Get(config, "zstd_static"),
	}

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

	confPath := app.Root + "/server/nginx/conf/nginx.conf"
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新常规设置
	config = confval.Nginx.Set(config, "worker_processes", req.WorkerProcesses)
	config = confval.Nginx.Set(config, "worker_connections", req.WorkerConnections)
	config = confval.Nginx.Set(config, "keepalive_timeout", req.KeepaliveTimeout)
	config = confval.Nginx.Set(config, "client_max_body_size", req.ClientMaxBodySize)
	config = confval.Nginx.Set(config, "client_body_buffer_size", req.ClientBodyBufferSize)
	config = confval.Nginx.Set(config, "client_header_buffer_size", req.ClientHeaderBufferSize)
	config = confval.Nginx.Set(config, "server_names_hash_bucket_size", req.ServerNamesHashBucketSize)
	config = confval.Nginx.Set(config, "server_tokens", req.ServerTokens)
	// 更新 Gzip 压缩
	config = confval.Nginx.Set(config, "gzip", req.Gzip)
	config = confval.Nginx.Set(config, "gzip_min_length", req.GzipMinLength)
	config = confval.Nginx.Set(config, "gzip_comp_level", req.GzipCompLevel)
	config = confval.Nginx.Set(config, "gzip_types", req.GzipTypes)
	config = confval.Nginx.Set(config, "gzip_vary", req.GzipVary)
	config = confval.Nginx.Set(config, "gzip_proxied", req.GzipProxied)
	// 更新 Brotli 压缩
	config = confval.Nginx.Set(config, "brotli", req.Brotli)
	config = confval.Nginx.Set(config, "brotli_min_length", req.BrotliMinLength)
	config = confval.Nginx.Set(config, "brotli_comp_level", req.BrotliCompLevel)
	config = confval.Nginx.Set(config, "brotli_types", req.BrotliTypes)
	config = confval.Nginx.Set(config, "brotli_static", req.BrotliStatic)
	// 更新 Zstd 压缩
	config = confval.Nginx.Set(config, "zstd", req.Zstd)
	config = confval.Nginx.Set(config, "zstd_min_length", req.ZstdMinLength)
	config = confval.Nginx.Set(config, "zstd_comp_level", req.ZstdCompLevel)
	config = confval.Nginx.Set(config, "zstd_types", req.ZstdTypes)
	config = confval.Nginx.Set(config, "zstd_static", req.ZstdStatic)

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

	if err = s.reload(r.Context()); err != nil {
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

// reload 重载 nginx，失败时删除 rollback 指定的配置文件
// 走方言的 Reload，错误里已带配置检查输出
func (s *App) reload(ctx context.Context, rollback ...string) error {
	d, err := webserver.Get(webserver.TypeNginx)
	if err != nil {
		return err
	}

	if err = d.Reload(ctx); err != nil {
		for _, path := range rollback {
			_ = os.Remove(path)
		}
		return errors.New(s.t.Get("failed to reload nginx: %v", err))
	}

	return nil
}
