package opensearch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"go.yaml.in/yaml/v4"
	"resty.dev/v3"

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
	ok, _ := systemctl.Status(ctx, "opensearch")
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status(r.Context(), "opensearch")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get opensearch status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	port := s.getPort()
	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	resp, err := client.R().Get(fmt.Sprintf("http://127.0.0.1:%s/_cluster/health", port))
	if err != nil || !resp.IsStatusSuccess() {
		service.Success(w, []types.NV{})
		return
	}

	var health struct {
		ClusterName         string `json:"cluster_name"`
		Status              string `json:"status"`
		NumberOfNodes       int    `json:"number_of_nodes"`
		NumberOfDataNodes   int    `json:"number_of_data_nodes"`
		ActiveShards        int    `json:"active_shards"`
		ActivePrimaryShards int    `json:"active_primary_shards"`
		RelocatingShards    int    `json:"relocating_shards"`
		UnassignedShards    int    `json:"unassigned_shards"`
	}
	if err = json.Unmarshal(resp.Bytes(), &health); err != nil {
		service.Success(w, []types.NV{})
		return
	}

	data := []types.NV{
		{Name: s.t.Get("Cluster Name"), Value: health.ClusterName},
		{Name: s.t.Get("Cluster Status"), Value: health.Status},
		{Name: s.t.Get("Number of Nodes"), Value: cast.ToString(health.NumberOfNodes)},
		{Name: s.t.Get("Number of Data Nodes"), Value: cast.ToString(health.NumberOfDataNodes)},
		{Name: s.t.Get("Active Shards"), Value: cast.ToString(health.ActiveShards)},
		{Name: s.t.Get("Active Primary Shards"), Value: cast.ToString(health.ActivePrimaryShards)},
		{Name: s.t.Get("Relocating Shards"), Value: cast.ToString(health.RelocatingShards)},
		{Name: s.t.Get("Unassigned Shards"), Value: cast.ToString(health.UnassignedShards)},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "opensearch"), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "opensearch", s.configPath(), 0644, "opensearch")
}

// GetConfigTune 获取 OpenSearch 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	_ = yaml.Unmarshal([]byte(raw), &cfg)
	if cfg == nil {
		cfg = make(map[string]any)
	}

	jvmRaw, _ := io.Read(s.jvmOptionsPath())
	heapInit, heapMax := s.parseJVMHeap(jvmRaw)

	tune := ConfigTune{
		ClusterName:   confval.GetYAML(cfg, "cluster.name"),
		NodeName:      confval.GetYAML(cfg, "node.name"),
		NetworkHost:   confval.GetYAML(cfg, "network.host"),
		HTTPPort:      confval.GetYAML(cfg, "http.port"),
		DiscoveryType: confval.GetYAML(cfg, "discovery.type"),
		PathData:      confval.GetYAML(cfg, "path.data"),
		PathLogs:      confval.GetYAML(cfg, "path.logs"),
		HeapInitSize:  heapInit,
		HeapMaxSize:   heapMax,
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "opensearch"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 OpenSearch 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	if err = yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		cfg = make(map[string]any)
	}

	confval.SetYAML(cfg, "cluster.name", req.ClusterName)
	confval.SetYAML(cfg, "node.name", req.NodeName)
	confval.SetYAML(cfg, "network.host", req.NetworkHost)
	confval.SetYAML(cfg, "http.port", req.HTTPPort)
	confval.SetYAML(cfg, "discovery.type", req.DiscoveryType)
	confval.SetYAML(cfg, "path.data", req.PathData)
	confval.SetYAML(cfg, "path.logs", req.PathLogs)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Write(s.configPath(), string(data), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if req.HeapInitSize != "" || req.HeapMaxSize != "" {
		jvmRaw, _ := io.Read(s.jvmOptionsPath())
		jvmRaw = s.setJVMHeap(jvmRaw, req.HeapInitSize, req.HeapMaxSize)
		if err = io.Write(s.jvmOptionsPath(), jvmRaw, 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = systemctl.Restart(r.Context(), "opensearch"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "opensearch"), slog.String("cluster_name", req.ClusterName), slog.String("node_name", req.NodeName), slog.String("network_host", req.NetworkHost), slog.String("http_port", req.HTTPPort), slog.String("discovery_type", req.DiscoveryType), slog.String("path_data", req.PathData), slog.String("path_logs", req.PathLogs), slog.String("heap_init_size", req.HeapInitSize), slog.String("heap_max_size", req.HeapMaxSize))

	service.Success(w, nil)
}

func (s *App) configPath() string {
	return app.Root + "/server/opensearch/config/opensearch.yml"
}

func (s *App) jvmOptionsPath() string {
	return app.Root + "/server/opensearch/config/jvm.options"
}

func (s *App) getPort() string {
	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	_ = yaml.Unmarshal([]byte(raw), &cfg)
	if cfg != nil {
		if v := confval.GetYAML(cfg, "http.port"); v != "" {
			return v
		}
	}
	return "9200"
}

func (s *App) parseJVMHeap(content string) (initSize, maxSize string) {
	reInit := regexp.MustCompile(`(?m)^-Xms(\S+)`)
	reMax := regexp.MustCompile(`(?m)^-Xmx(\S+)`)
	if m := reInit.FindStringSubmatch(content); len(m) == 2 {
		initSize = m[1]
	}
	if m := reMax.FindStringSubmatch(content); len(m) == 2 {
		maxSize = m[1]
	}
	return
}

func (s *App) setJVMHeap(content string, initSize, maxSize string) string {
	if initSize != "" {
		re := regexp.MustCompile(`(?m)^-Xms\S+`)
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, "-Xms"+initSize)
		} else {
			content += "\n-Xms" + initSize
		}
	}
	if maxSize != "" {
		re := regexp.MustCompile(`(?m)^-Xmx\S+`)
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, "-Xmx"+maxSize)
		} else {
			content += "\n-Xmx" + maxSize
		}
	}
	return content
}
