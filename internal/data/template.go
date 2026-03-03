package data

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"
	"go.yaml.in/yaml/v4"

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

// localTemplateData data.yml 的 YAML 结构（与 github.com/acepanel/templates 仓库格式一致）
type localTemplateData struct {
	Name          map[string]string                       `yaml:"name"`
	Description   map[string]string                       `yaml:"description"`
	Website       string                                  `yaml:"website"`
	Categories    []string                                `yaml:"categories"`
	Architectures []string                                `yaml:"architectures"`
	Environments  map[string]localTemplateDataEnvironment `yaml:"environments"`
}

type localTemplateDataEnvironment struct {
	Description map[string]string `yaml:"description"`
	Type        string            `yaml:"type"`
	Options     map[string]string `yaml:"options,omitempty"`
	Default     any               `yaml:"default,omitempty"`
}

// loadLocalTemplates 从本地目录加载模板（与 github.com/acepanel/templates 仓库格式一致）
func (r *templateRepo) loadLocalTemplates() api.Templates {
	dir := filepath.Join(app.Root, "panel/storage/templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("failed to read templates directory", "path", dir, "error", err)
		}
		return nil
	}

	var templates api.Templates
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		slug := entry.Name()
		tplDir := filepath.Join(dir, slug)

		// 读取 data.yml
		dataPath := filepath.Join(tplDir, "data.yml")
		dataBytes, err := os.ReadFile(dataPath)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("failed to read template data.yml", "path", dataPath, "error", err)
			}
			continue
		}

		var data localTemplateData
		if err = yaml.Unmarshal(dataBytes, &data); err != nil {
			slog.Warn("failed to parse template data.yml", "path", dataPath, "error", err)
			continue
		}

		// 读取 docker-compose.yml
		composePath := filepath.Join(tplDir, "docker-compose.yml")
		composeBytes, err := os.ReadFile(composePath)
		if err != nil {
			slog.Warn("failed to read template docker-compose.yml", "path", composePath, "error", err)
			continue
		}

		// 构建模板
		t := &api.Template{
			Slug:          slug,
			Name:          resolveLocale(data.Name),
			Description:   resolveLocale(data.Description),
			Website:       data.Website,
			Categories:    data.Categories,
			Architectures: data.Architectures,
			Compose:       string(composeBytes),
			Local:         true,
		}

		// 转换环境变量（从 map 格式转为数组格式）
		for name, env := range data.Environments {
			t.Environments = append(t.Environments, struct {
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Type        string            `json:"type"`
				Options     map[string]string `json:"options,omitempty"`
				Default     any               `json:"default,omitempty"`
			}{
				Name:        name,
				Description: resolveLocale(env.Description),
				Type:        env.Type,
				Options:     env.Options,
				Default:     env.Default,
			})
		}

		// 读取 logo（优先 svg，其次 png）
		if icon := readLogo(tplDir); icon != "" {
			t.Icon = icon
		}

		templates = append(templates, t)
	}

	return templates
}

// resolveLocale 根据当前语言环境解析国际化字段
func resolveLocale(m map[string]string) string {
	if m == nil {
		return ""
	}
	// 优先使用当前语言
	if v, ok := m[app.Locale]; ok {
		return v
	}
	// 回退到英文
	if v, ok := m["en"]; ok {
		return v
	}
	// 返回任意值（最后的兜底，此时既无当前语言也无英文）
	for _, v := range m {
		return v
	}
	return ""
}

// readLogo 读取模板目录中的 logo 文件并返回 base64 data URI
func readLogo(dir string) string {
	candidates := []struct {
		name string
		mime string
	}{
		{"logo.svg", "image/svg+xml"},
		{"logo.png", "image/png"},
	}
	for _, c := range candidates {
		data, err := os.ReadFile(filepath.Join(dir, c.name))
		if err != nil {
			continue
		}
		return "data:" + c.mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return ""
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
