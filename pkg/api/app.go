package api

import "time"

type App struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Categories  []string  `json:"categories"`
	Depends     string    `json:"depends"` // 依赖表达式
	Channels    []struct {
		Slug      string `json:"slug"`      // 渠道代号
		Name      string `json:"name"`      // 渠道名称
		Panel     string `json:"panel"`     // 最低支持面板版本
		Install   string `json:"install"`   // 安装脚本
		Uninstall string `json:"uninstall"` // 卸载脚本
		Update    string `json:"update"`    // 更新脚本
		Version   string `json:"version"`   // 版本号
		Log       string `json:"log"`       // 更新日志
	} `json:"channels"`
	Order int `json:"order"`
}

type Apps []*App

// Apps 返回所有应用
func (r *API) Apps() (*Apps, error) {
	return get[Apps](r, "/apps", nil)
}

// AppCallback 应用下载回调
func (r *API) AppCallback(slug string) error {
	return r.callback("/apps/" + slug + "/callback")
}
