package mysql

type SetRootPassword struct {
	Password string `form:"password" json:"password" validate:"required && password"`
}

// ConfigTune MySQL 配置调整
type ConfigTune struct {
	// 常规设置
	Port                 string `form:"port" json:"port" validate:"number && min:1 && max:65535"`
	MaxConnections       string `form:"max_connections" json:"max_connections"`
	MaxConnectErrors     string `form:"max_connect_errors" json:"max_connect_errors"`
	DefaultStorageEngine string `form:"default_storage_engine" json:"default_storage_engine"`
	TableOpenCache       string `form:"table_open_cache" json:"table_open_cache"`
	MaxAllowedPacket     string `form:"max_allowed_packet" json:"max_allowed_packet"`
	OpenFilesLimit       string `form:"open_files_limit" json:"open_files_limit"`
	// 性能调整
	KeyBufferSize        string `form:"key_buffer_size" json:"key_buffer_size"`
	SortBufferSize       string `form:"sort_buffer_size" json:"sort_buffer_size"`
	ReadBufferSize       string `form:"read_buffer_size" json:"read_buffer_size"`
	ReadRndBufferSize    string `form:"read_rnd_buffer_size" json:"read_rnd_buffer_size"`
	JoinBufferSize       string `form:"join_buffer_size" json:"join_buffer_size"`
	ThreadCacheSize      string `form:"thread_cache_size" json:"thread_cache_size"`
	ThreadStack          string `form:"thread_stack" json:"thread_stack"`
	TmpTableSize         string `form:"tmp_table_size" json:"tmp_table_size"`
	MaxHeapTableSize     string `form:"max_heap_table_size" json:"max_heap_table_size"`
	MyisamSortBufferSize string `form:"myisam_sort_buffer_size" json:"myisam_sort_buffer_size"`
	// InnoDB
	InnodbBufferPoolSize      string `form:"innodb_buffer_pool_size" json:"innodb_buffer_pool_size"`
	InnodbLogBufferSize       string `form:"innodb_log_buffer_size" json:"innodb_log_buffer_size"`
	InnodbFlushLogAtTrxCommit string `form:"innodb_flush_log_at_trx_commit" json:"innodb_flush_log_at_trx_commit" validate:"in:0,1,2"`
	InnodbLockWaitTimeout     string `form:"innodb_lock_wait_timeout" json:"innodb_lock_wait_timeout"`
	InnodbMaxDirtyPagesPct    string `form:"innodb_max_dirty_pages_pct" json:"innodb_max_dirty_pages_pct"`
	InnodbReadIoThreads       string `form:"innodb_read_io_threads" json:"innodb_read_io_threads"`
	InnodbWriteIoThreads      string `form:"innodb_write_io_threads" json:"innodb_write_io_threads"`
	// 日志
	SlowQueryLog  string `form:"slow_query_log" json:"slow_query_log"`
	LongQueryTime string `form:"long_query_time" json:"long_query_time"`
}

// TableQuery 表维护列表查询请求
type TableQuery struct {
	Database string `form:"database" json:"database"`
}

// MaintenanceTable 维护操作的目标表
type MaintenanceTable struct {
	Database string `form:"database" json:"database" validate:"required"`
	Table    string `form:"table" json:"table" validate:"required"`
}

// MaintenanceRun 表维护操作请求
type MaintenanceRun struct {
	Tables    []MaintenanceTable `form:"tables" json:"tables"`
	Operation string             `form:"operation" json:"operation" validate:"required"`
}

// BinlogPurge binlog 清理请求
type BinlogPurge struct {
	File string `form:"file" json:"file" validate:"required"`
}

// Process 数据库进程信息
type Process struct {
	ID      int64  `json:"id"`
	User    string `json:"user"`
	Host    string `json:"host"`
	DB      string `json:"db"`
	Command string `json:"command"`
	Time    int64  `json:"time"`
	State   string `json:"state"`
	Info    string `json:"info"`
}

// Transaction InnoDB 事务信息
type Transaction struct {
	ID           string `json:"id"`
	ThreadID     int64  `json:"thread_id"`
	State        string `json:"state"`
	Query        string `json:"query"`
	Seconds      int64  `json:"seconds"`
	RowsLocked   int64  `json:"rows_locked"`
	RowsModified int64  `json:"rows_modified"`
}

// LockWait 锁等待对
type LockWait struct {
	WaitingThreadID  int64  `json:"waiting_thread_id"`
	WaitingQuery     string `json:"waiting_query"`
	BlockingThreadID int64  `json:"blocking_thread_id"`
	BlockingQuery    string `json:"blocking_query"`
}

// Transactions 事务与锁等待响应
type Transactions struct {
	Transactions []Transaction `json:"transactions"`
	LockWaits    []LockWait    `json:"lock_waits"`
}

// TopSQLItem Top SQL 统计项
type TopSQLItem struct {
	Database     string  `json:"database"`
	Calls        int64   `json:"calls"`
	TotalMs      int64   `json:"total_ms"`
	MeanMs       float64 `json:"mean_ms"`
	RowsSent     int64   `json:"rows_sent"`
	RowsExamined int64   `json:"rows_examined"`
	Query        string  `json:"query"`
}

// TopSQL Top SQL 响应
type TopSQL struct {
	Supported      bool         `json:"supported"`
	Enabled        bool         `json:"enabled"`
	PendingRestart bool         `json:"pending_restart"`
	Items          []TopSQLItem `json:"items"`
}

// TableInfo 表维护信息
type TableInfo struct {
	Database     string  `json:"database"`
	Table        string  `json:"table"`
	Engine       string  `json:"engine"`
	Rows         int64   `json:"rows"`
	Size         string  `json:"size"`
	SizeBytes    int64   `json:"size_bytes"`
	FragmentRate float64 `json:"fragment_rate"`
}

// BinlogFile binlog 文件信息
type BinlogFile struct {
	Name      string `json:"name"`
	Size      string `json:"size"`
	SizeBytes int64  `json:"size_bytes"`
}

// Binlog binlog 状态响应
type Binlog struct {
	Enabled   bool         `json:"enabled"`
	TotalSize string       `json:"total_size"`
	Items     []BinlogFile `json:"items"`
}

// Replication 复制状态响应
type Replication struct {
	Enabled       bool   `json:"enabled"`
	IORunning     string `json:"io_running"`
	SQLRunning    string `json:"sql_running"`
	SecondsBehind string `json:"seconds_behind"`
	SourceHost    string `json:"source_host"`
	LastError     string `json:"last_error"`
}
