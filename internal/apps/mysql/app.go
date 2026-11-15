package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
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

func NewApp(
	t *gotext.Locale,
	log *slog.Logger,
	databaseServerRepo biz.DatabaseServerRepo,
	settingRepo biz.SettingRepo,
	taskRepo biz.TaskRepo,
) *App {
	return New("mysql", t, log, databaseServerRepo, settingRepo, taskRepo)
}

func New(
	slug string,
	t *gotext.Locale,
	log *slog.Logger,
	databaseServerRepo biz.DatabaseServerRepo,
	settingRepo biz.SettingRepo,
	taskRepo biz.TaskRepo,
) *App {
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

	service.Success(w, s.loadItems(s.parseStatus(raw)))
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, s.slug, s.configPath(), 0644, common.Restart("mysqld"))
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
	config, err := io.Read(s.configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := new(ConfigTune)
	confval.SectionINI.Read(config, s.tuneFields(tune))

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新 MySQL 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := s.configPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = confval.SectionINI.Write(config, s.tuneFields(req))

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
		config, _ := io.Read(s.configPath())
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

	confPath := s.configPath()
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

// parseStatus 把 mysqladmin extended-status 的表格输出解析成变量名到值的映射
func (s *App) parseStatus(raw string) map[string]string {
	status := make(map[string]string)
	for line := range strings.SplitSeq(raw, "\n") {
		cols := strings.Split(line, "|")
		if len(cols) < 4 {
			continue
		}
		status[strings.TrimSpace(cols[1])] = strings.TrimSpace(cols[2])
	}

	return status
}

// loadItems 从状态变量整理出负载页的展示项，实例没有的变量不展示
func (s *App) loadItems(status map[string]string) []types.NV {
	// 命中率由多个变量算出，先并入 status 再统一按表取值
	status["Key_hit_rate"] = s.hitRate(status, "Key_reads", "Key_read_requests")
	status["Innodb_buffer_pool_hit_rate"] = s.hitRate(status, "Innodb_buffer_pool_reads", "Innodb_buffer_pool_read_requests")
	// MySQL 8.0+ 删除了查询缓存功能
	if hits, ok := status["Qcache_hits"]; ok {
		rate := 0.0
		if total := cast.ToFloat64(hits) + cast.ToFloat64(status["Qcache_inserts"]) + cast.ToFloat64(status["Qcache_not_cached"]); total > 0 {
			rate = cast.ToFloat64(hits) / total * 100
		}
		status["Qcache_hit_rate"] = fmt.Sprintf("%.2f%%", rate)
	}

	items := []struct {
		key   string
		name  string
		bytes bool
	}{
		{"Uptime", s.t.Get("Uptime"), false},
		{"Queries", s.t.Get("Total Queries"), false},
		{"Connections", s.t.Get("Total Connections"), false},
		{"Com_commit", s.t.Get("Transactions per Second"), false},
		{"Com_rollback", s.t.Get("Rollbacks per Second"), false},
		{"Bytes_sent", s.t.Get("Bytes Sent"), true},
		{"Bytes_received", s.t.Get("Bytes Received"), true},
		{"Threads_connected", s.t.Get("Active Connections"), false},
		{"Max_used_connections", s.t.Get("Peak Connections"), false},
		{"Key_hit_rate", s.t.Get("Index Hit Rate"), false},
		{"Innodb_buffer_pool_hit_rate", s.t.Get("Innodb Index Hit Rate"), false},
		{"Created_tmp_disk_tables", s.t.Get("Temporary Tables Created on Disk"), false},
		{"Open_tables", s.t.Get("Open Tables"), false},
		{"Select_full_join", s.t.Get("Full Joins without Index"), false},
		{"Select_full_range_join", s.t.Get("Full Range Joins without Index"), false},
		{"Select_range_check", s.t.Get("Subqueries without Index"), false},
		{"Sort_merge_passes", s.t.Get("Sort Merge Passes"), false},
		{"Table_locks_waited", s.t.Get("Table Locks Waited"), false},
		{"Qcache_hits", s.t.Get("Query Cache Hits"), false},
		{"Qcache_inserts", s.t.Get("Query Cache Inserts"), false},
		{"Qcache_not_cached", s.t.Get("Query Cache Not Cached"), false},
		{"Qcache_hit_rate", s.t.Get("Query Cache Hit Rate"), false},
	}

	load := make([]types.NV, 0, len(items))
	for _, item := range items {
		value, ok := status[item.key]
		if !ok {
			continue
		}
		if item.bytes {
			value = tools.FormatBytes(cast.ToFloat64(value))
		}
		load = append(load, types.NV{Name: item.name, Value: value})
	}

	return load
}

// hitRate 命中率 = 1 - 未命中 / 总请求，没有请求也就没有未命中，按 100% 计
func (s *App) hitRate(status map[string]string, missKey, totalKey string) string {
	total := cast.ToFloat64(status[totalKey])
	if total == 0 {
		return "100.00%"
	}

	return fmt.Sprintf("%.2f%%", (1-cast.ToFloat64(status[missKey])/total)*100)
}

func (s *App) configPath() string {
	return app.Root + "/server/mysql/conf/my.cnf"
}

// tuneFields 调优参数全部属于 [mysqld] 段，[client]、[mysqldump] 里的同名项不能动
func (s *App) tuneFields(t *ConfigTune) []confval.Field {
	return []confval.Field{
		// 常规设置
		{Section: "mysqld", Key: "port", Value: &t.Port},
		{Section: "mysqld", Key: "max_connections", Value: &t.MaxConnections},
		{Section: "mysqld", Key: "max_connect_errors", Value: &t.MaxConnectErrors},
		{Section: "mysqld", Key: "default_storage_engine", Value: &t.DefaultStorageEngine},
		{Section: "mysqld", Key: "table_open_cache", Value: &t.TableOpenCache},
		{Section: "mysqld", Key: "max_allowed_packet", Value: &t.MaxAllowedPacket},
		{Section: "mysqld", Key: "open_files_limit", Value: &t.OpenFilesLimit},
		// 性能调整
		{Section: "mysqld", Key: "key_buffer_size", Value: &t.KeyBufferSize},
		{Section: "mysqld", Key: "sort_buffer_size", Value: &t.SortBufferSize},
		{Section: "mysqld", Key: "read_buffer_size", Value: &t.ReadBufferSize},
		{Section: "mysqld", Key: "read_rnd_buffer_size", Value: &t.ReadRndBufferSize},
		{Section: "mysqld", Key: "join_buffer_size", Value: &t.JoinBufferSize},
		{Section: "mysqld", Key: "thread_cache_size", Value: &t.ThreadCacheSize},
		{Section: "mysqld", Key: "thread_stack", Value: &t.ThreadStack},
		{Section: "mysqld", Key: "tmp_table_size", Value: &t.TmpTableSize},
		{Section: "mysqld", Key: "max_heap_table_size", Value: &t.MaxHeapTableSize},
		{Section: "mysqld", Key: "myisam_sort_buffer_size", Value: &t.MyisamSortBufferSize},
		// InnoDB
		{Section: "mysqld", Key: "innodb_buffer_pool_size", Value: &t.InnodbBufferPoolSize},
		{Section: "mysqld", Key: "innodb_log_buffer_size", Value: &t.InnodbLogBufferSize},
		{Section: "mysqld", Key: "innodb_flush_log_at_trx_commit", Value: &t.InnodbFlushLogAtTrxCommit},
		{Section: "mysqld", Key: "innodb_lock_wait_timeout", Value: &t.InnodbLockWaitTimeout},
		{Section: "mysqld", Key: "innodb_max_dirty_pages_pct", Value: &t.InnodbMaxDirtyPagesPct},
		{Section: "mysqld", Key: "innodb_read_io_threads", Value: &t.InnodbReadIoThreads},
		{Section: "mysqld", Key: "innodb_write_io_threads", Value: &t.InnodbWriteIoThreads},
		// 日志
		{Section: "mysqld", Key: "slow_query_log", Value: &t.SlowQueryLog},
		{Section: "mysqld", Key: "long_query_time", Value: &t.LongQueryTime},
	}
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
