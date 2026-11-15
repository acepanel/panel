package request

import "time"

type ToolboxSystemSWAP struct {
	Size int64 `form:"size" json:"size" validate:"min:0"`
}

type ToolboxSystemTimezone struct {
	Timezone string `form:"timezone" json:"timezone" validate:"required"`
}

type ToolboxSystemTime struct {
	Time time.Time `form:"time" json:"time" validate:"required"`
}

type ToolboxSystemHostname struct {
	Hostname string `form:"hostname" json:"hostname" validate:"required && regex:\"^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]$\""`
}

type ToolboxSystemHosts struct {
	Hosts string `form:"hosts" json:"hosts"`
}

type ToolboxSystemSyncTime struct {
	Server string `form:"server" json:"server"` // 可选的 NTP 服务器地址
}

type ToolboxSystemNTPServers struct {
	Servers []string `form:"servers" json:"servers" validate:"required && unique"`
}
