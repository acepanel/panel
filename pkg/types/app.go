package types

import (
	"context"

	"github.com/go-chi/chi/v5"
)

// 应用运行状态
const (
	AppStatusRunning = "running" // 正在运行
	AppStatusStopped = "stopped" // 已停止
	AppStatusPartial = "partial" // 部分运行（多服务应用）
	AppStatusNA      = "n/a"     // 不适用（无 systemd 服务的应用）
)

// App 应用接口
type App interface {
	Route(r chi.Router)
	Status(ctx context.Context) string
}

// AppDetail 应用详情
type AppDetail struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	Slug        string   `json:"slug"`
	Channels    []struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		Panel   string `json:"panel"`
		Version string `json:"version"`
		Log     string `json:"log"`
	} `json:"channels"`
	Installed        bool   `json:"installed"`
	InstalledChannel string `json:"installed_channel"`
	InstalledVersion string `json:"installed_version"`
	UpdateExist      bool   `json:"update_exist"`
	Show             bool   `json:"show"`
	Status           string `json:"status"`
	CustomSupported  bool   `json:"custom_supported"`
}

// AggregateAppStatus 根据各服务运行状态聚合应用状态
func AggregateAppStatus(running ...bool) string {
	if len(running) == 0 {
		return AppStatusNA
	}
	count := 0
	for _, r := range running {
		if r {
			count++
		}
	}
	switch count {
	case 0:
		return AppStatusStopped
	case len(running):
		return AppStatusRunning
	default:
		return AppStatusPartial
	}
}
