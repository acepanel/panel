package postgresql

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

type SetPostgresPassword struct {
	Password string `form:"password" json:"password" validate:"required|password"`
}

// ConfigTune PostgreSQL 配置调整
type ConfigTune struct {
	// 连接设置
	ListenAddresses              string `form:"listen_addresses" json:"listen_addresses"`
	Port                         string `form:"port" json:"port"`
	MaxConnections               string `form:"max_connections" json:"max_connections"`
	SuperuserReservedConnections string `form:"superuser_reserved_connections" json:"superuser_reserved_connections"`
	// 内存设置
	SharedBuffers      string `form:"shared_buffers" json:"shared_buffers"`
	WorkMem            string `form:"work_mem" json:"work_mem"`
	MaintenanceWorkMem string `form:"maintenance_work_mem" json:"maintenance_work_mem"`
	EffectiveCacheSize string `form:"effective_cache_size" json:"effective_cache_size"`
	HugePages          string `form:"huge_pages" json:"huge_pages"`
	// WAL 设置
	WalLevel                   string `form:"wal_level" json:"wal_level"`
	WalBuffers                 string `form:"wal_buffers" json:"wal_buffers"`
	MaxWalSize                 string `form:"max_wal_size" json:"max_wal_size"`
	MinWalSize                 string `form:"min_wal_size" json:"min_wal_size"`
	CheckpointCompletionTarget string `form:"checkpoint_completion_target" json:"checkpoint_completion_target"`
	// 查询优化
	DefaultStatisticsTarget string `form:"default_statistics_target" json:"default_statistics_target"`
	RandomPageCost          string `form:"random_page_cost" json:"random_page_cost"`
	EffectiveIoConcurrency  string `form:"effective_io_concurrency" json:"effective_io_concurrency"`
	// 日志设置
	LogDestination          string `form:"log_destination" json:"log_destination"`
	LogMinDurationStatement string `form:"log_min_duration_statement" json:"log_min_duration_statement"`
	LogTimezone             string `form:"log_timezone" json:"log_timezone"`
	// IO 设置
	IoMethod string `form:"io_method" json:"io_method"`
}
