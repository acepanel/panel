package types

// TemplateData data.yml 的 YAML 结构
type TemplateData struct {
	Name          map[string]string                    `yaml:"name"`
	Description   map[string]string                    `yaml:"description"`
	Website       string                               `yaml:"website"`
	Categories    []string                             `yaml:"categories"`
	Architectures []string                             `yaml:"architectures"`
	Environments  map[string]TemplateDataEnvironment   `yaml:"environments"`
}

// TemplateDataEnvironment 模板环境变量定义
type TemplateDataEnvironment struct {
	Description map[string]string `yaml:"description"`
	Type        string            `yaml:"type"`
	Options     map[string]string `yaml:"options,omitempty"`
	Default     any               `yaml:"default,omitempty"`
}
