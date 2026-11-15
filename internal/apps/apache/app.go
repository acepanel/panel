package apache

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

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
)

var mpmEventRegexp = regexp.MustCompile(`(?s)<IfModule mpm_event_module>(.*?)</IfModule>`)

type App struct {
	*common.WebServer
	t   *gotext.Locale
	log *slog.Logger
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		WebServer: common.NewWebServer(t, log, "apache", webserver.TypeApache, app.Root+"/server/apache/conf/httpd.conf", app.Root+"/server/apache/logs/error_log"),
		t:         t,
		log:       log,
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
	var eventBlock string
	if m := mpmEventRegexp.FindStringSubmatch(mpmConf); len(m) > 1 {
		eventBlock = m[1]
	}

	// 面板统一写入 httpd-default.conf，未写入过的从 httpd-mpm.conf 的 event 块读取默认值
	tune := new(ConfigTune)
	fields := s.tuneFields(tune)
	confval.Directive.Read(defaultConf, fields)
	for _, f := range fields {
		if *f.Value == "" {
			*f.Value = confval.Directive.Get(eventBlock, f.Key)
		}
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
	config = confval.Directive.Write(config, s.tuneFields(req))

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.Reload(r.Context()); err != nil {
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

func (s *App) tuneFields(t *ConfigTune) []confval.Field {
	return []confval.Field{
		// MPM 事件模型
		{Key: "StartServers", Value: &t.StartServers},
		{Key: "MinSpareThreads", Value: &t.MinSpareThreads},
		{Key: "MaxSpareThreads", Value: &t.MaxSpareThreads},
		{Key: "ThreadsPerChild", Value: &t.ThreadsPerChild},
		{Key: "MaxRequestWorkers", Value: &t.MaxRequestWorkers},
		{Key: "MaxConnectionsPerChild", Value: &t.MaxConnectionsPerChild},
		// 连接设置
		{Key: "Timeout", Value: &t.Timeout},
		{Key: "KeepAlive", Value: &t.KeepAlive},
		{Key: "MaxKeepAliveRequests", Value: &t.MaxKeepAliveRequests},
		{Key: "KeepAliveTimeout", Value: &t.KeepAliveTimeout},
	}
}
