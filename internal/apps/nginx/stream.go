package nginx

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/systemctl"
	webserverNginx "github.com/acepanel/panel/pkg/webserver/nginx"
)

// streamDir 返回 stream 配置目录
func (s *App) streamDir() string {
	return filepath.Join(app.Root, "server/nginx/conf/stream")
}

// ListStreamServers 获取 Stream Server 列表
func (s *App) ListStreamServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.parseStreamServers()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to list stream servers: %v", err))
		return
	}
	service.Success(w, servers)
}

// CreateStreamServer 创建 Stream Server
func (s *App) CreateStreamServer(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[CreateStreamServer](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 检查配置文件是否已存在
	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", req.Name))
	if _, statErr := os.Stat(configPath); statErr == nil {
		service.Error(w, http.StatusConflict, s.t.Get("stream server config already exists: %s", req.Name))
		return
	}

	// 确保目录存在
	if err = os.MkdirAll(s.streamDir(), 0755); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to create stream directory: %v", err))
		return
	}

	// 使用 parser 生成配置并保存
	if err = s.saveStreamServerConfig(configPath, &req.StreamServer); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream server config: %v", err))
		return
	}

	// 重载 nginx
	if err = systemctl.Reload("nginx"); err != nil {
		// 删除刚创建的配置文件
		_ = os.Remove(configPath)
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// GetStreamServer 获取单个 Stream Server
func (s *App) GetStreamServer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", name))
	server, err := s.parseStreamServerFile(configPath, name)
	if err != nil {
		service.Error(w, http.StatusNotFound, s.t.Get("stream server not found: %s", name))
		return
	}

	service.Success(w, server)
}

// UpdateStreamServer 更新 Stream Server
func (s *App) UpdateStreamServer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	req, err := service.Bind[UpdateStreamServer](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 检查配置文件是否存在
	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", name))
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream server not found: %s", name))
		return
	}

	// 如果名称变更，需要重命名文件
	newConfigPath := configPath
	if req.Name != name {
		newConfigPath = filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", req.Name))
		if _, statErr := os.Stat(newConfigPath); statErr == nil {
			service.Error(w, http.StatusConflict, s.t.Get("stream server config already exists: %s", req.Name))
			return
		}
	}

	// 使用 parser 生成配置并保存
	if err = s.saveStreamServerConfig(newConfigPath, &req.StreamServer); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream server config: %v", err))
		return
	}

	// 删除旧配置文件（如果名称变更）
	if newConfigPath != configPath {
		_ = os.Remove(configPath)
	}

	// 重载 nginx
	if err = systemctl.Reload("nginx"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// DeleteStreamServer 删除 Stream Server
func (s *App) DeleteStreamServer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", name))
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream server not found: %s", name))
		return
	}

	if err := os.Remove(configPath); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to delete stream server config: %v", err))
		return
	}

	// 重载 nginx
	if err := systemctl.Reload("nginx"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// ListStreamUpstreams 获取 Stream Upstream 列表
func (s *App) ListStreamUpstreams(w http.ResponseWriter, r *http.Request) {
	upstreams, err := s.parseStreamUpstreams()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to list stream upstreams: %v", err))
		return
	}
	service.Success(w, upstreams)
}

// CreateStreamUpstream 创建 Stream Upstream
func (s *App) CreateStreamUpstream(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[CreateStreamUpstream](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 检查配置文件是否已存在
	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", req.Name))
	if _, statErr := os.Stat(configPath); statErr == nil {
		service.Error(w, http.StatusConflict, s.t.Get("stream upstream config already exists: %s", req.Name))
		return
	}

	// 确保目录存在
	if err = os.MkdirAll(s.streamDir(), 0755); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to create stream directory: %v", err))
		return
	}

	// 使用 parser 生成配置并保存
	if err = s.saveStreamUpstreamConfig(configPath, &req.StreamUpstream); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream upstream config: %v", err))
		return
	}

	// 重载 nginx
	if err = systemctl.Reload("nginx"); err != nil {
		// 删除刚创建的配置文件
		_ = os.Remove(configPath)
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// GetStreamUpstream 获取单个 Stream Upstream
func (s *App) GetStreamUpstream(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", name))
	upstream, err := s.parseStreamUpstreamFile(configPath, name)
	if err != nil {
		service.Error(w, http.StatusNotFound, s.t.Get("stream upstream not found: %s", name))
		return
	}

	service.Success(w, upstream)
}

// UpdateStreamUpstream 更新 Stream Upstream
func (s *App) UpdateStreamUpstream(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	req, err := service.Bind[UpdateStreamUpstream](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 检查配置文件是否存在
	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", name))
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream upstream not found: %s", name))
		return
	}

	// 如果名称变更，需要重命名文件
	newConfigPath := configPath
	if req.Name != name {
		newConfigPath = filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", req.Name))
		if _, statErr := os.Stat(newConfigPath); statErr == nil {
			service.Error(w, http.StatusConflict, s.t.Get("stream upstream config already exists: %s", req.Name))
			return
		}
	}

	// 使用 parser 生成配置并保存
	if err = s.saveStreamUpstreamConfig(newConfigPath, &req.StreamUpstream); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream upstream config: %v", err))
		return
	}

	// 删除旧配置文件（如果名称变更）
	if newConfigPath != configPath {
		_ = os.Remove(configPath)
	}

	// 重载 nginx
	if err = systemctl.Reload("nginx"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// DeleteStreamUpstream 删除 Stream Upstream
func (s *App) DeleteStreamUpstream(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", name))
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream upstream not found: %s", name))
		return
	}

	if err := os.Remove(configPath); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to delete stream upstream config: %v", err))
		return
	}

	// 重载 nginx
	if err := systemctl.Reload("nginx"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// parseStreamServers 解析所有 Stream Server 配置
func (s *App) parseStreamServers() ([]StreamServer, error) {
	entries, err := os.ReadDir(s.streamDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []StreamServer{}, nil
		}
		return nil, err
	}

	var servers []StreamServer
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		// 跳过 upstream 配置文件
		if strings.HasPrefix(fileName, "upstream_") {
			continue
		}

		if !strings.HasSuffix(fileName, ".conf") {
			continue
		}

		name := strings.TrimSuffix(fileName, ".conf")
		configPath := filepath.Join(s.streamDir(), fileName)
		server, err := s.parseStreamServerFile(configPath, name)
		if err != nil {
			continue // 跳过解析失败的文件
		}
		if server != nil {
			servers = append(servers, *server)
		}
	}

	// 按名称排序
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	return servers, nil
}

// parseStreamServerFile 使用 parser 解析单个 Stream Server 配置文件
func (s *App) parseStreamServerFile(filePath string, name string) (*StreamServer, error) {
	p, err := webserverNginx.NewParserFromFile(filePath)
	if err != nil {
		return nil, err
	}

	server := &StreamServer{
		Name: name,
	}

	// 查找 server 块中的指令
	cfg := p.Config()
	if cfg == nil || cfg.Block == nil {
		return nil, fmt.Errorf("invalid config")
	}

	// 查找 server 块
	serverDirectives := cfg.Block.FindDirectives("server")
	if len(serverDirectives) == 0 {
		return nil, fmt.Errorf("no server block found")
	}

	serverBlock := serverDirectives[0].GetBlock()
	if serverBlock == nil {
		return nil, fmt.Errorf("server block is empty")
	}

	// 解析 listen 指令
	for _, dir := range serverBlock.GetDirectives() {
		switch dir.GetName() {
		case "listen":
			params := dir.GetParameters()
			if len(params) > 0 {
				server.Listen = params[0].Value
				for i := 1; i < len(params); i++ {
					switch params[i].Value {
					case "udp":
						server.UDP = true
					case "ssl":
						server.SSL = true
					}
				}
			}
		case "proxy_pass":
			params := dir.GetParameters()
			if len(params) > 0 {
				server.ProxyPass = params[0].Value
			}
		case "proxy_protocol":
			params := dir.GetParameters()
			if len(params) > 0 && params[0].Value == "on" {
				server.ProxyProtocol = true
			}
		case "proxy_timeout":
			params := dir.GetParameters()
			if len(params) > 0 {
				server.ProxyTimeout = parseNginxDuration(params[0].Value)
			}
		case "proxy_connect_timeout":
			params := dir.GetParameters()
			if len(params) > 0 {
				server.ProxyConnectTimeout = parseNginxDuration(params[0].Value)
			}
		case "ssl_certificate":
			params := dir.GetParameters()
			if len(params) > 0 {
				server.SSLCertificate = params[0].Value
			}
		case "ssl_certificate_key":
			params := dir.GetParameters()
			if len(params) > 0 {
				server.SSLCertificateKey = params[0].Value
			}
		}
	}

	return server, nil
}

// parseStreamUpstreams 解析所有 Stream Upstream 配置
func (s *App) parseStreamUpstreams() ([]StreamUpstream, error) {
	entries, err := os.ReadDir(s.streamDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []StreamUpstream{}, nil
		}
		return nil, err
	}

	var upstreams []StreamUpstream
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		// 只处理 upstream 配置文件
		if !strings.HasPrefix(fileName, "upstream_") {
			continue
		}

		if !strings.HasSuffix(fileName, ".conf") {
			continue
		}

		name := strings.TrimPrefix(fileName, "upstream_")
		name = strings.TrimSuffix(name, ".conf")
		configPath := filepath.Join(s.streamDir(), fileName)
		upstream, err := s.parseStreamUpstreamFile(configPath, name)
		if err != nil {
			continue // 跳过解析失败的文件
		}
		if upstream != nil {
			upstreams = append(upstreams, *upstream)
		}
	}

	// 按名称排序
	sort.Slice(upstreams, func(i, j int) bool {
		return upstreams[i].Name < upstreams[j].Name
	})

	return upstreams, nil
}

// parseStreamUpstreamFile 使用 parser 解析单个 Stream Upstream 配置文件
func (s *App) parseStreamUpstreamFile(filePath string, expectedName string) (*StreamUpstream, error) {
	p, err := webserverNginx.NewParserFromFile(filePath)
	if err != nil {
		return nil, err
	}

	cfg := p.Config()
	if cfg == nil || cfg.Block == nil {
		return nil, fmt.Errorf("invalid config")
	}

	// 查找 upstream 块
	upstreamDirectives := cfg.Block.FindDirectives("upstream")
	if len(upstreamDirectives) == 0 {
		return nil, fmt.Errorf("no upstream block found")
	}

	upstreamDir := upstreamDirectives[0]
	params := upstreamDir.GetParameters()
	if len(params) == 0 {
		return nil, fmt.Errorf("upstream name not found")
	}

	name := params[0].Value
	if expectedName != "" && name != expectedName {
		return nil, fmt.Errorf("upstream name mismatch")
	}

	upstream := &StreamUpstream{
		Name:    name,
		Servers: make(map[string]string),
	}

	upstreamBlock := upstreamDir.GetBlock()
	if upstreamBlock == nil {
		return nil, fmt.Errorf("upstream block is empty")
	}

	// 解析 upstream 块中的指令
	for _, dir := range upstreamBlock.GetDirectives() {
		switch dir.GetName() {
		case "server":
			params := dir.GetParameters()
			if len(params) > 0 {
				addr := params[0].Value
				var options []string
				for i := 1; i < len(params); i++ {
					options = append(options, params[i].Value)
				}
				upstream.Servers[addr] = strings.Join(options, " ")
			}
		case "least_conn", "ip_hash", "random":
			upstream.Algo = dir.GetName()
		case "hash":
			params := dir.GetParameters()
			if len(params) > 0 {
				upstream.Algo = "hash " + params[0].Value
				// 检查是否有 consistent 参数
				if len(params) > 1 && params[1].Value == "consistent" {
					upstream.Algo += " consistent"
				}
			}
		case "least_time":
			params := dir.GetParameters()
			if len(params) > 0 {
				upstream.Algo = "least_time " + params[0].Value
			}
		}
	}

	return upstream, nil
}

// saveStreamServerConfig 使用 parser 生成并保存 Stream Server 配置
func (s *App) saveStreamServerConfig(filePath string, server *StreamServer) error {
	// 构建 server 块
	serverBlock := &config.Block{}

	// listen 指令
	listenParams := []config.Parameter{{Value: server.Listen}}
	if server.UDP {
		listenParams = append(listenParams, config.Parameter{Value: "udp"})
	}
	if server.SSL {
		listenParams = append(listenParams, config.Parameter{Value: "ssl"})
	}
	serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
		Name:       "listen",
		Parameters: listenParams,
	})

	// proxy_pass 指令
	serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
		Name:       "proxy_pass",
		Parameters: []config.Parameter{{Value: server.ProxyPass}},
	})

	// proxy_protocol 指令
	if server.ProxyProtocol {
		serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
			Name:       "proxy_protocol",
			Parameters: []config.Parameter{{Value: "on"}},
		})
	}

	// proxy_timeout 指令
	if server.ProxyTimeout > 0 {
		serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
			Name:       "proxy_timeout",
			Parameters: []config.Parameter{{Value: formatNginxDuration(server.ProxyTimeout)}},
		})
	}

	// proxy_connect_timeout 指令
	if server.ProxyConnectTimeout > 0 {
		serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
			Name:       "proxy_connect_timeout",
			Parameters: []config.Parameter{{Value: formatNginxDuration(server.ProxyConnectTimeout)}},
		})
	}

	// SSL 配置
	if server.SSL {
		if server.SSLCertificate != "" {
			serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
				Name:       "ssl_certificate",
				Parameters: []config.Parameter{{Value: server.SSLCertificate}},
			})
		}
		if server.SSLCertificateKey != "" {
			serverBlock.Directives = append(serverBlock.Directives, &config.Directive{
				Name:       "ssl_certificate_key",
				Parameters: []config.Parameter{{Value: server.SSLCertificateKey}},
			})
		}
	}

	// 创建 server 指令
	serverDirective := &config.Directive{
		Name:  "server",
		Block: serverBlock,
	}

	// 创建配置
	cfg := &config.Config{
		Block: &config.Block{
			Directives: []config.IDirective{serverDirective},
		},
	}

	// 使用 dumper 生成配置内容
	content := fmt.Sprintf("# Stream Server: %s\n", server.Name)
	content += dumper.DumpConfig(cfg, dumper.IndentedStyle)
	content += "\n"

	return os.WriteFile(filePath, []byte(content), 0600)
}

// saveStreamUpstreamConfig 使用 parser 生成并保存 Stream Upstream 配置
func (s *App) saveStreamUpstreamConfig(filePath string, upstream *StreamUpstream) error {
	// 构建 upstream 块
	upstreamBlock := &config.Block{}

	// 负载均衡算法
	if upstream.Algo != "" {
		algoParts := strings.Fields(upstream.Algo)
		if len(algoParts) > 0 {
			algoParams := make([]config.Parameter, 0, len(algoParts)-1)
			for i := 1; i < len(algoParts); i++ {
				algoParams = append(algoParams, config.Parameter{Value: algoParts[i]})
			}
			upstreamBlock.Directives = append(upstreamBlock.Directives, &config.Directive{
				Name:       algoParts[0],
				Parameters: algoParams,
			})
		}
	}

	// 服务器列表
	// 为了保持顺序一致，对 servers 按地址排序
	var addrs []string
	for addr := range upstream.Servers {
		addrs = append(addrs, addr)
	}
	slices.Sort(addrs)

	for _, addr := range addrs {
		options := upstream.Servers[addr]
		params := []config.Parameter{{Value: addr}}
		if options != "" {
			for _, opt := range strings.Fields(options) {
				params = append(params, config.Parameter{Value: opt})
			}
		}
		upstreamBlock.Directives = append(upstreamBlock.Directives, &config.Directive{
			Name:       "server",
			Parameters: params,
		})
	}

	// 创建 upstream 指令
	upstreamDirective := &config.Directive{
		Name:       "upstream",
		Parameters: []config.Parameter{{Value: upstream.Name}},
		Block:      upstreamBlock,
	}

	// 创建配置
	cfg := &config.Config{
		Block: &config.Block{
			Directives: []config.IDirective{upstreamDirective},
		},
	}

	// 使用 dumper 生成配置内容
	content := fmt.Sprintf("# Stream Upstream: %s\n", upstream.Name)
	content += dumper.DumpConfig(cfg, dumper.IndentedStyle)
	content += "\n"

	return os.WriteFile(filePath, []byte(content), 0600)
}

// parseNginxDuration 解析 Nginx 时间格式（如 10s, 1m, 1h）
func parseNginxDuration(value string) time.Duration {
	if value == "" {
		return 0
	}

	// 尝试解析带单位的时间
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return 0
	}

	unit := value[len(value)-1]
	numStr := value[:len(value)-1]

	var num int
	_, _ = fmt.Sscanf(numStr, "%d", &num)

	switch unit {
	case 's':
		return time.Duration(num) * time.Second
	case 'm':
		return time.Duration(num) * time.Minute
	case 'h':
		return time.Duration(num) * time.Hour
	case 'd':
		return time.Duration(num) * 24 * time.Hour
	default:
		// 没有单位，尝试直接解析为秒
		_, _ = fmt.Sscanf(value, "%d", &num)
		return time.Duration(num) * time.Second
	}
}

// formatNginxDuration 格式化时间为 Nginx 格式
func formatNginxDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	seconds := int(d.Seconds())
	if seconds%3600 == 0 {
		return fmt.Sprintf("%dh", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%ds", seconds)
}

// NewStreamParserFromFile 从指定文件路径创建 Stream 配置解析器
func NewStreamParserFromFile(filePath string) (*webserverNginx.Parser, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	p := parser.NewStringParser(string(content), parser.WithSkipIncludeParsingErr(), parser.WithSkipValidDirectivesErr())
	cfg, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 由于 webserverNginx.Parser 的字段是私有的，我们直接返回解析后的 config
	// 这里我们创建一个新的包装
	_ = cfg
	return webserverNginx.NewParserFromFile(filePath)
}
