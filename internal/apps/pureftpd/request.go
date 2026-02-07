package pureftpd

type Create struct {
	Username string `form:"username" json:"username" validate:"required"`
	Password string `form:"password" json:"password" validate:"required|password"`
	Path     string `form:"path" json:"path" validate:"required"`
}

type Delete struct {
	Username string `form:"username" json:"username" validate:"required"`
}

type ChangePassword struct {
	Username string `form:"username" json:"username" validate:"required"`
	Password string `form:"password" json:"password" validate:"required|password"`
}

type UpdatePort struct {
	Port uint `form:"port" json:"port" validate:"required|number|min:1|max:65535"`
}

// ConfigTune Pure-FTPd 配置调整
type ConfigTune struct {
	MaxClientsNumber string `form:"max_clients_number" json:"max_clients_number"`
	MaxClientsPerIP  string `form:"max_clients_per_ip" json:"max_clients_per_ip"`
	MaxIdleTime      string `form:"max_idle_time" json:"max_idle_time"`
	MaxLoad          string `form:"max_load" json:"max_load"`
	PassivePortRange string `form:"passive_port_range" json:"passive_port_range"`
	AnonymousOnly    string `form:"anonymous_only" json:"anonymous_only"`
	NoAnonymous      string `form:"no_anonymous" json:"no_anonymous"`
	MaxDiskUsage     string `form:"max_disk_usage" json:"max_disk_usage"`
}
