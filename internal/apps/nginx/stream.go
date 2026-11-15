package nginx

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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

// upstreamPrefix Upstream 配置文件名的前缀，同目录下其余 .conf 都是 Server
const upstreamPrefix = "upstream_"

var (
	nginxDurationRegexp = regexp.MustCompile(`(\d+)\s*(ms|[smhdwMy]?)`)
	nginxDurationUnits  = map[string]time.Duration{
		"":   time.Second,
		"ms": time.Millisecond,
		"s":  time.Second,
		"m":  time.Minute,
		"h":  time.Hour,
		"d":  24 * time.Hour,
		"w":  7 * 24 * time.Hour,
		"M":  30 * 24 * time.Hour,
		"y":  365 * 24 * time.Hour,
	}
)

// streamKind 描述一类 stream 配置，Server 与 Upstream 的增删改查流程相同，只有文件名、读写方式与文案不同
type streamKind[T any] struct {
	prefix string
	name   func(*T) string
	// render 生成配置文件内容
	render func(*T) string
	parse  func(path, name string) (*T, error)
	// attrs 创建与更新日志里的业务字段
	attrs func(*T) []any

	// 以下文案都是未带参数的翻译模板
	listFailed, listViewed    string
	exists, notFound          string
	writeFailed, deleteFailed string
	created, updated, deleted string
}

func (s *App) streamServer() streamKind[StreamServer] {
	return streamKind[StreamServer]{
		name:   func(server *StreamServer) string { return server.Name },
		render: renderStreamServer,
		parse:  parseStreamServerFile,
		attrs: func(server *StreamServer) []any {
			return []any{slog.String("listen", server.Listen), slog.String("proxy_pass", server.ProxyPass), slog.Bool("udp", server.UDP), slog.Bool("ssl", server.SSL)}
		},
		listFailed:   s.t.Get("failed to list stream servers: %v"),
		listViewed:   s.t.Get("nginx stream server list viewed"),
		exists:       s.t.Get("stream server config already exists: %s"),
		notFound:     s.t.Get("stream server not found: %s"),
		writeFailed:  s.t.Get("failed to write stream server config: %v"),
		deleteFailed: s.t.Get("failed to delete stream server config: %v"),
		created:      s.t.Get("nginx stream server created"),
		updated:      s.t.Get("nginx stream server updated"),
		deleted:      s.t.Get("nginx stream server deleted"),
	}
}

func (s *App) streamUpstream() streamKind[StreamUpstream] {
	return streamKind[StreamUpstream]{
		prefix: upstreamPrefix,
		name:   func(upstream *StreamUpstream) string { return upstream.Name },
		render: renderStreamUpstream,
		parse:  parseStreamUpstreamFile,
		attrs: func(upstream *StreamUpstream) []any {
			return []any{slog.String("algo", upstream.Algo), slog.Any("servers", lo.Keys(upstream.Servers))}
		},
		listFailed:   s.t.Get("failed to list stream upstreams: %v"),
		listViewed:   s.t.Get("nginx stream upstream list viewed"),
		exists:       s.t.Get("stream upstream config already exists: %s"),
		notFound:     s.t.Get("stream upstream not found: %s"),
		writeFailed:  s.t.Get("failed to write stream upstream config: %v"),
		deleteFailed: s.t.Get("failed to delete stream upstream config: %v"),
		created:      s.t.Get("nginx stream upstream created"),
		updated:      s.t.Get("nginx stream upstream updated"),
		deleted:      s.t.Get("nginx stream upstream deleted"),
	}
}

// ListStreamServers 获取 Stream Server 列表
func (s *App) ListStreamServers(w http.ResponseWriter, r *http.Request) {
	listStream(s, s.streamServer(), w, r)
}

// CreateStreamServer 创建 Stream Server
func (s *App) CreateStreamServer(w http.ResponseWriter, r *http.Request) {
	createStream(s, s.streamServer(), w, r)
}

// UpdateStreamServer 更新 Stream Server
func (s *App) UpdateStreamServer(w http.ResponseWriter, r *http.Request) {
	updateStream(s, s.streamServer(), w, r)
}

// DeleteStreamServer 删除 Stream Server
func (s *App) DeleteStreamServer(w http.ResponseWriter, r *http.Request) {
	deleteStream(s, s.streamServer(), w, r)
}

// ListStreamUpstreams 获取 Stream Upstream 列表
func (s *App) ListStreamUpstreams(w http.ResponseWriter, r *http.Request) {
	listStream(s, s.streamUpstream(), w, r)
}

// CreateStreamUpstream 创建 Stream Upstream
func (s *App) CreateStreamUpstream(w http.ResponseWriter, r *http.Request) {
	createStream(s, s.streamUpstream(), w, r)
}

// UpdateStreamUpstream 更新 Stream Upstream
func (s *App) UpdateStreamUpstream(w http.ResponseWriter, r *http.Request) {
	updateStream(s, s.streamUpstream(), w, r)
}

// DeleteStreamUpstream 删除 Stream Upstream
func (s *App) DeleteStreamUpstream(w http.ResponseWriter, r *http.Request) {
	deleteStream(s, s.streamUpstream(), w, r)
}

// path 返回该类配置下名为 name 的配置文件路径
func (k streamKind[T]) path(name string) string {
	return filepath.Join(streamDir(), k.prefix+name+".conf")
}

// list 解析目录下属于该类的全部配置，按名称排序，解析失败的文件跳过
func (k streamKind[T]) list() ([]T, error) {
	entries, err := os.ReadDir(streamDir())
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		fileName := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(fileName, ".conf") || strings.HasPrefix(fileName, upstreamPrefix) != (k.prefix == upstreamPrefix) {
			continue
		}
		names = append(names, strings.TrimSuffix(strings.TrimPrefix(fileName, k.prefix), ".conf"))
	}
	slices.Sort(names)

	items := make([]T, 0, len(names))
	for _, name := range names {
		if item, err := k.parse(k.path(name), name); err == nil {
			items = append(items, *item)
		}
	}

	return items, nil
}

func listStream[T any](s *App, k streamKind[T], w http.ResponseWriter, r *http.Request) {
	items, err := k.list()
	if err != nil {
		service.Error(w, http.StatusInternalServerError, k.listFailed, err)
		return
	}

	s.log.Info(k.listViewed, slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", streamDir()))

	service.Success(w, items)
}

func createStream[T any](s *App, k streamKind[T], w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[T](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := k.name(req)
	path := k.path(name)
	if _, statErr := os.Stat(path); statErr == nil {
		service.Error(w, http.StatusConflict, k.exists, name)
		return
	}

	if err = os.WriteFile(path, []byte(k.render(req)), 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, k.writeFailed, err)
		return
	}

	if err = s.Reload(r.Context()); err != nil {
		_ = os.Remove(path)
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	attrs := []any{slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", name), slog.String("path", path)}
	s.log.Info(k.created, append(attrs, k.attrs(req)...)...)

	service.Success(w, nil)
}

func updateStream[T any](s *App, k streamKind[T], w http.ResponseWriter, r *http.Request) {
	oldName := chi.URLParam(r, "name")
	if oldName == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	req, err := service.Bind[T](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldPath := k.path(oldName)
	old, err := os.ReadFile(oldPath)
	if errors.Is(err, fs.ErrNotExist) {
		service.Error(w, http.StatusNotFound, k.notFound, oldName)
		return
	}
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	name := k.name(req)
	path := k.path(name)
	renamed := path != oldPath
	if renamed {
		if _, statErr := os.Stat(path); statErr == nil {
			service.Error(w, http.StatusConflict, k.exists, name)
			return
		}
	}

	if err = os.WriteFile(path, []byte(k.render(req)), 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, k.writeFailed, err)
		return
	}
	if renamed {
		_ = os.Remove(oldPath)
	}

	if err = s.Reload(r.Context()); err != nil {
		// 新配置没通过检查，磁盘恢复成改动前的样子，否则其它模块的重载会一直被它拖垮
		if renamed {
			_ = os.Remove(path)
		}
		_ = os.WriteFile(oldPath, old, 0600)
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	attrs := []any{slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", name), slog.String("old_name", oldName), slog.String("path", path)}
	s.log.Info(k.updated, append(attrs, k.attrs(req)...)...)

	service.Success(w, nil)
}

func deleteStream[T any](s *App, k streamKind[T], w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		service.Error(w, http.StatusBadRequest, s.t.Get("name is required"))
		return
	}

	path := k.path(name)
	old, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		service.Error(w, http.StatusNotFound, k.notFound, name)
		return
	}
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = os.Remove(path); err != nil {
		service.Error(w, http.StatusInternalServerError, k.deleteFailed, err)
		return
	}

	if err = s.Reload(r.Context()); err != nil {
		// 多半是删掉了仍被引用的 upstream，放回去，否则其它模块的重载会一直被它拖垮
		_ = os.WriteFile(path, old, 0600)
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(k.deleted, slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("name", name), slog.String("path", path))

	service.Success(w, nil)
}

// parseStreamServerFile 解析单个 Stream Server 配置文件
func parseStreamServerFile(filePath string, name string) (*StreamServer, error) {
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

// parseStreamUpstreamFile 解析单个 Stream Upstream 配置文件
func parseStreamUpstreamFile(filePath string, expectedName string) (*StreamUpstream, error) {
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
	if name != expectedName {
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

// renderStreamServer 生成 Stream Server 配置
func renderStreamServer(server *StreamServer) string {
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

	return webserverNginx.Export(cfg)
}

// renderStreamUpstream 生成 Stream Upstream 配置
func renderStreamUpstream(upstream *StreamUpstream) string {
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
	slices.Sort(addrs)

	for _, addr := range addrs {
		options := upstream.Servers[addr]
		if options != "" {
			_, _ = fmt.Fprintf(&sb, "    server %s %s;\n", addr, options)
		} else {
			_, _ = fmt.Fprintf(&sb, "    server %s;\n", addr)
		}
	}

	sb.WriteString("}\n")

	return sb.String()
}

// parseNginxDuration 解析 Nginx 时间格式（如 500ms、10s、1h 30m），不带单位按秒计
func parseNginxDuration(value string) time.Duration {
	var total time.Duration
	for _, match := range nginxDurationRegexp.FindAllStringSubmatch(value, -1) {
		n, _ := strconv.ParseInt(match[1], 10, 64)
		total += time.Duration(n) * nginxDurationUnits[match[2]]
	}

	return total
}

// formatNginxDuration 格式化时间为 Nginx 格式，取能整除的最大单位
func formatNginxDuration(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	default:
		return fmt.Sprintf("%dms", d/time.Millisecond)
	}
}

// streamDir 返回 stream 配置目录
func streamDir() string {
	return filepath.Join(app.Root, "server/nginx/conf/stream")
}
