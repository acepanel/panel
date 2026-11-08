package rocketmq

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
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
	conf, _ := io.Read(s.configPath())
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rocketmq"), slog.String("path", s.configPath()))
	service.Success(w, conf)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(s.configPath(), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = s.restartServices(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "rocketmq"), slog.String("path", s.configPath()))

	service.Success(w, nil)
}

// GetConfigTune 获取 RocketMQ 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, _ := io.Read(s.configPath())
	heapRaw, _ := io.Read(s.heapEnvPath())

	namesrvInit, namesrvMax := s.parseHeapLine(heapRaw, "ROCKETMQ_NAMESRV_HEAP")
	brokerInit, brokerMax := s.parseHeapLine(heapRaw, "ROCKETMQ_BROKER_HEAP")

	tune := ConfigTune{
		BrokerName:          confval.Properties.Get(config, "brokerName"),
		ListenPort:          confval.Properties.Get(config, "listenPort"),
		NamesrvAddr:         confval.Properties.Get(config, "namesrvAddr"),
		BrokerRole:          confval.Properties.Get(config, "brokerRole"),
		FlushDiskType:       confval.Properties.Get(config, "flushDiskType"),
		StorePathRootDir:    confval.Properties.Get(config, "storePathRootDir"),
		StorePathCommitLog:  confval.Properties.Get(config, "storePathCommitLog"),
		MaxMessageSize:      confval.Properties.Get(config, "maxMessageSize"),
		NamesrvHeapInitSize: namesrvInit,
		NamesrvHeapMaxSize:  namesrvMax,
		BrokerHeapInitSize:  brokerInit,
		BrokerHeapMaxSize:   brokerMax,
	}

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

	config = confval.Properties.Set(config, "brokerName", req.BrokerName)
	config = confval.Properties.Set(config, "listenPort", req.ListenPort)
	config = confval.Properties.Set(config, "namesrvAddr", req.NamesrvAddr)
	config = confval.Properties.Set(config, "brokerRole", req.BrokerRole)
	config = confval.Properties.Set(config, "flushDiskType", req.FlushDiskType)
	config = confval.Properties.Set(config, "storePathRootDir", req.StorePathRootDir)
	config = confval.Properties.Set(config, "storePathCommitLog", req.StorePathCommitLog)
	config = confval.Properties.Set(config, "maxMessageSize", req.MaxMessageSize)

	if err = io.Write(s.configPath(), config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新 JVM 堆内存
	if req.NamesrvHeapInitSize != "" || req.NamesrvHeapMaxSize != "" || req.BrokerHeapInitSize != "" || req.BrokerHeapMaxSize != "" {
		heapRaw, _ := io.Read(s.heapEnvPath())
		heapRaw = s.setHeapEnv(heapRaw, *req)
		if err = io.Write(s.heapEnvPath(), heapRaw, 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = s.restartServices(r.Context()); err != nil {
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

// restartServices 重启 NameServer 和 Broker 服务
func (s *App) restartServices(ctx context.Context) error {
	// 配置已落盘，且 namesrv 与 broker 要一起重启，整段不跟随请求取消
	ctx = context.WithoutCancel(ctx)
	if err := systemctl.Restart(ctx, "rocketmq-namesrv"); err != nil {
		return err
	}
	return systemctl.Restart(ctx, "rocketmq-broker")
}

// configPath 返回 broker 配置文件路径
func (s *App) configPath() string {
	return app.Root + "/server/rocketmq/conf/broker.conf"
}

// heapEnvPath 返回 JVM 堆内存配置文件路径
func (s *App) heapEnvPath() string {
	return app.Root + "/server/rocketmq/conf/heap.env"
}

// parseHeapLine 从 heap.env 中提取指定环境变量的堆内存配置
func (s *App) parseHeapLine(content string, envKey string) (initSize, maxSize string) {
	re := regexp.MustCompile(envKey + `=(.+)`)
	m := re.FindStringSubmatch(content)
	if len(m) != 2 {
		return
	}
	opts := m[1]
	if mi := regexp.MustCompile(`-Xms(\S+)`).FindStringSubmatch(opts); len(mi) == 2 {
		initSize = mi[1]
	}
	if mx := regexp.MustCompile(`-Xmx(\S+)`).FindStringSubmatch(opts); len(mx) == 2 {
		maxSize = mx[1]
	}
	return
}

// setHeapEnv 设置 heap.env 中的堆内存配置
func (s *App) setHeapEnv(content string, req ConfigTune) string {
	// 读取已有值作为默认
	oldNsInit, oldNsMax := s.parseHeapLine(content, "ROCKETMQ_NAMESRV_HEAP")
	oldBrInit, oldBrMax := s.parseHeapLine(content, "ROCKETMQ_BROKER_HEAP")

	nsInit := req.NamesrvHeapInitSize
	if nsInit == "" {
		nsInit = oldNsInit
	}
	if nsInit == "" {
		nsInit = "512m"
	}
	nsMax := req.NamesrvHeapMaxSize
	if nsMax == "" {
		nsMax = oldNsMax
	}
	if nsMax == "" {
		nsMax = "512m"
	}

	brInit := req.BrokerHeapInitSize
	if brInit == "" {
		brInit = oldBrInit
	}
	if brInit == "" {
		brInit = "1g"
	}
	brMax := req.BrokerHeapMaxSize
	if brMax == "" {
		brMax = oldBrMax
	}
	if brMax == "" {
		brMax = "1g"
	}

	return fmt.Sprintf("ROCKETMQ_NAMESRV_HEAP=-Xms%s -Xmx%s\nROCKETMQ_BROKER_HEAP=-Xms%s -Xmx%s\n", nsInit, nsMax, brInit, brMax)
}
