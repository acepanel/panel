package opensearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"go.yaml.in/yaml/v4"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{t: t}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

func (s *App) Status() string {
	ok, _ := systemctl.Status("opensearch")
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status("opensearch")
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
	conf, _ := io.Read(s.configPath())
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

	if err = systemctl.Restart("opensearch"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
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
		ClusterName:   s.getYAMLValue(cfg, "cluster.name"),
		NodeName:      s.getYAMLValue(cfg, "node.name"),
		NetworkHost:   s.getYAMLValue(cfg, "network.host"),
		HTTPPort:      s.getYAMLValue(cfg, "http.port"),
		DiscoveryType: s.getYAMLValue(cfg, "discovery.type"),
		PathData:      s.getYAMLValue(cfg, "path.data"),
		PathLogs:      s.getYAMLValue(cfg, "path.logs"),
		HeapInitSize:  heapInit,
		HeapMaxSize:   heapMax,
	}

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

	s.setYAMLValue(cfg, "cluster.name", req.ClusterName)
	s.setYAMLValue(cfg, "node.name", req.NodeName)
	s.setYAMLValue(cfg, "network.host", req.NetworkHost)
	s.setYAMLValue(cfg, "http.port", req.HTTPPort)
	s.setYAMLValue(cfg, "discovery.type", req.DiscoveryType)
	s.setYAMLValue(cfg, "path.data", req.PathData)
	s.setYAMLValue(cfg, "path.logs", req.PathLogs)

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

	if err = systemctl.Restart("opensearch"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) configPath() string {
	return fmt.Sprintf("%s/server/opensearch/config/opensearch.yml", app.Root)
}

func (s *App) jvmOptionsPath() string {
	return fmt.Sprintf("%s/server/opensearch/config/jvm.options", app.Root)
}

func (s *App) getPort() string {
	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	_ = yaml.Unmarshal([]byte(raw), &cfg)
	if cfg != nil {
		if v := s.getYAMLValue(cfg, "http.port"); v != "" {
			return v
		}
	}
	return "9200"
}

// getYAMLValue 获取 YAML 值，优先匹配平铺键（如 "path.data"），回退到嵌套键（如 path -> data）
func (s *App) getYAMLValue(cfg map[string]any, key string) string {
	// 优先匹配平铺键（安装脚本用 sed 生成的格式）
	if val, ok := cfg[key]; ok {
		return cast.ToString(val)
	}
	// 回退到嵌套键
	parts := strings.SplitN(key, ".", 2)
	val, ok := cfg[parts[0]]
	if !ok {
		return ""
	}
	if len(parts) == 1 {
		return cast.ToString(val)
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return ""
	}
	return s.getYAMLValue(nested, parts[1])
}

// setYAMLValue 设置 YAML 值
func (s *App) setYAMLValue(cfg map[string]any, key string, value string) {
	if value == "" {
		return
	}
	// 使用平铺键，同时清理可能存在的嵌套键
	cfg[key] = value
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 2 {
		if nested, ok := cfg[parts[0]].(map[string]any); ok {
			delete(nested, parts[1])
			if len(nested) == 0 {
				delete(cfg, parts[0])
			}
		}
	}
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
