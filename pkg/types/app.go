package types

import "github.com/gofiber/fiber/v3"

// App 应用接口
type App interface {
	Route(r fiber.Router)
}

// AppCenter 应用中心结构
type AppCenter struct {
	Icon        string `json:"icon"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
	Channels    []struct {
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		Panel     string `json:"panel"`
		Install   string `json:"-"`
		Uninstall string `json:"-"`
		Update    string `json:"-"`
		Subs      []struct {
			Log     string `json:"log"`
			Version string `json:"version"`
		} `json:"subs"`
	} `json:"channels"`
	Installed        bool   `json:"installed"`
	InstalledChannel string `json:"installed_channel"`
	InstalledVersion string `json:"installed_version"`
	UpdateExist      bool   `json:"update_exist"`
	Show             bool   `json:"show"`
}
