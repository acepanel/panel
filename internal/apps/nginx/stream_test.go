package nginx

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/service"
)

func TestParseNginxDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"":       0,
		"30":     30 * time.Second,
		"10s":    10 * time.Second,
		"500ms":  500 * time.Millisecond,
		"5m":     5 * time.Minute,
		"2h":     2 * time.Hour,
		"1d":     24 * time.Hour,
		"1h 30m": 90 * time.Minute,
		"1h30m":  90 * time.Minute,
	}
	for value, want := range cases {
		check.Equal(t, parseNginxDuration(value), want)
	}
}

func TestFormatNginxDuration(t *testing.T) {
	cases := map[time.Duration]string{
		500 * time.Millisecond:  "500ms",
		1500 * time.Millisecond: "1500ms",
		90 * time.Second:        "90s",
		5 * time.Minute:         "5m",
		2 * time.Hour:           "2h",
	}
	for d, want := range cases {
		check.Equal(t, formatNginxDuration(d), want)
		// 写出去的值读回来不能变
		check.Equal(t, parseNginxDuration(want), d)
	}
}

func bindStreamServer(t *testing.T, pathName, body string) (*StreamServer, error) {
	t.Helper()

	var got *StreamServer
	var bindErr error
	router := chi.NewRouter()
	router.Put("/stream/servers/{name}", func(_ http.ResponseWriter, r *http.Request) {
		got, bindErr = service.Bind[StreamServer](r)
	})

	req := httptest.NewRequest(http.MethodPut, "/stream/servers/"+pathName, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), req)

	return got, bindErr
}

func TestStreamServerNameNotOverriddenByPath(t *testing.T) {
	got, err := bindStreamServer(t, "original", `{"name":"renamed","listen":"12345","proxy_pass":"127.0.0.1:3306"}`)
	must.NoError(t, err)
	// 路径里的是旧名，请求体里的新名不能被它盖掉，否则重命名永远不生效
	check.Equal(t, got.Name, "renamed")
}

func TestStreamServerNameRejectsUpstreamPrefix(t *testing.T) {
	// upstream_ 前缀的文件会被当成 Upstream 配置
	_, err := bindStreamServer(t, "original", `{"name":"upstream_db","listen":"12345","proxy_pass":"127.0.0.1:3306"}`)
	check.Error(t, err)

	_, err = bindStreamServer(t, "original", `{"name":"db_upstream_1","listen":"12345","proxy_pass":"127.0.0.1:3306"}`)
	check.NoError(t, err)
}

func TestStreamKindListSeparatesServersAndUpstreams(t *testing.T) {
	root := app.Root
	app.Root = t.TempDir()
	t.Cleanup(func() { app.Root = root })
	must.NoError(t, os.MkdirAll(streamDir(), 0755))

	s := &App{t: gotext.NewLocale("", "en")}
	server, upstream := s.streamServer(), s.streamUpstream()
	files := map[string]string{
		server.path("web"):                      renderStreamServer(&StreamServer{Name: "web", Listen: "8080", ProxyPass: "backend", ProxyTimeout: 500 * time.Millisecond}),
		server.path("a-b"):                      renderStreamServer(&StreamServer{Name: "a-b", Listen: "8081", UDP: true, ProxyPass: "127.0.0.1:53"}),
		server.path("a"):                        renderStreamServer(&StreamServer{Name: "a", Listen: "8082", ProxyPass: "127.0.0.1:80"}),
		upstream.path("backend"):                renderStreamUpstream(&StreamUpstream{Name: "backend", Algo: "least_conn", Servers: map[string]string{"127.0.0.1:3306": "weight=5"}, Resolver: []string{"8.8.8.8"}, ResolverTimeout: 5 * time.Second}),
		filepath.Join(streamDir(), "notes.txt"): "ignored",
	}
	for path, content := range files {
		must.NoError(t, os.WriteFile(path, []byte(content), 0600))
	}

	servers, err := server.list()
	must.NoError(t, err)
	must.Equal(t, len(servers), 3)
	// 按名称而不是文件名排序
	check.Equal(t, servers[0].Name, "a")
	check.Equal(t, servers[1].Name, "a-b")
	check.True(t, servers[1].UDP)
	check.Equal(t, servers[2].Name, "web")
	check.Equal(t, servers[2].ProxyTimeout, 500*time.Millisecond)

	upstreams, err := upstream.list()
	must.NoError(t, err)
	must.Equal(t, len(upstreams), 1)
	check.Equal(t, upstreams[0].Name, "backend")
	check.Equal(t, upstreams[0].Algo, "least_conn")
	check.DeepEqual(t, upstreams[0].Servers, map[string]string{"127.0.0.1:3306": "weight=5"})
	check.Equal(t, upstreams[0].ResolverTimeout, 5*time.Second)
}
