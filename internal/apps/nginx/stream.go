package nginx

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/webserver/conf"
	webserverNginx "github.com/acepanel/panel/v3/pkg/webserver/nginx"
)

// ListStreamServers 获取 Stream Server 列表
func (s *App) ListStreamServers(w http.ResponseWriter, r *http.Request) {
	servers, err := s.parseStreamServers()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to list stream servers: %v", err))
		return
	}

	s.log.Info(s.t.Get("nginx stream server list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.streamDir()))

	service.Success(w, servers)
}

// CreateStreamServer 创建 Stream Server
func (s *App) CreateStreamServer(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[StreamServer](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	configPath := filepath.Join(s.streamDir(), req.Name+".conf")
	if _, statErr := os.Stat(configPath); statErr == nil {
		service.Error(w, http.StatusConflict, s.t.Get("stream server config already exists: %s", req.Name))
		return
	}

	if err = s.saveStreamServerConfig(configPath, req); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to write stream server config: %v", err))
		return
	}

	if err = s.reload(r.Context(), configPath); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("nginx stream server created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", req.Name), slog.String("path", configPath), slog.String("listen", req.Listen), slog.String("proxy_pass", req.ProxyPass), slog.Bool("udp", req.UDP), slog.Bool("ssl", req.SSL))

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

	configPath := filepath.Join(s.streamDir(), name+".conf")
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream server not found: %s", name))
		return
	}

	newConfigPath := configPath
	if req.Name != name {
		newConfigPath = filepath.Join(s.streamDir(), req.Name+".conf")
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

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("nginx stream server updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", req.Name), slog.String("old_name", name), slog.String("path", newConfigPath), slog.String("listen", req.Listen), slog.String("proxy_pass", req.ProxyPass), slog.Bool("udp", req.UDP), slog.Bool("ssl", req.SSL))

	service.Success(w, nil)
}

// DeleteStreamServer 删除 Stream Server
func (s *App) DeleteStreamServer(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	configPath := filepath.Join(s.streamDir(), name+".conf")
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		service.Error(w, http.StatusNotFound, s.t.Get("stream server not found: %s", name))
		return
	}

	if err := os.Remove(configPath); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to delete stream server config: %v", err))
		return
	}

	if err := s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("nginx stream server deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", name), slog.String("path", configPath))

	service.Success(w, nil)
}

// ListStreamUpstreams 获取 Stream Upstream 列表
func (s *App) ListStreamUpstreams(w http.ResponseWriter, r *http.Request) {
	upstreams, err := s.parseStreamUpstreams()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to list stream upstreams: %v", err))
		return
	}

	s.log.Info(s.t.Get("nginx stream upstream list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.streamDir()))

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

	if err = s.reload(r.Context(), configPath); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("nginx stream upstream created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", req.Name), slog.String("path", configPath), slog.String("algo", req.Algo), slog.Any("servers", lo.Keys(req.Servers)))

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

	if err = s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("nginx stream upstream updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", req.Name), slog.String("old_name", name), slog.String("path", newConfigPath), slog.String("algo", req.Algo), slog.Any("servers", lo.Keys(req.Servers)))

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

	if err := s.reload(r.Context()); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("nginx stream upstream deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", name), slog.String("path", configPath))

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
	cfg, err := webserverNginx.ParseFile(filePath)
	if err != nil {
		return nil, err
	}
	srv := cfg.GetBlock("server")
	if srv == nil {
		return nil, errors.New("no server block found")
	}

	server := &StreamServer{
		Name:                name,
		ProxyPass:           srv.Value("proxy_pass"),
		ProxyProtocol:       srv.Value("proxy_protocol") == "on",
		ProxyTimeout:        parseNginxDuration(srv.Value("proxy_timeout")),
		ProxyConnectTimeout: parseNginxDuration(srv.Value("proxy_connect_timeout")),
		SSLCertificate:      srv.Value("ssl_certificate"),
		SSLCertificateKey:   srv.Value("ssl_certificate_key"),
	}
	if listen := srv.Get("listen"); listen != nil {
		server.Listen = listen.Arg(0)
		for _, arg := range listen.ArgsFrom(1) {
			switch arg {
			case "udp":
				server.UDP = true
			case "ssl":
				server.SSL = true
			}
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
	cfg, err := webserverNginx.ParseFile(filePath)
	if err != nil {
		return nil, err
	}
	up := cfg.GetBlock("upstream")
	if up == nil {
		return nil, errors.New("no upstream block found")
	}
	name := up.Arg(0)
	if name == "" {
		return nil, errors.New("upstream name not found")
	}
	if expectedName != "" && name != expectedName {
		return nil, errors.New("upstream name mismatch")
	}

	upstream := &StreamUpstream{
		Name:     name,
		Servers:  make(map[string]string),
		Resolver: []string{},
	}
	for _, d := range up.All() {
		switch d.Name {
		case "server":
			if d.Arg(0) != "" {
				upstream.Servers[d.Arg(0)] = strings.Join(d.ArgsFrom(1), " ")
			}
		case "least_conn", "ip_hash", "random":
			upstream.Algo = d.Name
		case "hash":
			if d.Arg(0) != "" {
				upstream.Algo = "hash " + d.Arg(0)
				if d.Arg(1) == "consistent" {
					upstream.Algo += " consistent"
				}
			}
		case "least_time":
			if d.Arg(0) != "" {
				upstream.Algo = "least_time " + d.Arg(0)
			}
		case "resolver":
			upstream.Resolver = append(upstream.Resolver, d.Values()...)
		case "resolver_timeout":
			upstream.ResolverTimeout = parseNginxDuration(d.Arg(0))
		}
	}

	return upstream, nil
}

// saveStreamServerConfig 生成并保存 Stream Server 配置
func (s *App) saveStreamServerConfig(filePath string, server *StreamServer) error {
	cfg := &conf.Config{}
	srv := cfg.AddBlock("server")

	listen := []string{server.Listen}
	if server.UDP {
		listen = append(listen, "udp")
	}
	if server.SSL {
		listen = append(listen, "ssl")
	}
	srv.Add("listen", listen...)
	srv.Add("proxy_pass", server.ProxyPass)
	if server.ProxyProtocol {
		srv.Add("proxy_protocol", "on")
	}
	if server.ProxyTimeout > 0 {
		srv.Add("proxy_timeout", formatNginxDuration(server.ProxyTimeout))
	}
	if server.ProxyConnectTimeout > 0 {
		srv.Add("proxy_connect_timeout", formatNginxDuration(server.ProxyConnectTimeout))
	}
	if server.SSL {
		if server.SSLCertificate != "" {
			srv.Add("ssl_certificate", server.SSLCertificate)
		}
		if server.SSLCertificateKey != "" {
			srv.Add("ssl_certificate_key", server.SSLCertificateKey)
		}
	}

	return os.WriteFile(filePath, []byte(webserverNginx.Export(cfg)), 0600)
}

// saveStreamUpstreamConfig 生成并保存 Stream Upstream 配置
func (s *App) saveStreamUpstreamConfig(filePath string, upstream *StreamUpstream) error {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "upstream %s {\n", upstream.Name)

	// 负载均衡算法
	if upstream.Algo != "" {
		_, _ = fmt.Fprintf(&sb, "    %s;\n", upstream.Algo)
	}

	// resolver 配置
	if len(upstream.Resolver) > 0 {
		_, _ = fmt.Fprintf(&sb, "    resolver %s;\n", strings.Join(upstream.Resolver, " "))
		if upstream.ResolverTimeout > 0 {
			_, _ = fmt.Fprintf(&sb, "    resolver_timeout %s;\n", formatNginxDuration(upstream.ResolverTimeout))
		}
	}

	// 服务器列表
	addrs := lo.Keys(upstream.Servers)
	sort.Strings(addrs)

	for _, addr := range addrs {
		options := upstream.Servers[addr]
		if options != "" {
			_, _ = fmt.Fprintf(&sb, "    server %s %s;\n", addr, options)
		} else {
			_, _ = fmt.Fprintf(&sb, "    server %s;\n", addr)
		}
	}

	sb.WriteString("}\n")

	return os.WriteFile(filePath, []byte(sb.String()), 0600)
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
	if len(value) == 1 {
		value += "s" // 单个字符，默认为秒
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
