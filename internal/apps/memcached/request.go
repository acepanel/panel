package memcached

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

// ConfigTune Memcached 配置调整
type ConfigTune struct {
	Port           string `form:"port" json:"port"`
	UDPPort        string `form:"udp_port" json:"udp_port"`
	ListenAddress  string `form:"listen_address" json:"listen_address"`
	Memory         string `form:"memory" json:"memory"`
	MaxConnections string `form:"max_connections" json:"max_connections"`
	Threads        string `form:"threads" json:"threads"`
}
