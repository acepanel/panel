package nginx

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/tools"
	"github.com/acepanel/panel/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.SaveConfig)
	r.Get("/error_log", s.ErrorLog)
	r.Post("/clear_error_log", s.ClearErrorLog)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)

	r.Get("/stream/servers", s.ListStreamServers)
	r.Post("/stream/servers", s.CreateStreamServer)
	r.Put("/stream/servers/{name}", s.UpdateStreamServer)
	r.Delete("/stream/servers/{name}", s.DeleteStreamServer)
	r.Get("/stream/upstreams", s.ListStreamUpstreams)
	r.Post("/stream/upstreams", s.CreateStreamUpstream)
	r.Put("/stream/upstreams/{name}", s.UpdateStreamUpstream)
	r.Delete("/stream/upstreams/{name}", s.DeleteStreamUpstream)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

func (s *App) SaveConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root), req.Config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

func (s *App) ErrorLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, fmt.Sprintf("%s/%s", app.Root, "server/nginx/nginx-error.log"))
}

func (s *App) ClearErrorLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf("cat /dev/null > %s/%s", app.Root, "server/nginx/nginx-error.log"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	client := resty.New().SetTimeout(10 * time.Second)
	resp, err := client.R().Get("http://127.0.0.1/nginx_status")
	if err != nil || !resp.IsSuccess() {
		service.Success(w, []types.NV{})
		return
	}

	raw := resp.String()
	var data []types.NV

	workers, err := shell.Execf("ps aux | grep nginx | grep 'worker process' | wc -l")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
		return
	}
	data = append(data, types.NV{
		Name:  s.t.Get("Workers"),
		Value: workers,
	})

	out, err := shell.Execf("ps aux | grep nginx | grep 'worker process' | awk '{memsum+=$6};END {print memsum}'")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get nginx workers: %v", err))
		return
	}
	mem := tools.FormatBytes(cast.ToFloat64(out))
	data = append(data, types.NV{
		Name:  s.t.Get("Memory"),
		Value: mem,
	})

	match := regexp.MustCompile(`Active connections:\s+(\d+)`).FindStringSubmatch(raw)
	if len(match) == 2 {
		data = append(data, types.NV{
			Name:  s.t.Get("Active connections"),
			Value: match[1],
		})
	}

	match = regexp.MustCompile(`server accepts handled requests\s+(\d+)\s+(\d+)\s+(\d+)`).FindStringSubmatch(raw)
	if len(match) == 4 {
		data = append(data, types.NV{
			Name:  s.t.Get("Total connections"),
			Value: match[1],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Total handshakes"),
			Value: match[2],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Total requests"),
			Value: match[3],
		})
	}

	match = regexp.MustCompile(`Reading:\s+(\d+)\s+Writing:\s+(\d+)\s+Waiting:\s+(\d+)`).FindStringSubmatch(raw)
	if len(match) == 4 {
		data = append(data, types.NV{
			Name:  s.t.Get("Reading"),
			Value: match[1],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Writing"),
			Value: match[2],
		})
		data = append(data, types.NV{
			Name:  s.t.Get("Waiting"),
			Value: match[3],
		})
	}

	service.Success(w, data)
}

// GetConfigTune 获取 Nginx 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		// 常规设置
		WorkerProcesses:           s.getNginxValue(config, "worker_processes"),
		WorkerConnections:         s.getNginxValue(config, "worker_connections"),
		KeepaliveTimeout:          s.getNginxValue(config, "keepalive_timeout"),
		ClientMaxBodySize:         s.getNginxValue(config, "client_max_body_size"),
		ClientBodyBufferSize:      s.getNginxValue(config, "client_body_buffer_size"),
		ClientHeaderBufferSize:    s.getNginxValue(config, "client_header_buffer_size"),
		ServerNamesHashBucketSize: s.getNginxValue(config, "server_names_hash_bucket_size"),
		ServerTokens:              s.getNginxValue(config, "server_tokens"),
		// Gzip 压缩
		Gzip:          s.getNginxValue(config, "gzip"),
		GzipMinLength: s.getNginxValue(config, "gzip_min_length"),
		GzipCompLevel: s.getNginxValue(config, "gzip_comp_level"),
		GzipTypes:     s.getNginxValue(config, "gzip_types"),
		GzipVary:      s.getNginxValue(config, "gzip_vary"),
		GzipProxied:   s.getNginxValue(config, "gzip_proxied"),
		// Brotli 压缩
		Brotli:          s.getNginxValue(config, "brotli"),
		BrotliMinLength: s.getNginxValue(config, "brotli_min_length"),
		BrotliCompLevel: s.getNginxValue(config, "brotli_comp_level"),
		BrotliTypes:     s.getNginxValue(config, "brotli_types"),
		BrotliStatic:    s.getNginxValue(config, "brotli_static"),
		// Zstd 压缩
		Zstd:          s.getNginxValue(config, "zstd"),
		ZstdMinLength: s.getNginxValue(config, "zstd_min_length"),
		ZstdCompLevel: s.getNginxValue(config, "zstd_comp_level"),
		ZstdTypes:     s.getNginxValue(config, "zstd_types"),
		ZstdStatic:    s.getNginxValue(config, "zstd_static"),
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 Nginx 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := fmt.Sprintf("%s/server/nginx/conf/nginx.conf", app.Root)
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新常规设置
	config = s.setNginxValue(config, "worker_processes", req.WorkerProcesses)
	config = s.setNginxValue(config, "worker_connections", req.WorkerConnections)
	config = s.setNginxValue(config, "keepalive_timeout", req.KeepaliveTimeout)
	config = s.setNginxValue(config, "client_max_body_size", req.ClientMaxBodySize)
	config = s.setNginxValue(config, "client_body_buffer_size", req.ClientBodyBufferSize)
	config = s.setNginxValue(config, "client_header_buffer_size", req.ClientHeaderBufferSize)
	config = s.setNginxValue(config, "server_names_hash_bucket_size", req.ServerNamesHashBucketSize)
	config = s.setNginxValue(config, "server_tokens", req.ServerTokens)
	// 更新 Gzip 压缩
	config = s.setNginxValue(config, "gzip", req.Gzip)
	config = s.setNginxValue(config, "gzip_min_length", req.GzipMinLength)
	config = s.setNginxValue(config, "gzip_comp_level", req.GzipCompLevel)
	config = s.setNginxValue(config, "gzip_types", req.GzipTypes)
	config = s.setNginxValue(config, "gzip_vary", req.GzipVary)
	config = s.setNginxValue(config, "gzip_proxied", req.GzipProxied)
	// 更新 Brotli 压缩
	config = s.setNginxValue(config, "brotli", req.Brotli)
	config = s.setNginxValue(config, "brotli_min_length", req.BrotliMinLength)
	config = s.setNginxValue(config, "brotli_comp_level", req.BrotliCompLevel)
	config = s.setNginxValue(config, "brotli_types", req.BrotliTypes)
	config = s.setNginxValue(config, "brotli_static", req.BrotliStatic)
	// 更新 Zstd 压缩
	config = s.setNginxValue(config, "zstd", req.Zstd)
	config = s.setNginxValue(config, "zstd_min_length", req.ZstdMinLength)
	config = s.setNginxValue(config, "zstd_comp_level", req.ZstdCompLevel)
	config = s.setNginxValue(config, "zstd_types", req.ZstdTypes)
	config = s.setNginxValue(config, "zstd_static", req.ZstdStatic)

	if err = io.Write(confPath, config, 0600); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

// getNginxValue 从 Nginx 配置内容中获取指定指令的值
func (s *App) getNginxValue(content string, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasSuffix(trimmed, ";") {
			continue
		}
		trimmed = strings.TrimSuffix(trimmed, ";")
		trimmed = strings.TrimSpace(trimmed)
		parts := strings.Fields(trimmed)
		if len(parts) >= 2 && parts[0] == key {
			return strings.Join(parts[1:], " ")
		}
	}
	return ""
}

// setNginxValue 在 Nginx 配置内容中设置指定指令的值
func (s *App) setNginxValue(content string, key string, value string) string {
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	lines := strings.Split(content, "\n")
	found := false
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "{" || trimmed == "}" {
			result = append(result, line)
			continue
		}

		// 检查指令（可能被注释）
		checkLine := trimmed
		if strings.HasPrefix(checkLine, "#") {
			checkLine = strings.TrimSpace(checkLine[1:])
		}

		if !strings.HasSuffix(checkLine, ";") {
			result = append(result, line)
			continue
		}
		checkLine = strings.TrimSuffix(checkLine, ";")
		checkLine = strings.TrimSpace(checkLine)
		parts := strings.Fields(checkLine)
		if len(parts) >= 2 && parts[0] == key {
			if found {
				continue
			}
			found = true
			// 值为空时注释掉该配置项
			if value == "" {
				if !strings.HasPrefix(trimmed, "#") {
					indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
					result = append(result, indent+"#"+strings.TrimLeft(line, " \t"))
				} else {
					result = append(result, line)
				}
				continue
			}
			// 保留原行缩进
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			result = append(result, indent+key+" "+value+";")
		} else {
			result = append(result, line)
		}
	}
	if !found && value != "" {
		result = append(result, "    "+key+" "+value+";")
	}
	return strings.Join(result, "\n")
}
