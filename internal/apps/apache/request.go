package apache

// ConfigTune Apache 配置调整
type ConfigTune struct {
	// MPM 事件模型
	StartServers           string `form:"start_servers" json:"start_servers"`
	MinSpareThreads        string `form:"min_spare_threads" json:"min_spare_threads"`
	MaxSpareThreads        string `form:"max_spare_threads" json:"max_spare_threads"`
	ThreadsPerChild        string `form:"threads_per_child" json:"threads_per_child"`
	MaxRequestWorkers      string `form:"max_request_workers" json:"max_request_workers"`
	MaxConnectionsPerChild string `form:"max_connections_per_child" json:"max_connections_per_child"`
	// 连接设置
	Timeout              string `form:"timeout" json:"timeout"`
	KeepAlive            string `form:"keep_alive" json:"keep_alive"`
	MaxKeepAliveRequests string `form:"max_keep_alive_requests" json:"max_keep_alive_requests"`
	KeepAliveTimeout     string `form:"keep_alive_timeout" json:"keep_alive_timeout"`
}
