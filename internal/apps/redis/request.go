package redis

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

// ConfigTune Redis 配置调整
type ConfigTune struct {
	// 常规设置
	Bind         string `form:"bind" json:"bind"`
	Port         string `form:"port" json:"port"`
	Databases    string `form:"databases" json:"databases"`
	Requirepass  string `form:"requirepass" json:"requirepass"`
	Timeout      string `form:"timeout" json:"timeout"`
	TCPKeepalive string `form:"tcp_keepalive" json:"tcp_keepalive"`
	// 内存
	Maxmemory       string `form:"maxmemory" json:"maxmemory"`
	MaxmemoryPolicy string `form:"maxmemory_policy" json:"maxmemory_policy"`
	// 持久化
	Appendonly  string `form:"appendonly" json:"appendonly"`
	Appendfsync string `form:"appendfsync" json:"appendfsync"`
}
