package api

import (
	"slices"
	"strings"
	"time"
)

type Template struct {
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Slug          string    `json:"slug"`
	Icon          string    `json:"icon"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Website       string    `json:"website"`
	Categories    []string  `json:"categories"`
	Architectures []string  `json:"architectures"`
	Compose       string    `json:"compose"`
	Environments  []struct {
		Name        string            `json:"name"`              // 变量名
		Description string            `json:"description"`       // 变量描述
		Type        string            `json:"type"`              // 变量类型， text, password, number, port, select
		Options     map[string]string `json:"options,omitempty"` // 下拉框选项，key -> value
		Default     any               `json:"default,omitempty"` // 默认值，string or number
	} `json:"environments"`
	Local bool `json:"local"` // 是否为本地模板
}

type Templates []*Template

// FilterByCategory 按分类过滤模版
func (t Templates) FilterByCategory(category string) Templates {
	filtered := make(Templates, 0)
	for _, tpl := range t {
		if slices.Contains(tpl.Categories, category) {
			filtered = append(filtered, tpl)
		}
	}
	return filtered
}

// FilterByQuery 按关键词过滤模版
func (t Templates) FilterByQuery(query string) Templates {
	query = strings.ToLower(query)
	filtered := make(Templates, 0)
	for _, tpl := range t {
		if strings.Contains(strings.ToLower(tpl.Name), query) ||
			strings.Contains(strings.ToLower(tpl.Description), query) ||
			strings.Contains(strings.ToLower(tpl.Website), query) {
			filtered = append(filtered, tpl)
		}
	}
	return filtered
}

// Templates 返回所有模版
func (r *API) Templates() (*Templates, error) {
	return get[Templates](r, "/templates", nil)
}

// TemplateCallback 模版下载回调
func (r *API) TemplateCallback(slug string) error {
	return r.callback("/templates/" + slug + "/callback")
}
