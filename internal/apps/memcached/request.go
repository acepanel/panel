package memcached

// ConfigTune Memcached 配置调整
type ConfigTune struct {
	Port           string `form:"port" json:"port" validate:"number && min:1 && max:65535"`
	UDPPort        string `form:"udp_port" json:"udp_port" validate:"number && min:0 && max:65535"`
	ListenAddress  string `form:"listen_address" json:"listen_address"`
	Memory         string `form:"memory" json:"memory" validate:"number && min:1"`
	MaxConnections string `form:"max_connections" json:"max_connections" validate:"number && min:1"`
	Threads        string `form:"threads" json:"threads" validate:"number && min:1"`
}
