package nginx

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/systemctl"
	webserverNginx "github.com/acepanel/panel/pkg/webserver/nginx"
	"github.com/go-chi/chi/v5"
)

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
	req, err := service.Bind[StreamServer](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", req.Name))
	if _, statErr := os.Stat(configPath); statErr == nil {
		service.Error(w, http.StatusConflict, s.t.Get("stream server config already exists: %s", req.Name))
		return
	}

	if err = s.saveStreamServerConfig(configPath, req); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream server config: %v", err))
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_ = os.Remove(configPath)
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// UpdateStreamServer 更新 Stream Server
func (s *App) UpdateStreamServer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	req, err := service.Bind[StreamServer](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", name))
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream server not found: %s", name))
		return
	}

	newConfigPath := configPath
	if req.Name != name {
		newConfigPath = filepath.Join(s.streamDir(), fmt.Sprintf("%s.conf", req.Name))
		if _, statErr := os.Stat(newConfigPath); statErr == nil {
			service.Error(w, http.StatusConflict, s.t.Get("stream server config already exists: %s", req.Name))
			return
		}
	}

	if err = s.saveStreamServerConfig(newConfigPath, req); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream server config: %v", err))
		return
	}

	if newConfigPath != configPath {
		_ = os.Remove(configPath)
	}

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
	req, err := service.Bind[StreamUpstream](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", req.Name))
	if _, statErr := os.Stat(configPath); statErr == nil {
		service.Error(w, http.StatusConflict, s.t.Get("stream upstream config already exists: %s", req.Name))
		return
	}

	if err = s.saveStreamUpstreamConfig(configPath, req); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream upstream config: %v", err))
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_ = os.Remove(configPath)
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// UpdateStreamUpstream 更新 Stream Upstream
func (s *App) UpdateStreamUpstream(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	req, err := service.Bind[StreamUpstream](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	configPath := filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", name))
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream upstream not found: %s", name))
		return
	}

	newConfigPath := configPath
	if req.Name != name {
		newConfigPath = filepath.Join(s.streamDir(), fmt.Sprintf("upstream_%s.conf", req.Name))
		if _, statErr := os.Stat(newConfigPath); statErr == nil {
			service.Error(w, http.StatusConflict, s.t.Get("stream upstream config already exists: %s", req.Name))
			return
		}
	}

	if err = s.saveStreamUpstreamConfig(newConfigPath, req); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream upstream config: %v", err))
		return
	}

	if newConfigPath != configPath {
		_ = os.Remove(configPath)
	}

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
		return nil, err
	}

	servers := make([]StreamServer, 0)
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

// parseStreamServerFile 解析单个 Stream Server 配置文件
func (s *App) parseStreamServerFile(filePath string, name string) (*StreamServer, error) {
	p, err := webserverNginx.NewParserFromFile(filePath)
	if err != nil {
		return nil, err
	}

	server := &StreamServer{
		Name: name,
	}

	// 解析 listen 指令
	listenDirs, err := p.Find("server.listen")
	if err == nil && len(listenDirs) > 0 {
		params := listenDirs[0].GetParameters()
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
	}
	// 解析 proxy_pass 指令
	proxyPassDir, err := p.FindOne("server.proxy_pass")
	if err == nil {
		params := proxyPassDir.GetParameters()
		if len(params) > 0 {
			server.ProxyPass = params[0].Value
		}
	}
	// 解析 proxy_protocol 指令
	proxyProtocolDir, err := p.FindOne("server.proxy_protocol")
	if err == nil {
		params := proxyProtocolDir.GetParameters()
		if len(params) > 0 && params[0].Value == "on" {
			server.ProxyProtocol = true
		}
	}
	// 解析 proxy_timeout 指令
	proxyTimeoutDir, err := p.FindOne("server.proxy_timeout")
	if err == nil {
		params := proxyTimeoutDir.GetParameters()
		if len(params) > 0 {
			server.ProxyTimeout = parseNginxDuration(params[0].Value)
		}
	}
	// 解析 proxy_connect_timeout 指令
	proxyConnectTimeoutDir, err := p.FindOne("server.proxy_connect_timeout")
	if err == nil {
		params := proxyConnectTimeoutDir.GetParameters()
		if len(params) > 0 {
			server.ProxyConnectTimeout = parseNginxDuration(params[0].Value)
		}
	}
	// 解析 ssl_certificate 指令
	sslCertDir, err := p.FindOne("server.ssl_certificate")
	if err == nil {
		params := sslCertDir.GetParameters()
		if len(params) > 0 {
			server.SSLCertificate = params[0].Value
		}
	}
	// 解析 ssl_certificate_key 指令
	sslKeyDir, err := p.FindOne("server.ssl_certificate_key")
	if err == nil {
		params := sslKeyDir.GetParameters()
		if len(params) > 0 {
			server.SSLCertificateKey = params[0].Value
		}
	}

	return server, nil
}

// parseStreamUpstreams 解析所有 Stream Upstream 配置
func (s *App) parseStreamUpstreams() ([]StreamUpstream, error) {
	entries, err := os.ReadDir(s.streamDir())
	if err != nil {
		return nil, err
	}

	upstreams := make([]StreamUpstream, 0)
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

// parseStreamUpstreamFile 解析单个 Stream Upstream 配置文件
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
			dirParams := dir.GetParameters()
			if len(dirParams) > 0 {
				addr := dirParams[0].Value
				var options []string
				for i := 1; i < len(dirParams); i++ {
					options = append(options, dirParams[i].Value)
				}
				upstream.Servers[addr] = strings.Join(options, " ")
			}
		case "least_conn", "ip_hash", "random":
			upstream.Algo = dir.GetName()
		case "hash":
			dirParams := dir.GetParameters()
			if len(dirParams) > 0 {
				upstream.Algo = "hash " + dirParams[0].Value
				// 检查是否有 consistent 参数
				if len(dirParams) > 1 && dirParams[1].Value == "consistent" {
					upstream.Algo += " consistent"
				}
			}
		case "least_time":
			dirParams := dir.GetParameters()
			if len(dirParams) > 0 {
				upstream.Algo = "least_time " + dirParams[0].Value
			}
		}
	}

	return upstream, nil
}

// saveStreamServerConfig 生成并保存 Stream Server 配置
func (s *App) saveStreamServerConfig(filePath string, server *StreamServer) error {
	p, err := webserverNginx.NewParserFromString("server {}")
	if err != nil {
		return err
	}
	p.SetConfigPath(filePath)

	// listen 指令
	listenParams := []string{server.Listen}
	if server.UDP {
		listenParams = append(listenParams, "udp")
	}
	if server.SSL {
		listenParams = append(listenParams, "ssl")
	}
	if err = p.SetOne("server.listen", listenParams); err != nil {
		return err
	}
	// proxy_pass 指令
	if err = p.SetOne("server.proxy_pass", []string{server.ProxyPass}); err != nil {
		return err
	}
	// proxy_protocol 指令
	if server.ProxyProtocol {
		if err = p.SetOne("server.proxy_protocol", []string{"on"}); err != nil {
			return err
		}
	}
	// proxy_timeout 指令
	if server.ProxyTimeout > 0 {
		if err = p.SetOne("server.proxy_timeout", []string{formatNginxDuration(server.ProxyTimeout)}); err != nil {
			return err
		}
	}
	// proxy_connect_timeout 指令
	if server.ProxyConnectTimeout > 0 {
		if err = p.SetOne("server.proxy_connect_timeout", []string{formatNginxDuration(server.ProxyConnectTimeout)}); err != nil {
			return err
		}
	}
	// SSL 配置
	if server.SSL {
		if server.SSLCertificate != "" {
			if err = p.SetOne("server.ssl_certificate", []string{server.SSLCertificate}); err != nil {
				return err
			}
		}
		if server.SSLCertificateKey != "" {
			if err = p.SetOne("server.ssl_certificate_key", []string{server.SSLCertificateKey}); err != nil {
				return err
			}
		}
	}

	return os.WriteFile(filePath, []byte(p.Dump()), 0600)
}

// saveStreamUpstreamConfig 生成并保存 Stream Upstream 配置
func (s *App) saveStreamUpstreamConfig(filePath string, upstream *StreamUpstream) error {
	// 创建空配置的 parser
	p, err := webserverNginx.NewParserFromString(fmt.Sprintf("upstream %s {}", upstream.Name))
	if err != nil {
		return err
	}
	p.SetConfigPath(filePath)

	// 负载均衡算法
	if upstream.Algo != "" {
		algoParts := strings.Fields(upstream.Algo)
		if len(algoParts) > 0 {
			algoParams := algoParts[1:]
			if err = p.SetOne("upstream."+algoParts[0], algoParams); err != nil {
				return err
			}
		}
	}

	// 服务器列表
	var addrs []string
	for addr := range upstream.Servers {
		addrs = append(addrs, addr)
	}
	sort.Strings(addrs)

	for _, addr := range addrs {
		options := upstream.Servers[addr]
		params := []string{addr}
		if options != "" {
			params = append(params, strings.Fields(options)...)
		}
		if err = p.SetOne("upstream.server", params); err != nil {
			return err
		}
	}

	return os.WriteFile(filePath, []byte(p.Dump()), 0600)
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

// streamDir 返回 stream 配置目录
func (s *App) streamDir() string {
	return filepath.Join(app.Root, "server/nginx/conf/stream")
}
