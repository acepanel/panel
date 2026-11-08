package apache

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

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
)

var mpmEventRegexp = regexp.MustCompile(`(?s)<IfModule mpm_event_module>(.*?)</IfModule>`)

type App struct {
	t   *gotext.Locale
	log *slog.Logger
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		t:   t,
		log: log,
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
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "apache")
	return types.AggregateAppStatus(ok)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	path := app.Root + "/server/apache/conf/httpd.conf"
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "apache"), slog.String("path", path))

	service.Success(w, config)
}

func (s *App) SaveConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := app.Root + "/server/apache/conf/httpd.conf"
	if err = io.Write(path, req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "apache"), slog.String("path", path))

	service.Success(w, nil)
}

// reload 走方言的 Reload，错误里已带配置检查输出
func (s *App) reload(ctx context.Context) error {
	d, err := webserver.Get(webserver.TypeApache)
	if err != nil {
		return err
	}

	if err = d.Reload(ctx); err != nil {
		return fmt.Errorf("%s", s.t.Get("failed to reload apache: %v", err))
	}

	return nil
}

func (s *App) ErrorLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, fmt.Sprintf("%s/%s", app.Root, "server/apache/logs/error_log"))
}

func (s *App) ClearErrorLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf(r.Context(), "cat /dev/null > %s/%s", app.Root, "server/apache/logs/error_log"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app error log cleared"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "apache"), slog.String("path", app.Root+"/server/apache/logs/error_log"))

	service.Success(w, nil)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := shell.Execf(r.Context(), "curl -s http://127.0.0.1/server_status?auto 2>/dev/null || true")
	if err != nil {
		service.Success(w, []types.NV{})
		return
	}

	var data []types.NV

	workers, err := shell.Execf(r.Context(), "ps aux | grep httpd | grep -v grep | wc -l")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get apache workers: %v", err))
		return
	}
	data = append(data, types.NV{
		Name:  s.t.Get("Workers"),
		Value: workers,
	})

	out, err := shell.Execf(r.Context(), "ps aux | grep httpd | grep -v grep | awk '{memsum+=$6};END {print memsum}'")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get apache workers: %v", err))
		return
	}
	mem := tools.FormatBytes(cast.ToFloat64(out))
	data = append(data, types.NV{
		Name:  s.t.Get("Memory"),
		Value: mem,
	})

	// Parse server-status output
	if match := regexp.MustCompile(`Total Accesses:\s*(\d+)`).FindStringSubmatch(status); len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Total Accesses"),
			Value: match[1],
		})
	}

	if match := regexp.MustCompile(`Total kBytes:\s*(\d+)`).FindStringSubmatch(status); len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Total Traffic"),
			Value: tools.FormatBytes(cast.ToFloat64(match[1]) * 1024),
		})
	}

	if match := regexp.MustCompile(`BusyWorkers:\s*(\d+)`).FindStringSubmatch(status); len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Busy Workers"),
			Value: match[1],
		})
	}

	if match := regexp.MustCompile(`IdleWorkers:\s*(\d+)`).FindStringSubmatch(status); len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Idle Workers"),
			Value: match[1],
		})
	}

	if match := regexp.MustCompile(`ReqPerSec:\s*([\d.]+)`).FindStringSubmatch(status); len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Requests/sec"),
			Value: match[1],
		})
	}

	if match := regexp.MustCompile(`BytesPerSec:\s*([\d.]+)`).FindStringSubmatch(status); len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Bytes/sec"),
			Value: tools.FormatBytes(cast.ToFloat64(match[1])),
		})
	}

	service.Success(w, data)
}

// GetConfigTune 获取 Apache 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	confPath := app.Root + "/server/apache/conf/extra/httpd-default.conf"
	defaultConf, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	mpmConf, _ := io.Read(app.Root + "/server/apache/conf/extra/httpd-mpm.conf")
	eventBlock := ""
	if m := mpmEventRegexp.FindStringSubmatch(mpmConf); len(m) > 1 {
		eventBlock = m[1]
	}

	// 面板统一写入 httpd-default.conf，未写入过的从 httpd-mpm.conf 的 event 块读取默认值
	get := func(key string) string {
		if v := confval.Directive.Get(defaultConf, key); v != "" {
			return v
		}
		return confval.Directive.Get(eventBlock, key)
	}

	tune := ConfigTune{
		// MPM 事件模型
		StartServers:           get("StartServers"),
		MinSpareThreads:        get("MinSpareThreads"),
		MaxSpareThreads:        get("MaxSpareThreads"),
		ThreadsPerChild:        get("ThreadsPerChild"),
		MaxRequestWorkers:      get("MaxRequestWorkers"),
		MaxConnectionsPerChild: get("MaxConnectionsPerChild"),
		// 连接设置
		Timeout:              get("Timeout"),
		KeepAlive:            get("KeepAlive"),
		MaxKeepAliveRequests: get("MaxKeepAliveRequests"),
		KeepAliveTimeout:     get("KeepAliveTimeout"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "apache"), slog.String("path", confPath))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Apache 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := app.Root + "/server/apache/conf/extra/httpd-default.conf"
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// MPM 参数一并写入 httpd-default.conf，其 Include 顺序在 httpd-mpm.conf 之后，顶层定义覆盖块内默认值
	config = confval.Directive.Set(config, "StartServers", req.StartServers)
	config = confval.Directive.Set(config, "MinSpareThreads", req.MinSpareThreads)
	config = confval.Directive.Set(config, "MaxSpareThreads", req.MaxSpareThreads)
	config = confval.Directive.Set(config, "ThreadsPerChild", req.ThreadsPerChild)
	config = confval.Directive.Set(config, "MaxRequestWorkers", req.MaxRequestWorkers)
	config = confval.Directive.Set(config, "MaxConnectionsPerChild", req.MaxConnectionsPerChild)
	config = confval.Directive.Set(config, "Timeout", req.Timeout)
	config = confval.Directive.Set(config, "KeepAlive", req.KeepAlive)
	config = confval.Directive.Set(config, "MaxKeepAliveRequests", req.MaxKeepAliveRequests)
	config = confval.Directive.Set(config, "KeepAliveTimeout", req.KeepAliveTimeout)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", "apache"),
		slog.String("path", confPath),
		slog.String("start_servers", req.StartServers),
		slog.String("min_spare_threads", req.MinSpareThreads),
		slog.String("max_spare_threads", req.MaxSpareThreads),
		slog.String("threads_per_child", req.ThreadsPerChild),
		slog.String("max_request_workers", req.MaxRequestWorkers),
		slog.String("max_connections_per_child", req.MaxConnectionsPerChild),
		slog.String("timeout", req.Timeout),
		slog.String("keep_alive", req.KeepAlive),
		slog.String("max_keep_alive_requests", req.MaxKeepAliveRequests),
		slog.String("keep_alive_timeout", req.KeepAliveTimeout),
	)

	service.Success(w, nil)
}
