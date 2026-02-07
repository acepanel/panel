package mysql

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

type SetRootPassword struct {
	Password string `form:"password" json:"password" validate:"required|password"`
}

// ConfigTune MySQL 配置调整
type ConfigTune struct {
	// 常规设置
	Port               string `form:"port" json:"port"`
	MaxConnections     string `form:"max_connections" json:"max_connections"`
	MaxConnectErrors   string `form:"max_connect_errors" json:"max_connect_errors"`
	DefaultStorageEngine string `form:"default_storage_engine" json:"default_storage_engine"`
	TableOpenCache     string `form:"table_open_cache" json:"table_open_cache"`
	MaxAllowedPacket   string `form:"max_allowed_packet" json:"max_allowed_packet"`
	OpenFilesLimit     string `form:"open_files_limit" json:"open_files_limit"`
	// 性能调整
	KeyBufferSize          string `form:"key_buffer_size" json:"key_buffer_size"`
	SortBufferSize         string `form:"sort_buffer_size" json:"sort_buffer_size"`
	ReadBufferSize         string `form:"read_buffer_size" json:"read_buffer_size"`
	ReadRndBufferSize      string `form:"read_rnd_buffer_size" json:"read_rnd_buffer_size"`
	JoinBufferSize         string `form:"join_buffer_size" json:"join_buffer_size"`
	ThreadCacheSize        string `form:"thread_cache_size" json:"thread_cache_size"`
	ThreadStack            string `form:"thread_stack" json:"thread_stack"`
	TmpTableSize           string `form:"tmp_table_size" json:"tmp_table_size"`
	MaxHeapTableSize       string `form:"max_heap_table_size" json:"max_heap_table_size"`
	MyisamSortBufferSize   string `form:"myisam_sort_buffer_size" json:"myisam_sort_buffer_size"`
	// InnoDB
	InnodbBufferPoolSize       string `form:"innodb_buffer_pool_size" json:"innodb_buffer_pool_size"`
	InnodbLogBufferSize        string `form:"innodb_log_buffer_size" json:"innodb_log_buffer_size"`
	InnodbFlushLogAtTrxCommit  string `form:"innodb_flush_log_at_trx_commit" json:"innodb_flush_log_at_trx_commit"`
	InnodbLockWaitTimeout      string `form:"innodb_lock_wait_timeout" json:"innodb_lock_wait_timeout"`
	InnodbMaxDirtyPagesPct     string `form:"innodb_max_dirty_pages_pct" json:"innodb_max_dirty_pages_pct"`
	InnodbReadIoThreads        string `form:"innodb_read_io_threads" json:"innodb_read_io_threads"`
	InnodbWriteIoThreads       string `form:"innodb_write_io_threads" json:"innodb_write_io_threads"`
	// 日志
	SlowQueryLog  string `form:"slow_query_log" json:"slow_query_log"`
	LongQueryTime string `form:"long_query_time" json:"long_query_time"`
}
