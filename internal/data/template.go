package data

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/types"
)

type templateRepo struct {
	t        *gotext.Locale
	cache    biz.CacheRepo
	api      *api.API
	firewall firewall.Firewall
}

func NewTemplateRepo(t *gotext.Locale, cache biz.CacheRepo) biz.TemplateRepo {
	return &templateRepo{
		t:        t,
		cache:    cache,
		api:      api.NewAPI(app.Version, app.Locale),
		firewall: firewall.NewFirewall(),
	}
}

// List 获取所有模版（包括本地模板）
func (r *templateRepo) List() api.Templates {
	cached, err := r.cache.Get(biz.CacheKeyTemplates)
	if err != nil {
		return nil
	}
	templates := make(api.Templates, 0)
	if err = json.Unmarshal([]byte(cached), &templates); err != nil {
		return nil
	}

	// 加载本地模板并合并（本地模板覆盖同 slug 的远端模板）
	localTemplates := r.loadLocalTemplates()
	if len(localTemplates) > 0 {
		slugMap := make(map[string]int, len(templates))
		for i, t := range templates {
			slugMap[t.Slug] = i
		}
		for _, lt := range localTemplates {
			if i, ok := slugMap[lt.Slug]; ok {
				templates[i] = lt
			} else {
				templates = append(templates, lt)
			}
		}
	}

	return templates
}

// loadLocalTemplates 从本地目录加载模板
func (r *templateRepo) loadLocalTemplates() api.Templates {
	dir := filepath.Join(app.Root, "templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var templates api.Templates
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		t := new(api.Template)
		if err = json.Unmarshal(data, t); err != nil {
			continue
		}
		if t.Slug == "" {
			continue
		}
		t.Local = true
		templates = append(templates, t)
	}

	return templates
}

// Get 获取模版详情
func (r *templateRepo) Get(slug string) (*api.Template, error) {
	templates := r.List()

	for _, t := range templates {
		if t.Slug == slug {
			return t, nil
		}
	}

	return nil, errors.New(r.t.Get("template %s not found", slug))
}

// Callback 模版下载回调
func (r *templateRepo) Callback(slug string) error {
	return r.api.TemplateCallback(slug)
}

// CreateCompose 创建编排
func (r *templateRepo) CreateCompose(name, compose string, envs []types.KV, autoFirewall bool) (string, error) {
	dir := filepath.Join(app.Root, "compose", name)

	// 检查编排是否已存在
	if _, err := os.Stat(dir); err == nil {
		return "", errors.New(r.t.Get("compose %s already exists", name))
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, kv := range envs {
		sb.WriteString(kv.Key)
		sb.WriteString("=")
		sb.WriteString(kv.Value)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(sb.String()), 0644); err != nil {
		return "", err
	}

	// 自动放行端口
	if autoFirewall {
		ports := r.parsePortsFromCompose(compose)
		for _, port := range ports {
			_ = r.firewall.Port(firewall.FireInfo{
				Family:    "ipv4",
				PortStart: port.Port,
				PortEnd:   port.Port,
				Protocol:  port.Protocol,
				Strategy:  firewall.StrategyAccept,
				Direction: "in",
			}, firewall.OperationAdd)
		}
	}

	return dir, nil
}

type composePort struct {
	Port     uint
	Protocol firewall.Protocol
}

// parsePortsFromCompose 从 compose 文件中解析端口
func (r *templateRepo) parsePortsFromCompose(compose string) []composePort {
	var ports []composePort
	seen := make(map[string]bool)

	// 匹配 ports 部分的端口映射
	// 支持格式: "8080:80", "8080:80/tcp", "8080:80/udp", "80", "80/tcp"
	portRegex := regexp.MustCompile(`(?m)^\s*-\s*["']?(\d+)(?::\d+)?(?:/(\w+))?["']?\s*$`)
	matches := portRegex.FindAllStringSubmatch(compose, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		portStr := match[1]
		protocol := firewall.ProtocolTCP
		if len(match) > 2 && match[2] != "" {
			switch strings.ToLower(match[2]) {
			case "udp":
				protocol = firewall.ProtocolUDP
			case "tcp":
				protocol = firewall.ProtocolTCP
			}
		}

		// 去重
		key := portStr + "/" + string(protocol)
		if seen[key] {
			continue
		}
		seen[key] = true

		var port uint
		if _, _, found := strings.Cut(portStr, ":"); found {
			// 格式: host:container
			parts := strings.Split(portStr, ":")
			if len(parts) > 0 {
				port = parseUint(parts[0])
			}
		} else {
			port = parseUint(portStr)
		}

		if port > 0 && port <= 65535 {
			ports = append(ports, composePort{
				Port:     port,
				Protocol: protocol,
			})
		}
	}

	return ports
}

func parseUint(s string) uint {
	var n uint
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + uint(c-'0')
		} else {
			break
		}
	}
	return n
}
