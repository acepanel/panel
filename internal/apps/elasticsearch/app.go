package elasticsearch

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
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

var (
	heapInitLineRegexp = regexp.MustCompile(`(?m)^-Xms\S+`)
	heapMaxLineRegexp  = regexp.MustCompile(`(?m)^-Xmx\S+`)
)

type App struct {
	t                  *gotext.Locale
	log                *slog.Logger
	databaseServerRepo biz.DatabaseServerRepo
	slug               string   // 服务名与安装目录名，如 elasticsearch、opensearch
	name               string   // 展示名，如 Elasticsearch、OpenSearch
	heapFiles          []string // 相对 config 目录的 JVM 堆内存配置文件
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo) *App {
	// ES 9.x 不再在 jvm.options 中设置堆内存，改为 jvm.options.d/heap.options，旧版本的仍留在 jvm.options
	return New("elasticsearch", "Elasticsearch", []string{"jvm.options.d/heap.options", "jvm.options"}, t, log, databaseServerRepo)
}

// New 构造 Elasticsearch 协议兼容服务的应用
// heapFiles 读取时按顺序取第一个带堆内存设置的文件，写入时只写第一个
func New(
	slug, name string,
	heapFiles []string,
	t *gotext.Locale,
	log *slog.Logger,
	databaseServerRepo biz.DatabaseServerRepo,
) *App {
	return &App{
		t:                  t,
		log:                log,
		databaseServerRepo: databaseServerRepo,
		slug:               slug,
		name:               name,
		heapFiles:          heapFiles,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, s.slug)
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status(r.Context(), s.slug)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get %s status: %v", s.name, err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	resp, err := client.R().Get("http://127.0.0.1:" + s.port() + "/_cluster/health")
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
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, s.slug, s.configPath(), 0644, common.Restart(s.slug))
}

// GetConfigTune 获取配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	cfg, _ := confval.LoadYAML(s.configPath())

	var heapInit, heapMax string
	for _, file := range s.heapFiles {
		raw, _ := io.Read(s.configDir(file))
		if heapInit, heapMax = confval.JVMHeap(raw); heapInit != "" || heapMax != "" {
			break
		}
	}

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

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	cfg, err := confval.LoadYAML(s.configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
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
		heapPath := s.configDir(s.heapFiles[0])
		heapRaw, _ := io.Read(heapPath)
		if err = io.Write(heapPath, s.setJVMHeap(heapRaw, req.HeapInitSize, req.HeapMaxSize), 0644); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = systemctl.Restart(r.Context(), s.slug); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// http.port 可以写成端口范围，解析不出单个端口时不动面板记录
	if port := cast.ToUint(s.port()); port > 0 {
		if err = s.databaseServerRepo.UpdatePort("local_"+s.slug, port); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("cluster_name", req.ClusterName), slog.String("node_name", req.NodeName), slog.String("network_host", req.NetworkHost), slog.String("http_port", req.HTTPPort), slog.String("discovery_type", req.DiscoveryType), slog.String("path_data", req.PathData), slog.String("path_logs", req.PathLogs), slog.String("heap_init_size", req.HeapInitSize), slog.String("heap_max_size", req.HeapMaxSize))

	service.Success(w, nil)
}

// port 从配置中获取 HTTP 端口
func (s *App) port() string {
	cfg, _ := confval.LoadYAML(s.configPath())
	if port := confval.GetYAML(cfg, "http.port"); port != "" {
		return port
	}

	return "9200"
}

// configPath 返回主配置文件路径
func (s *App) configPath() string {
	return s.configDir(s.slug + ".yml")
}

// configDir 返回 config 目录下的文件路径
func (s *App) configDir(file string) string {
	return filepath.Join(app.Root, "server", s.slug, "config", file)
}

// setJVMHeap 替换 jvm.options 格式内容中的堆内存配置，没有则追加
func (s *App) setJVMHeap(content string, initSize, maxSize string) string {
	if initSize != "" {
		if heapInitLineRegexp.MatchString(content) {
			content = heapInitLineRegexp.ReplaceAllString(content, "-Xms"+initSize)
		} else {
			content += "\n-Xms" + initSize
		}
	}
	if maxSize != "" {
		if heapMaxLineRegexp.MatchString(content) {
			content = heapMaxLineRegexp.ReplaceAllString(content, "-Xmx"+maxSize)
		} else {
			content += "\n-Xmx" + maxSize
		}
	}

	return content
}
