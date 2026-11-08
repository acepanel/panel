package data

import (
	"encoding/base64"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/leonelquinteros/gotext"
	"go.yaml.in/yaml/v4"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/types"
)

type templateRepo struct {
	t   *gotext.Locale
	log *slog.Logger
	api *api.API
}

func NewTemplateRepo(t *gotext.Locale, log *slog.Logger) biz.TemplateRepo {
	return &templateRepo{
		t:   t,
		log: log,
		api: api.NewAPI(app.Version, app.Locale),
	}
}

// Callback 模版下载回调
func (r *templateRepo) Callback(slug string) error {
	return r.api.TemplateCallback(slug)
}

// WriteCompose 写入编排文件
func (r *templateRepo) WriteCompose(name, compose string, envs []types.KV) (string, error) {
	dir := filepath.Join(app.Root, "compose", name)

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

	return dir, nil
}

// LoadLocalTemplates 从本地目录加载模板
func (r *templateRepo) LoadLocalTemplates() api.Templates {
	dir := filepath.Join(app.Root, "panel/storage/templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			r.log.Warn(r.t.Get("failed to read templates directory"), "path", dir, "err", err)
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
			r.log.Warn(r.t.Get("failed to read template data.yml"), "path", dataPath, "err", err)
			continue
		}

		var data types.TemplateData
		if err = yaml.Unmarshal(dataBytes, &data); err != nil {
			r.log.Warn(r.t.Get("failed to parse template data.yml"), "path", dataPath, "err", err)
			continue
		}

		// 读取 docker-compose.yml
		composePath := filepath.Join(tplDir, "docker-compose.yml")
		composeBytes, err := os.ReadFile(composePath)
		if err != nil {
			r.log.Warn(r.t.Get("failed to read template docker-compose.yml"), "path", composePath, "err", err)
			continue
		}

		// 构建模板
		t := &api.Template{
			Slug:          slug,
			Name:          r.resolveLocale(data.Name),
			Description:   r.resolveLocale(data.Description),
			Website:       data.Website,
			Categories:    data.Categories,
			Architectures: data.Architectures,
			Compose:       string(composeBytes),
			Local:         true,
		}

		// 转换环境变量，从 map 格式转为数组格式，保留 YAML 定义顺序
		names := r.extractEnvKeyOrder(dataBytes, "environments")
		for _, name := range names {
			env := data.Environments[name]
			t.Environments = append(t.Environments, struct {
				Name        string            `json:"name"`
				Description string            `json:"description"`
				Type        string            `json:"type"`
				Options     map[string]string `json:"options,omitempty"`
				Default     any               `json:"default,omitempty"`
			}{
				Name:        name,
				Description: r.resolveLocale(env.Description),
				Type:        env.Type,
				Options:     env.Options,
				Default:     env.Default,
			})
		}

		// 读取 logo
		if icon := r.readLogo(tplDir); icon != "" {
			t.Icon = icon
		}

		templates = append(templates, t)
	}

	return templates
}

// resolveLocale 根据当前语言环境解析国际化字段
func (r *templateRepo) resolveLocale(m map[string]string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[app.Locale]; ok {
		return v
	}
	if v, ok := m["en"]; ok {
		return v
	}
	// 按 key 排序取第一个，保证结果稳定
	keys := slices.Sorted(maps.Keys(m))
	if len(keys) > 0 {
		return m[keys[0]]
	}
	return ""
}

// extractEnvKeyOrder 从原始 YAML 数据中提取指定 mapping 字段的键定义顺序
func (r *templateRepo) extractEnvKeyOrder(data []byte, field string) []string {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil
	}

	if node.Kind != yaml.DocumentNode || len(node.Content) == 0 {
		return nil
	}
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == field {
			m := root.Content[i+1]
			if m.Kind != yaml.MappingNode {
				return nil
			}
			keys := make([]string, 0, len(m.Content)/2)
			for j := 0; j+1 < len(m.Content); j += 2 {
				keys = append(keys, m.Content[j].Value)
			}
			return keys
		}
	}

	return nil
}

// readLogo 读取模板目录中的 logo 文件并返回 base64 data URI
func (r *templateRepo) readLogo(dir string) string {
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
