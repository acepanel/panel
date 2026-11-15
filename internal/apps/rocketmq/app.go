package rocketmq

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

const (
	namesrvHeapKey = "ROCKETMQ_NAMESRV_HEAP"
	brokerHeapKey  = "ROCKETMQ_BROKER_HEAP"
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
	namesrv, _ := systemctl.Status(ctx, "rocketmq-namesrv")
	broker, _ := systemctl.Status(ctx, "rocketmq-broker")
	return types.AggregateAppStatus(namesrv, broker)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	namesrvStatus, _ := systemctl.Status(r.Context(), "rocketmq-namesrv")
	brokerStatus, _ := systemctl.Status(r.Context(), "rocketmq-broker")

	namesrvStr := "stopped"
	if namesrvStatus {
		namesrvStr = "running"
	}
	brokerStr := "stopped"
	if brokerStatus {
		brokerStr = "running"
	}

	if !namesrvStatus && !brokerStatus {
		service.Success(w, []types.NV{})
		return
	}

	config, _ := io.Read(s.configPath())

	data := []types.NV{
		{Name: s.t.Get("NameServer Status"), Value: namesrvStr},
		{Name: s.t.Get("Broker Status"), Value: brokerStr},
		{Name: s.t.Get("Broker Name"), Value: confval.Properties.Get(config, "brokerName")},
		{Name: s.t.Get("Listen Port"), Value: confval.Properties.Get(config, "listenPort")},
		{Name: s.t.Get("NameServer Address"), Value: confval.Properties.Get(config, "namesrvAddr")},
		{Name: s.t.Get("Broker Role"), Value: confval.Properties.Get(config, "brokerRole")},
		{Name: s.t.Get("Flush Disk Type"), Value: confval.Properties.Get(config, "flushDiskType")},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rocketmq"), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "rocketmq", s.configPath(), 0644, s.restart)
}

// GetConfigTune 获取 RocketMQ 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, _ := io.Read(s.configPath())
	heapRaw, _ := io.Read(s.heapEnvPath())

	tune := new(ConfigTune)
	confval.Properties.Read(config, s.tuneFields(tune))
	tune.NamesrvHeapInitSize, tune.NamesrvHeapMaxSize = s.heapOpts(heapRaw, namesrvHeapKey)
	tune.BrokerHeapInitSize, tune.BrokerHeapMaxSize = s.heapOpts(heapRaw, brokerHeapKey)

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rocketmq"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 RocketMQ 配置调整参数
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
	if req.NamesrvHeapInitSize != "" || req.NamesrvHeapMaxSize != "" || req.BrokerHeapInitSize != "" || req.BrokerHeapMaxSize != "" {
		heapRaw, _ := io.Read(s.heapEnvPath())
		if err = io.Write(s.heapEnvPath(), s.setHeapEnv(heapRaw, req), 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = s.restart(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"),
		slog.String("type", biz.OperationTypeApp),
		slog.Uint64("operator_id", biz.OperatorID(r.Context())),
		slog.String("app", "rocketmq"),
		slog.String("broker_name", req.BrokerName),
		slog.String("listen_port", req.ListenPort),
		slog.String("namesrv_addr", req.NamesrvAddr),
		slog.String("broker_role", req.BrokerRole),
		slog.String("flush_disk_type", req.FlushDiskType),
		slog.String("store_path_root_dir", req.StorePathRootDir),
		slog.String("max_message_size", req.MaxMessageSize),
	)

	service.Success(w, nil)
}

// configPath 返回 broker 配置文件路径
func (s *App) configPath() string {
	return app.Root + "/server/rocketmq/conf/broker.conf"
}

// heapEnvPath 返回 JVM 堆内存配置文件路径
func (s *App) heapEnvPath() string {
	return app.Root + "/server/rocketmq/conf/heap.env"
}

// restart 依次重启 NameServer 和 Broker
func (s *App) restart(ctx context.Context) error {
	return common.Restart("rocketmq-namesrv", "rocketmq-broker")(ctx)
}

func (s *App) tuneFields(t *ConfigTune) []confval.Field {
	return []confval.Field{
		{Key: "brokerName", Value: &t.BrokerName},
		{Key: "listenPort", Value: &t.ListenPort},
		{Key: "namesrvAddr", Value: &t.NamesrvAddr},
		{Key: "brokerRole", Value: &t.BrokerRole},
		{Key: "flushDiskType", Value: &t.FlushDiskType},
		{Key: "storePathRootDir", Value: &t.StorePathRootDir},
		{Key: "storePathCommitLog", Value: &t.StorePathCommitLog},
		{Key: "maxMessageSize", Value: &t.MaxMessageSize},
	}
}

// heapOpts 从 heap.env 中提取指定环境变量的堆内存配置
func (s *App) heapOpts(content string, envKey string) (initSize, maxSize string) {
	return confval.JVMHeap(confval.Properties.Get(content, envKey))
}

// setHeapEnv 生成 heap.env 的内容，未填的项沿用已有值
func (s *App) setHeapEnv(content string, req *ConfigTune) string {
	nsInit, nsMax := s.heapOpts(content, namesrvHeapKey)
	brInit, brMax := s.heapOpts(content, brokerHeapKey)

	return fmt.Sprintf("%s=-Xms%s -Xmx%s\n%s=-Xms%s -Xmx%s\n",
		namesrvHeapKey, lo.CoalesceOrEmpty(req.NamesrvHeapInitSize, nsInit, "512m"), lo.CoalesceOrEmpty(req.NamesrvHeapMaxSize, nsMax, "512m"),
		brokerHeapKey, lo.CoalesceOrEmpty(req.BrokerHeapInitSize, brInit, "1g"), lo.CoalesceOrEmpty(req.BrokerHeapMaxSize, brMax, "1g"))
}
