package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t                  *gotext.Locale
	log                *slog.Logger
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
	taskRepo           biz.TaskRepo
	slug               string
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo, taskRepo biz.TaskRepo) *App {
	return New("mysql", t, log, databaseServerRepo, settingRepo, taskRepo)
}

func New(slug string, t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo, taskRepo biz.TaskRepo) *App {
	return &App{
		t:                  t,
		log:                log,
		settingRepo:        settingRepo,
		databaseServerRepo: databaseServerRepo,
		taskRepo:           taskRepo,
		slug:               slug,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/slow_log", s.SlowLog)
	r.Get("/root_password", s.GetRootPassword)
	r.Post("/root_password", s.SetRootPassword)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	// 性能
	r.Get("/processes", s.ProcessList)
	r.Post("/processes/{id}/kill", s.KillProcess)
	r.Get("/transactions", s.TransactionList)
	r.Get("/top_sql", s.TopSQL)
	r.Post("/top_sql/enable", s.EnableTopSQL)
	r.Post("/top_sql/reset", s.ResetTopSQL)
	// 维护
	r.Get("/databases", s.DatabaseList)
	r.Get("/tables", s.TableList)
	r.Post("/maintenance", s.RunMaintenance)
	r.Get("/binlogs", s.BinlogList)
	r.Post("/binlogs/purge", s.PurgeBinlog)
	r.Get("/replication", s.ReplicationStatus)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "mysqld")
	return types.AggregateAppStatus(ok)
}

// Load 获取负载
func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, _ := systemctl.Status(r.Context(), "mysqld")
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))
		return
	}

	raw, err := shell.ExecfWithEnv(r.Context(), []string{"MYSQL_PWD=" + rootPassword}, `mysqladmin -u root extended-status`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get MySQL status: %v", err))
		return
	}

	var load []map[string]string
	expressions := []struct {
		regex string
		name  string
	}{
		{`Uptime\s+\|\s+(\d+)\s+\|`, s.t.Get("Uptime")},
		{`Queries\s+\|\s+(\d+)\s+\|`, s.t.Get("Total Queries")},
		{`Connections\s+\|\s+(\d+)\s+\|`, s.t.Get("Total Connections")},
		{`Com_commit\s+\|\s+(\d+)\s+\|`, s.t.Get("Transactions per Second")},
		{`Com_rollback\s+\|\s+(\d+)\s+\|`, s.t.Get("Rollbacks per Second")},
		{`Bytes_sent\s+\|\s+(\d+)\s+\|`, s.t.Get("Bytes Sent")},
		{`Bytes_received\s+\|\s+(\d+)\s+\|`, s.t.Get("Bytes Received")},
		{`Threads_connected\s+\|\s+(\d+)\s+\|`, s.t.Get("Active Connections")},
		{`Max_used_connections\s+\|\s+(\d+)\s+\|`, s.t.Get("Peak Connections")},
		{`Key_read_requests\s+\|\s+(\d+)\s+\|`, s.t.Get("Index Hit Rate")},
		{`Innodb_buffer_pool_reads\s+\|\s+(\d+)\s+\|`, s.t.Get("Innodb Index Hit Rate")},
		{`Created_tmp_disk_tables\s+\|\s+(\d+)\s+\|`, s.t.Get("Temporary Tables Created on Disk")},
		{`Open_tables\s+\|\s+(\d+)\s+\|`, s.t.Get("Open Tables")},
		{`Select_full_join\s+\|\s+(\d+)\s+\|`, s.t.Get("Full Joins without Index")},
		{`Select_full_range_join\s+\|\s+(\d+)\s+\|`, s.t.Get("Full Range Joins without Index")},
		{`Select_range_check\s+\|\s+(\d+)\s+\|`, s.t.Get("Subqueries without Index")},
		{`Sort_merge_passes\s+\|\s+(\d+)\s+\|`, s.t.Get("Sort Merge Passes")},
		{`Table_locks_waited\s+\|\s+(\d+)\s+\|`, s.t.Get("Table Locks Waited")},
	}

	for _, expression := range expressions {
		re := regexp.MustCompile(expression.regex)
		matches := re.FindStringSubmatch(raw)
		if len(matches) > 1 {
			d := map[string]string{"name": expression.name, "value": matches[1]}
			if expression.name == s.t.Get("Bytes Sent") || expression.name == s.t.Get("Bytes Received") {
				d["value"] = tools.FormatBytes(cast.ToFloat64(matches[1]))
			}

			load = append(load, d)
		}
	}

	// 索引命中率
	readRequests := cast.ToFloat64(load[9]["value"])
	reads := cast.ToFloat64(load[10]["value"])
	load[9]["value"] = fmt.Sprintf("%.2f%%", readRequests/(reads+readRequests)*100)
	// Innodb 索引命中率
	bufferPoolReads := cast.ToFloat64(load[11]["value"])
	bufferPoolReadRequests := cast.ToFloat64(load[12]["value"])
	load[10]["value"] = fmt.Sprintf("%.2f%%", bufferPoolReadRequests/(bufferPoolReads+bufferPoolReadRequests)*100)

	// 查询缓存命中率
	// MySQL 8.0+ 删除了查询缓存功能
	qcacheHitsRe := regexp.MustCompile(`Qcache_hits\s+\|\s+(\d+)\s+\|`)
	qcacheHitsMatches := qcacheHitsRe.FindStringSubmatch(raw)
	qcacheInsertsRe := regexp.MustCompile(`Qcache_inserts\s+\|\s+(\d+)\s+\|`)
	qcacheInsertsMatches := qcacheInsertsRe.FindStringSubmatch(raw)
	qcacheNotCachedRe := regexp.MustCompile(`Qcache_not_cached\s+\|\s+(\d+)\s+\|`)
	qcacheNotCachedMatches := qcacheNotCachedRe.FindStringSubmatch(raw)
	if len(qcacheHitsMatches) > 1 && len(qcacheInsertsMatches) > 1 && len(qcacheNotCachedMatches) > 1 {
		qcacheHits := cast.ToFloat64(qcacheHitsMatches[1])
		qcacheInserts := cast.ToFloat64(qcacheInsertsMatches[1])
		qcacheNotCached := cast.ToFloat64(qcacheNotCachedMatches[1])
		var qcacheHitRate float64
		denominator := qcacheHits + qcacheInserts + qcacheNotCached
		if denominator > 0 {
			qcacheHitRate = qcacheHits / denominator * 100
		}
		load = append(load, map[string]string{
			"name":  s.t.Get("Query Cache Hits"),
			"value": qcacheHitsMatches[1],
		})
		load = append(load, map[string]string{
			"name":  s.t.Get("Query Cache Inserts"),
			"value": qcacheInsertsMatches[1],
		})
		load = append(load, map[string]string{
			"name":  s.t.Get("Query Cache Not Cached"),
			"value": qcacheNotCachedMatches[1],
		})
		load = append(load, map[string]string{
			"name":  s.t.Get("Query Cache Hit Rate"),
			"value": fmt.Sprintf("%.2f%%", qcacheHitRate),
		})
	}

	service.Success(w, load)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", configPath()))

	service.Success(w, config)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(configPath(), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "mysqld"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to restart MySQL: %v", err))
		return
	}

	s.log.Info(s.t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", configPath()))

	service.Success(w, nil)
}

// SlowLog 获取慢查询日志
func (s *App) SlowLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, app.Root+"/server/mysql/mysql-slow.log")
}

// GetRootPassword 获取root密码
func (s *App) GetRootPassword(w http.ResponseWriter, r *http.Request) {
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))
		return
	}

	s.log.Info(s.t.Get("app password viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("username", "root"))

	service.Success(w, rootPassword)
}

// SetRootPassword 设置root密码
func (s *App) SetRootPassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SetRootPassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldRootPassword, _ := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	// 改密后还要写回面板设置，中途取消会让面板存的密码与实际不符
	ctx := context.WithoutCancel(r.Context())
	mysql, err := db.NewMySQL(r.Context(), "root", oldRootPassword, db.MySQLSocket(app.Root), "unix")
	if err != nil {
		// 尝试安全模式直接改密
		if err = db.MySQLResetRootPassword(ctx, req.Password, app.Root); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	} else {
		defer mysql.Close()
		if err = mysql.UserPassword(ctx, "root", req.Password, "localhost"); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = s.settingRepo.Set(biz.SettingKeyMySQLRootPassword, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	_ = s.databaseServerRepo.UpdatePassword("local_mysql", req.Password)

	s.log.Info(s.t.Get("app password updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("username", "root"))

	service.Success(w, nil)
}

// GetConfigTune 获取 MySQL 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		// 常规设置
		Port:                 confval.INI.Get(config, "port"),
		MaxConnections:       confval.INI.Get(config, "max_connections"),
		MaxConnectErrors:     confval.INI.Get(config, "max_connect_errors"),
		DefaultStorageEngine: confval.INI.Get(config, "default_storage_engine"),
		TableOpenCache:       confval.INI.Get(config, "table_open_cache"),
		MaxAllowedPacket:     confval.INI.Get(config, "max_allowed_packet"),
		OpenFilesLimit:       confval.INI.Get(config, "open_files_limit"),
		// 性能调整
		KeyBufferSize:        confval.INI.Get(config, "key_buffer_size"),
		SortBufferSize:       confval.INI.Get(config, "sort_buffer_size"),
		ReadBufferSize:       confval.INI.Get(config, "read_buffer_size"),
		ReadRndBufferSize:    confval.INI.Get(config, "read_rnd_buffer_size"),
		JoinBufferSize:       confval.INI.Get(config, "join_buffer_size"),
		ThreadCacheSize:      confval.INI.Get(config, "thread_cache_size"),
		ThreadStack:          confval.INI.Get(config, "thread_stack"),
		TmpTableSize:         confval.INI.Get(config, "tmp_table_size"),
		MaxHeapTableSize:     confval.INI.Get(config, "max_heap_table_size"),
		MyisamSortBufferSize: confval.INI.Get(config, "myisam_sort_buffer_size"),
		// InnoDB
		InnodbBufferPoolSize:      confval.INI.Get(config, "innodb_buffer_pool_size"),
		InnodbLogBufferSize:       confval.INI.Get(config, "innodb_log_buffer_size"),
		InnodbFlushLogAtTrxCommit: confval.INI.Get(config, "innodb_flush_log_at_trx_commit"),
		InnodbLockWaitTimeout:     confval.INI.Get(config, "innodb_lock_wait_timeout"),
		InnodbMaxDirtyPagesPct:    confval.INI.Get(config, "innodb_max_dirty_pages_pct"),
		InnodbReadIoThreads:       confval.INI.Get(config, "innodb_read_io_threads"),
		InnodbWriteIoThreads:      confval.INI.Get(config, "innodb_write_io_threads"),
		// 日志
		SlowQueryLog:  confval.INI.Get(config, "slow_query_log"),
		LongQueryTime: confval.INI.Get(config, "long_query_time"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 MySQL 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := configPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新常规设置
	config = confval.INI.Set(config, "port", req.Port)
	config = confval.INI.Set(config, "max_connections", req.MaxConnections)
	config = confval.INI.Set(config, "max_connect_errors", req.MaxConnectErrors)
	config = confval.INI.Set(config, "default_storage_engine", req.DefaultStorageEngine)
	config = confval.INI.Set(config, "table_open_cache", req.TableOpenCache)
	config = confval.INI.Set(config, "max_allowed_packet", req.MaxAllowedPacket)
	config = confval.INI.Set(config, "open_files_limit", req.OpenFilesLimit)
	// 更新性能调整
	config = confval.INI.Set(config, "key_buffer_size", req.KeyBufferSize)
	config = confval.INI.Set(config, "sort_buffer_size", req.SortBufferSize)
	config = confval.INI.Set(config, "read_buffer_size", req.ReadBufferSize)
	config = confval.INI.Set(config, "read_rnd_buffer_size", req.ReadRndBufferSize)
	config = confval.INI.Set(config, "join_buffer_size", req.JoinBufferSize)
	config = confval.INI.Set(config, "thread_cache_size", req.ThreadCacheSize)
	config = confval.INI.Set(config, "thread_stack", req.ThreadStack)
	config = confval.INI.Set(config, "tmp_table_size", req.TmpTableSize)
	config = confval.INI.Set(config, "max_heap_table_size", req.MaxHeapTableSize)
	config = confval.INI.Set(config, "myisam_sort_buffer_size", req.MyisamSortBufferSize)
	// 更新 InnoDB
	config = confval.INI.Set(config, "innodb_buffer_pool_size", req.InnodbBufferPoolSize)
	config = confval.INI.Set(config, "innodb_log_buffer_size", req.InnodbLogBufferSize)
	config = confval.INI.Set(config, "innodb_flush_log_at_trx_commit", req.InnodbFlushLogAtTrxCommit)
	config = confval.INI.Set(config, "innodb_lock_wait_timeout", req.InnodbLockWaitTimeout)
	config = confval.INI.Set(config, "innodb_max_dirty_pages_pct", req.InnodbMaxDirtyPagesPct)
	config = confval.INI.Set(config, "innodb_read_io_threads", req.InnodbReadIoThreads)
	config = confval.INI.Set(config, "innodb_write_io_threads", req.InnodbWriteIoThreads)
	// 更新日志
	config = confval.INI.Set(config, "slow_query_log", req.SlowQueryLog)
	config = confval.INI.Set(config, "long_query_time", req.LongQueryTime)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", confPath), slog.String("port", req.Port), slog.String("max_connections", req.MaxConnections), slog.String("innodb_buffer_pool_size", req.InnodbBufferPoolSize), slog.String("slow_query_log", req.SlowQueryLog))

	service.Success(w, nil)
}

// ProcessList 获取进程列表
func (s *App) ProcessList(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	rows, err := mysql.Query(r.Context(), `
		SELECT ID, coalesce(USER,''), coalesce(HOST,''), coalesce(DB,''), coalesce(COMMAND,''),
		       TIME, coalesce(STATE,''), coalesce(INFO,'')
		FROM information_schema.PROCESSLIST WHERE ID != CONNECTION_ID() ORDER BY TIME DESC`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	processes := make([]Process, 0)
	for rows.Next() {
		var item Process
		if err = rows.Scan(&item.ID, &item.User, &item.Host, &item.DB, &item.Command, &item.Time, &item.State, &item.Info); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		processes = append(processes, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mysql process list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug))

	service.Success(w, processes)
}

// KillProcess 终止进程
func (s *App) KillProcess(w http.ResponseWriter, r *http.Request) {
	id := cast.ToInt64(chi.URLParam(r, "id"))
	if id <= 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid process id"))
		return
	}

	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	var user, host, database string
	_ = mysql.QueryRow(r.Context(), `SELECT coalesce(USER,''), coalesce(HOST,''), coalesce(DB,'') FROM information_schema.PROCESSLIST WHERE ID = ?`, id).Scan(&user, &host, &database)

	// KILL 不支持预编译参数，id 已校验为正整数
	if _, err = mysql.Exec(r.Context(), fmt.Sprintf(`KILL %d`, id)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("mysql process killed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.Int64("process_id", id), slog.String("user", user), slog.String("host", host), slog.String("database", database))

	service.Success(w, nil)
}

// TransactionList 获取事务及锁等待列表
func (s *App) TransactionList(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	rows, err := mysql.Query(r.Context(), `
		SELECT trx_id, trx_mysql_thread_id, coalesce(trx_state,''), coalesce(trx_query,''),
		       timestampdiff(SECOND, trx_started, now()), trx_rows_locked, trx_rows_modified
		FROM information_schema.INNODB_TRX ORDER BY trx_started`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	result := Transactions{Transactions: make([]Transaction, 0), LockWaits: make([]LockWait, 0)}
	for rows.Next() {
		var item Transaction
		if err = rows.Scan(&item.ID, &item.ThreadID, &item.State, &item.Query, &item.Seconds, &item.RowsLocked, &item.RowsModified); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		result.Transactions = append(result.Transactions, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 锁等待对，MariaDB 与 MySQL 8 的表不同，查询失败时忽略
	lockSQL := `
		SELECT r.trx_mysql_thread_id, coalesce(r.trx_query,''), b.trx_mysql_thread_id, coalesce(b.trx_query,'')
		FROM performance_schema.data_lock_waits w
		JOIN information_schema.innodb_trx r ON r.trx_id = w.REQUESTING_ENGINE_TRANSACTION_ID
		JOIN information_schema.innodb_trx b ON b.trx_id = w.BLOCKING_ENGINE_TRANSACTION_ID`
	if s.isMariaDB(r.Context(), mysql) {
		lockSQL = `
		SELECT r.trx_mysql_thread_id, coalesce(r.trx_query,''), b.trx_mysql_thread_id, coalesce(b.trx_query,'')
		FROM information_schema.INNODB_LOCK_WAITS w
		JOIN information_schema.INNODB_TRX r ON r.trx_id = w.requesting_trx_id
		JOIN information_schema.INNODB_TRX b ON b.trx_id = w.blocking_trx_id`
	}
	if lockRows, lockErr := mysql.Query(r.Context(), lockSQL); lockErr == nil {
		defer func() { _ = lockRows.Close() }()
		for lockRows.Next() {
			var item LockWait
			if err = lockRows.Scan(&item.WaitingThreadID, &item.WaitingQuery, &item.BlockingThreadID, &item.BlockingQuery); err != nil {
				break
			}
			result.LockWaits = append(result.LockWaits, item)
		}
		if lockRows.Err() != nil {
			// 锁等待表结构不兼容，丢弃不完整结果
			result.LockWaits = nil
		}
	}

	s.log.Info(s.t.Get("mysql transaction list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug))

	service.Success(w, result)
}

// TopSQL 获取 SQL 性能统计
func (s *App) TopSQL(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	var enabled int
	if err = mysql.QueryRow(r.Context(), `SELECT @@performance_schema`).Scan(&enabled); err != nil {
		if strings.Contains(err.Error(), "Unknown system variable") {
			service.Success(w, TopSQL{Supported: false, Items: []TopSQLItem{}})
			return
		}
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if enabled == 0 {
		// 检查是否已配置等待重启
		config, _ := io.Read(configPath())
		pending := strings.EqualFold(confval.SectionINI.GetIn(config, "mysqld", "performance_schema"), "on")
		service.Success(w, TopSQL{Supported: true, Enabled: false, PendingRestart: pending, Items: []TopSQLItem{}})
		return
	}

	rows, err := mysql.Query(r.Context(), `
		SELECT coalesce(SCHEMA_NAME,''), COUNT_STAR, SUM_TIMER_WAIT DIV 1000000000,
		       round(AVG_TIMER_WAIT/1e9,2), SUM_ROWS_SENT, SUM_ROWS_EXAMINED, coalesce(DIGEST_TEXT,'')
		FROM performance_schema.events_statements_summary_by_digest
		WHERE DIGEST_TEXT IS NOT NULL ORDER BY SUM_TIMER_WAIT DESC LIMIT 50`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	items := make([]TopSQLItem, 0)
	for rows.Next() {
		var item TopSQLItem
		if err = rows.Scan(&item.Database, &item.Calls, &item.TotalMs, &item.MeanMs, &item.RowsSent, &item.RowsExamined, &item.Query); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app top sql viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug))

	service.Success(w, TopSQL{Supported: true, Enabled: true, Items: items})
}

// EnableTopSQL 启用 performance_schema
func (s *App) EnableTopSQL(w http.ResponseWriter, r *http.Request) {
	// 实例不支持时禁止写入配置
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()
	var enabled int
	if err = mysql.QueryRow(r.Context(), `SELECT @@performance_schema`).Scan(&enabled); err != nil {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("performance_schema is not supported by this instance"))
		return
	}

	confPath := configPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// my.cnf 分段，必须用 SectionINI 保证写入 [mysqld] 段内，重启后生效
	config = confval.SectionINI.SetIn(config, "mysqld", "performance_schema", "on")
	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app top sql enabled"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", confPath))

	service.Success(w, nil)
}

// ResetTopSQL 重置 SQL 性能统计
func (s *App) ResetTopSQL(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	if _, err = mysql.Exec(r.Context(), `TRUNCATE TABLE performance_schema.events_statements_summary_by_digest`); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app top sql reset"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("table", "performance_schema.events_statements_summary_by_digest"))

	service.Success(w, nil)
}

// DatabaseList 获取业务数据库列表
func (s *App) DatabaseList(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	rows, err := mysql.Query(r.Context(), `
		SELECT SCHEMA_NAME FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME NOT IN ('mysql','information_schema','performance_schema','sys')
		ORDER BY SCHEMA_NAME`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	databases := make([]string, 0)
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		databases = append(databases, name)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, databases)
}

// TableList 获取表维护信息
func (s *App) TableList(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[TableQuery](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	query := `
		SELECT table_schema, table_name, coalesce(engine,''), coalesce(table_rows,0),
		       coalesce(data_length + index_length,0), coalesce(data_free,0)
		FROM information_schema.TABLES
		WHERE table_schema NOT IN ('mysql','information_schema','performance_schema','sys')
		  AND table_type = 'BASE TABLE'`
	var args []any
	if req.Database != "" {
		query += ` AND table_schema = ?`
		args = append(args, req.Database)
	}
	query += ` ORDER BY data_length + index_length DESC LIMIT 50`
	rows, err := mysql.Query(r.Context(), query, args...)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	items := make([]TableInfo, 0)
	for rows.Next() {
		var item TableInfo
		var size, free int64
		if err = rows.Scan(&item.Database, &item.Table, &item.Engine, &item.Rows, &size, &free); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
		item.Size = tools.FormatBytes(float64(size))
		item.SizeBytes = size
		if size+free > 0 {
			item.FragmentRate = float64(free) * 100 / float64(size+free)
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app table list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("database", req.Database))

	service.Success(w, items)
}

// RunMaintenance 对表执行维护操作（异步任务）
func (s *App) RunMaintenance(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[MaintenanceRun](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if len(req.Tables) == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("no tables selected"))
		return
	}
	if !slices.Contains([]string{"optimize", "analyze"}, req.Operation) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid operation"))
		return
	}

	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tables := make([]string, 0, len(req.Tables))
	for _, table := range req.Tables {
		tables = append(tables, fmt.Sprintf("`%s`.`%s`", table.Database, table.Table))
	}
	escaped := strings.ReplaceAll(rootPassword, `'`, `'\''`)
	cmd := fmt.Sprintf("MYSQL_PWD='%s' mysql -u root -e '%s TABLE %s'", escaped, strings.ToUpper(req.Operation), strings.Join(tables, ", "))

	task := new(biz.Task)
	task.Key = "mysql:maintenance"
	task.Name = s.t.Get("Run %[1]s on %[2]d tables", req.Operation, len(req.Tables))
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app maintenance queued"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("operation", req.Operation), slog.Any("tables", tables), slog.Uint64("task_id", uint64(task.ID)))

	service.Success(w, nil)
}

// BinlogList 获取 binlog 状态
func (s *App) BinlogList(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	var enabled int
	if err = mysql.QueryRow(r.Context(), `SELECT @@log_bin`).Scan(&enabled); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if enabled == 0 {
		service.Success(w, Binlog{Enabled: false, Items: []BinlogFile{}})
		return
	}

	rows, err := mysql.Query(r.Context(), `SHOW BINARY LOGS`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// SHOW BINARY LOGS 列数两派不同（MySQL 8 多 Encrypted 列），动态取列
	maps, err := s.rowsToMaps(rows)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	binlog := Binlog{Enabled: true, Items: make([]BinlogFile, 0, len(maps))}
	var total int64
	for _, m := range maps {
		size := cast.ToInt64(m["File_size"])
		total += size
		binlog.Items = append(binlog.Items, BinlogFile{Name: m["Log_name"], Size: tools.FormatBytes(float64(size)), SizeBytes: size})
	}
	binlog.TotalSize = tools.FormatBytes(float64(total))

	s.log.Info(s.t.Get("mysql binlog list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug))

	service.Success(w, binlog)
}

// PurgeBinlog 清理指定文件之前的 binlog
func (s *App) PurgeBinlog(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[BinlogPurge](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	// 校验文件名存在于 binlog 列表，PURGE 不支持预编译参数
	rows, err := mysql.Query(r.Context(), `SHOW BINARY LOGS`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	maps, err := s.rowsToMaps(rows)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if !slices.ContainsFunc(maps, func(m map[string]string) bool { return m["Log_name"] == req.File }) {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("binlog file %s does not exist", req.File))
		return
	}

	if _, err = mysql.Exec(r.Context(), fmt.Sprintf(`PURGE BINARY LOGS TO '%s'`, req.File)); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("mysql binlog purged"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("to_file", req.File))

	service.Success(w, nil)
}

// ReplicationStatus 获取复制状态
func (s *App) ReplicationStatus(w http.ResponseWriter, r *http.Request) {
	mysql, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer mysql.Close()

	rows, err := mysql.Query(r.Context(), `SHOW REPLICA STATUS`)
	if err != nil {
		// 老版本 fallback
		if rows, err = mysql.Query(r.Context(), `SHOW SLAVE STATUS`); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	maps, err := s.rowsToMaps(rows)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if len(maps) == 0 {
		service.Success(w, Replication{Enabled: false})
		return
	}

	// MySQL 8 与 MariaDB 的列名两派不同，按候选名取值
	pick := func(m map[string]string, keys ...string) string {
		for _, key := range keys {
			if v, ok := m[key]; ok && v != "" {
				return v
			}
		}
		return ""
	}
	m := maps[0]
	s.log.Info(s.t.Get("mysql replication status viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug))

	service.Success(w, Replication{
		Enabled:       true,
		IORunning:     pick(m, "Replica_IO_Running", "Slave_IO_Running"),
		SQLRunning:    pick(m, "Replica_SQL_Running", "Slave_SQL_Running"),
		SecondsBehind: pick(m, "Seconds_Behind_Source", "Seconds_Behind_Master"),
		SourceHost:    pick(m, "Source_Host", "Master_Host"),
		LastError:     pick(m, "Last_Error"),
	})
}

func configPath() string {
	return app.Root + "/server/mysql/conf/my.cnf"
}

// connect 以 root 用户通过 unix socket 连接
func (s *App) connect(ctx context.Context) (db.Operator, error) {
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return nil, err
	}

	return db.NewMySQL(ctx, "root", rootPassword, db.MySQLSocket(app.Root), "unix")
}

// isMariaDB 判断当前实例是否为 MariaDB
func (s *App) isMariaDB(ctx context.Context, op db.Operator) bool {
	var version string
	if err := op.QueryRow(ctx, `SELECT VERSION()`).Scan(&version); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(version), "mariadb")
}

// rowsToMaps 将查询结果按列名转为 map，用于列名/列数不固定的查询
func (s *App) rowsToMaps(rows *sql.Rows) ([]map[string]string, error) {
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]string, 0)
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		scans := make([]any, len(columns))
		for i := range values {
			scans[i] = &values[i]
		}
		if err = rows.Scan(scans...); err != nil {
			return nil, err
		}
		m := make(map[string]string, len(columns))
		for i, column := range columns {
			m[column] = values[i].String
		}
		result = append(result, m)
	}

	return result, rows.Err()
}
