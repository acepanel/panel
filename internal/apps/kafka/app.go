package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t   *gotext.Locale
	log *slog.Logger
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{t: t, log: log}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "kafka")
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status(r.Context(), "kafka")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get kafka status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	config, _ := io.Read(s.configPath())

	data := []types.NV{
		{Name: s.t.Get("Node ID"), Value: confval.Properties.Get(config, "node.id")},
		{Name: s.t.Get("Listeners"), Value: confval.Properties.Get(config, "listeners")},
		{Name: s.t.Get("Log Dirs"), Value: confval.Properties.Get(config, "log.dirs")},
		{Name: s.t.Get("Num Partitions"), Value: confval.Properties.Get(config, "num.partitions")},
		{Name: s.t.Get("Log Retention Hours"), Value: confval.Properties.Get(config, "log.retention.hours")},
		{Name: s.t.Get("Log Segment Bytes"), Value: confval.Properties.Get(config, "log.segment.bytes")},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "kafka"), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "kafka", s.configPath(), 0644, common.Restart("kafka"))
}

// GetConfigTune 获取 Kafka 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, _ := io.Read(s.configPath())

	heapRaw, _ := io.Read(s.heapEnvPath())

	tune := new(ConfigTune)
	confval.Properties.Read(config, s.tuneFields(tune))
	tune.HeapInitSize, tune.HeapMaxSize = s.heapOpts(heapRaw)

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "kafka"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Kafka 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, _ := io.Read(s.configPath())

	config = confval.Properties.Write(config, s.tuneFields(req))

	if err = io.Write(s.configPath(), config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新 JVM 堆内存
	if req.HeapInitSize != "" || req.HeapMaxSize != "" {
		heapRaw, _ := io.Read(s.heapEnvPath())
		if err = io.Write(s.heapEnvPath(), s.setHeapEnv(heapRaw, req.HeapInitSize, req.HeapMaxSize), 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = systemctl.Restart(r.Context(), "kafka"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "kafka"), slog.String("node_id", req.NodeID), slog.String("listeners", req.Listeners), slog.String("log_dirs", req.LogDirs), slog.String("num_partitions", req.NumPartitions), slog.String("retention_hours", req.RetentionHours), slog.String("log_segment_bytes", req.LogSegmentBytes), slog.String("heap_init_size", req.HeapInitSize), slog.String("heap_max_size", req.HeapMaxSize))

	service.Success(w, nil)
}

// configPath 返回配置文件路径
func (s *App) configPath() string {
	return app.Root + "/server/kafka/config/server.properties"
}

// heapEnvPath 返回 JVM 堆内存配置文件路径
func (s *App) heapEnvPath() string {
	return app.Root + "/server/kafka/config/heap.env"
}

func (s *App) tuneFields(t *ConfigTune) []confval.Field {
	return []confval.Field{
		{Key: "node.id", Value: &t.NodeID},
		{Key: "listeners", Value: &t.Listeners},
		{Key: "log.dirs", Value: &t.LogDirs},
		{Key: "num.partitions", Value: &t.NumPartitions},
		{Key: "log.retention.hours", Value: &t.RetentionHours},
		{Key: "log.segment.bytes", Value: &t.LogSegmentBytes},
	}
}

// heapOpts 从 heap.env 中提取堆内存配置
func (s *App) heapOpts(content string) (initSize, maxSize string) {
	return confval.JVMHeap(confval.Properties.Get(content, "KAFKA_HEAP_OPTS"))
}

// setHeapEnv 生成 heap.env 的内容，未填的项沿用已有值
func (s *App) setHeapEnv(content string, initSize, maxSize string) string {
	oldInit, oldMax := s.heapOpts(content)

	return fmt.Sprintf("KAFKA_HEAP_OPTS=-Xms%s -Xmx%s\n", lo.CoalesceOrEmpty(initSize, oldInit, "1g"), lo.CoalesceOrEmpty(maxSize, oldMax, "1g"))
}
