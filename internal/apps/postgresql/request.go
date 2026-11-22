package postgresql

import "github.com/acepanel/panel/v3/internal/request"

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

// TopSQLQuery Top SQL 查询请求
type TopSQLQuery struct {
	request.Paginate
	Sort  string `form:"sort" json:"sort" query:"sort" validate:"in:calls,total_ms,mean_ms,rows,hit_rate"`
	Order string `form:"order" json:"order" query:"order" validate:"in:asc,desc"`
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
	Total          uint         `json:"total"`
	Items          []TopSQLItem `json:"items"`
}

// BloatQuery 表膨胀查询请求
type BloatQuery struct {
	request.Paginate
	Database string `form:"database" json:"database" validate:"required"`
	Sort     string `form:"sort" json:"sort" query:"sort" validate:"in:size,live_tuples,dead_tuples,dead_rate,last_vacuum,last_analyze"`
	Order    string `form:"order" json:"order" query:"order" validate:"in:asc,desc"`
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
	Total           uint        `json:"total"`
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
	ListenAddresses                 string `form:"listen_addresses" json:"listen_addresses"`
	Port                            string `form:"port" json:"port" validate:"number && min:1 && max:65535"`
	MaxConnections                  string `form:"max_connections" json:"max_connections"`
	SuperuserReservedConnections    string `form:"superuser_reserved_connections" json:"superuser_reserved_connections"`
	IdleInTransactionSessionTimeout string `form:"idle_in_transaction_session_timeout" json:"idle_in_transaction_session_timeout"`
	// 内存设置
	SharedBuffers      string `form:"shared_buffers" json:"shared_buffers"`
	WorkMem            string `form:"work_mem" json:"work_mem"`
	HashMemMultiplier  string `form:"hash_mem_multiplier" json:"hash_mem_multiplier"`
	MaintenanceWorkMem string `form:"maintenance_work_mem" json:"maintenance_work_mem"`
	TempBuffers        string `form:"temp_buffers" json:"temp_buffers"`
	EffectiveCacheSize string `form:"effective_cache_size" json:"effective_cache_size"`
	HugePages          string `form:"huge_pages" json:"huge_pages"`
	// WAL 设置
	WalLevel                   string `form:"wal_level" json:"wal_level"`
	MaxWalSenders              string `form:"max_wal_senders" json:"max_wal_senders"`
	WalBuffers                 string `form:"wal_buffers" json:"wal_buffers"`
	WalCompression             string `form:"wal_compression" json:"wal_compression"`
	SynchronousCommit          string `form:"synchronous_commit" json:"synchronous_commit"`
	MaxWalSize                 string `form:"max_wal_size" json:"max_wal_size"`
	MinWalSize                 string `form:"min_wal_size" json:"min_wal_size"`
	CheckpointTimeout          string `form:"checkpoint_timeout" json:"checkpoint_timeout"`
	CheckpointCompletionTarget string `form:"checkpoint_completion_target" json:"checkpoint_completion_target"`
	// 查询优化
	DefaultStatisticsTarget  string `form:"default_statistics_target" json:"default_statistics_target"`
	RandomPageCost           string `form:"random_page_cost" json:"random_page_cost"`
	EffectiveIoConcurrency   string `form:"effective_io_concurrency" json:"effective_io_concurrency"`
	MaintenanceIoConcurrency string `form:"maintenance_io_concurrency" json:"maintenance_io_concurrency"`
	// 并行设置
	MaxWorkerProcesses            string `form:"max_worker_processes" json:"max_worker_processes"`
	MaxParallelWorkers            string `form:"max_parallel_workers" json:"max_parallel_workers"`
	MaxParallelWorkersPerGather   string `form:"max_parallel_workers_per_gather" json:"max_parallel_workers_per_gather"`
	MaxParallelMaintenanceWorkers string `form:"max_parallel_maintenance_workers" json:"max_parallel_maintenance_workers"`
	// 自动清理
	AutovacuumMaxWorkers         string `form:"autovacuum_max_workers" json:"autovacuum_max_workers"`
	AutovacuumWorkMem            string `form:"autovacuum_work_mem" json:"autovacuum_work_mem"`
	AutovacuumVacuumCostLimit    string `form:"autovacuum_vacuum_cost_limit" json:"autovacuum_vacuum_cost_limit"`
	AutovacuumVacuumScaleFactor  string `form:"autovacuum_vacuum_scale_factor" json:"autovacuum_vacuum_scale_factor"`
	AutovacuumAnalyzeScaleFactor string `form:"autovacuum_analyze_scale_factor" json:"autovacuum_analyze_scale_factor"`
	// 日志设置
	LogDestination          string `form:"log_destination" json:"log_destination"`
	LogStatement            string `form:"log_statement" json:"log_statement"`
	LogMinDurationStatement string `form:"log_min_duration_statement" json:"log_min_duration_statement"`
	LogLockWaits            string `form:"log_lock_waits" json:"log_lock_waits"`
	LogTempFiles            string `form:"log_temp_files" json:"log_temp_files"`
	LogTimezone             string `form:"log_timezone" json:"log_timezone"`
	// IO 与存储设置
	IoMethod                string `form:"io_method" json:"io_method"`
	DefaultToastCompression string `form:"default_toast_compression" json:"default_toast_compression"`
}
