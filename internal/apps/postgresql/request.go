package postgresql

type SetPostgresPassword struct {
	Password string `form:"password" json:"password" validate:"required && password"`
}

// Extension PostgreSQL 扩展信息
type Extension struct {
	Name             string `json:"name"`
	Slug             string `json:"slug"`     // 下载脚本名，如 pgvector
	ExtName          string `json:"ext_name"` // CREATE EXTENSION 使用的扩展名，如 vector
	Description      string `json:"description"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installed_version"` // control 文件中的 default_version，仅展示
}

// ExtensionSlug 扩展操作请求
type ExtensionSlug struct {
	Slug string `form:"slug" json:"slug" validate:"required"`
}

// ExtensionEnable 在指定数据库启用扩展请求
type ExtensionEnable struct {
	Slug     string `form:"slug" json:"slug" validate:"required"`
	Database string `form:"database" json:"database" validate:"required"`
}

// Session 数据库会话信息
type Session struct {
	PID           int64  `json:"pid"`
	Database      string `json:"database"`
	User          string `json:"user"`
	ClientAddr    string `json:"client_addr"`
	State         string `json:"state"`
	WaitEventType string `json:"wait_event_type"`
	WaitEvent     string `json:"wait_event"`
	BlockedBy     string `json:"blocked_by"`
	XactSeconds   int64  `json:"xact_seconds"`
	QuerySeconds  int64  `json:"query_seconds"`
	Query         string `json:"query"`
}

// TopSQLItem Top SQL 统计项
type TopSQLItem struct {
	Database string  `json:"database"`
	Calls    int64   `json:"calls"`
	TotalMs  int64   `json:"total_ms"`
	MeanMs   float64 `json:"mean_ms"`
	Rows     int64   `json:"rows"`
	HitRate  float64 `json:"hit_rate"`
	Query    string  `json:"query"`
}

// TopSQL Top SQL 响应
type TopSQL struct {
	Enabled        bool         `json:"enabled"`
	PendingRestart bool         `json:"pending_restart"` // 已配置 preload，等待重启生效
	Items          []TopSQLItem `json:"items"`
}

// BloatQuery 表膨胀查询请求
type BloatQuery struct {
	Database string `form:"database" json:"database" validate:"required"`
}

// BloatItem 表膨胀信息
type BloatItem struct {
	Schema         string  `json:"schema"`
	Table          string  `json:"table"`
	Size           string  `json:"size"`
	SizeBytes      int64   `json:"size_bytes"`
	LiveTuples     int64   `json:"live_tuples"`
	DeadTuples     int64   `json:"dead_tuples"`
	DeadRate       float64 `json:"dead_rate"`
	LastVacuum     string  `json:"last_vacuum"`
	LastAutovacuum string  `json:"last_autovacuum"`
	LastAnalyze    string  `json:"last_analyze"`
	LastAutoanalyz string  `json:"last_autoanalyze"`
}

// Bloat 表膨胀响应
type Bloat struct {
	RepackInstalled bool        `json:"repack_installed"`
	Items           []BloatItem `json:"items"`
}

// MaintenanceTable 维护操作的目标表
type MaintenanceTable struct {
	Schema string `form:"schema" json:"schema" validate:"required"`
	Table  string `form:"table" json:"table" validate:"required"`
}

// MaintenanceRun 表维护操作请求
type MaintenanceRun struct {
	Database  string             `form:"database" json:"database" validate:"required"`
	Tables    []MaintenanceTable `form:"tables" json:"tables"`
	Operation string             `form:"operation" json:"operation" validate:"required"`
}

// ReplicationSlot 复制槽信息
type ReplicationSlot struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Active      bool   `json:"active"`
	RetainedWal string `json:"retained_wal"`
}

// Replication 流复制连接信息
type Replication struct {
	ClientAddr string `json:"client_addr"`
	State      string `json:"state"`
	SyncState  string `json:"sync_state"`
	Lag        string `json:"lag"`
}

// WalArchiver WAL 归档统计
type WalArchiver struct {
	ArchivedCount   int64  `json:"archived_count"`
	FailedCount     int64  `json:"failed_count"`
	LastArchivedWal string `json:"last_archived_wal"`
	LastFailedWal   string `json:"last_failed_wal"`
}

// Wal WAL 状态响应
type Wal struct {
	WalSize      string            `json:"wal_size"`
	Archiver     WalArchiver       `json:"archiver"`
	Slots        []ReplicationSlot `json:"slots"`
	Replications []Replication     `json:"replications"`
}

// ConfigTune PostgreSQL 配置调整
type ConfigTune struct {
	// 连接设置
	ListenAddresses              string `form:"listen_addresses" json:"listen_addresses"`
	Port                         string `form:"port" json:"port" validate:"number && min:1 && max:65535"`
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
