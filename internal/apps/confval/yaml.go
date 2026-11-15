package confval

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cast"
	"go.yaml.in/yaml/v4"
)

// LoadYAML 读取 YAML 配置文件，文件不存在、内容为空或只有注释时得到空 map，返回的 map 始终非 nil
func LoadYAML(path string) (map[string]any, error) {
	cfg := make(map[string]any)

	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err = yaml.Unmarshal(raw, &cfg); err != nil {
		return make(map[string]any), err
	}
	// 空文档会把 cfg 置回 nil
	if cfg == nil {
		cfg = make(map[string]any)
	}

	return cfg, nil
}

// GetYAML 读取 YAML 配置项，优先平铺键（安装脚本用 sed 生成的格式），回退到嵌套键
func GetYAML(cfg map[string]any, key string) string {
	if val, ok := cfg[key]; ok {
		return cast.ToString(val)
	}

	prefix, rest, nested := strings.Cut(key, ".")
	val, ok := cfg[prefix]
	if !ok {
		return ""
	}
	if !nested {
		return cast.ToString(val)
	}
	child, ok := val.(map[string]any)
	if !ok {
		return ""
	}

	return GetYAML(child, rest)
}

// SetYAML 以平铺键写入 YAML 配置项，同时清理可能残留的同名嵌套键
func SetYAML(cfg map[string]any, key string, value string) {
	if value == "" {
		return
	}

	cfg[key] = value
	prefix, rest, nested := strings.Cut(key, ".")
	if !nested {
		return
	}
	child, ok := cfg[prefix].(map[string]any)
	if !ok {
		return
	}
	delete(child, rest)
	if len(child) == 0 {
		delete(cfg, prefix)
	}
}

// SetYAMLNested 以嵌套键写入 YAML 配置项，逐层补建 map，同时清理可能残留的同名平铺键
func SetYAMLNested(cfg map[string]any, key string, value string) {
	if value == "" {
		return
	}

	prefix, rest, nested := strings.Cut(key, ".")
	if !nested {
		cfg[key] = value
		return
	}

	// GetYAML 优先读平铺键，留着会盖住刚写入的嵌套值
	delete(cfg, key)
	child, ok := cfg[prefix].(map[string]any)
	if !ok {
		child = make(map[string]any)
		cfg[prefix] = child
	}
	SetYAMLNested(child, rest, value)
}
