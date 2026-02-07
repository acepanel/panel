package mysql

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/acepanel/panel/pkg/types"
	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/service"
	"github.com/acepanel/panel/pkg/db"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/systemctl"
	"github.com/acepanel/panel/pkg/tools"
)

type App struct {
	t                  *gotext.Locale
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
}

func NewApp(t *gotext.Locale, setting biz.SettingRepo, databaseServer biz.DatabaseServerRepo) *App {
	return &App{
		t:                  t,
		settingRepo:        setting,
		databaseServerRepo: databaseServer,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Post("/clear_log", s.ClearLog)
	r.Get("/slow_log", s.SlowLog)
	r.Post("/clear_slow_log", s.ClearSlowLog)
	r.Get("/root_password", s.GetRootPassword)
	r.Post("/root_password", s.SetRootPassword)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(app.Root + "/server/mysql/conf/my.cnf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(app.Root+"/server/mysql/conf/my.cnf", req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("mysqld"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to restart MySQL: %v", err))
		return
	}

	service.Success(w, nil)
}

// Load 获取负载
func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, _ := systemctl.Status("mysqld")
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))
		return
	}

	if err = os.Setenv("MYSQL_PWD", rootPassword); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set MYSQL_PWD env: %v", err))
		return
	}
	raw, err := shell.Execf(`mysqladmin -u root extended-status`)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get MySQL status: %v", err))
		return
	}
	if err = os.Unsetenv("MYSQL_PWD"); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to unset MYSQL_PWD env: %v", err))
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

// ClearLog 清空日志
func (s *App) ClearLog(w http.ResponseWriter, r *http.Request) {
	if err := systemctl.LogClear("mysqld"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// SlowLog 获取慢查询日志
func (s *App) SlowLog(w http.ResponseWriter, r *http.Request) {
	service.Success(w, fmt.Sprintf("%s/server/mysql/mysql-slow.log", app.Root))
}

// ClearSlowLog 清空慢查询日志
func (s *App) ClearSlowLog(w http.ResponseWriter, r *http.Request) {
	if _, err := shell.Execf("cat /dev/null > %s/server/mysql/mysql-slow.log", app.Root); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// GetRootPassword 获取root密码
func (s *App) GetRootPassword(w http.ResponseWriter, r *http.Request) {
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))
		return
	}

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
	mysql, err := db.NewMySQL("root", oldRootPassword, s.getSock(), "unix")
	if err != nil {
		// 尝试安全模式直接改密
		if err = db.MySQLResetRootPassword(req.Password); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	} else {
		defer mysql.Close()
		if err = mysql.UserPassword("root", req.Password, "localhost"); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = s.settingRepo.Set(biz.SettingKeyMySQLRootPassword, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	_ = s.databaseServerRepo.UpdatePassword("local_mysql", req.Password)

	service.Success(w, nil)
}

func (s *App) getSock() string {
	if io.Exists("/tmp/mysql.sock") {
		return "/tmp/mysql.sock"
	}
	if io.Exists(app.Root + "/server/mysql/config/my.cnf") {
		config, _ := io.Read(app.Root + "/server/mysql/config/my.cnf")
		re := regexp.MustCompile(`socket\s*=\s*(['"]?)([^'"]+)`)
		matches := re.FindStringSubmatch(config)
		if len(matches) > 2 {
			return matches[2]
		}
	}
	if io.Exists("/etc/my.cnf") {
		config, _ := io.Read("/etc/my.cnf")
		re := regexp.MustCompile(`socket\s*=\s*(['"]?)([^'"]+)`)
		matches := re.FindStringSubmatch(config)
		if len(matches) > 2 {
			return matches[2]
		}
	}

	return "/tmp/mysql.sock"
}

// GetConfigTune 获取 MySQL 配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(app.Root + "/server/mysql/conf/my.cnf")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		// 常规设置
		Port:                 s.getINIValue(config, "port"),
		MaxConnections:       s.getINIValue(config, "max_connections"),
		MaxConnectErrors:     s.getINIValue(config, "max_connect_errors"),
		DefaultStorageEngine: s.getINIValue(config, "default_storage_engine"),
		TableOpenCache:       s.getINIValue(config, "table_open_cache"),
		MaxAllowedPacket:     s.getINIValue(config, "max_allowed_packet"),
		OpenFilesLimit:       s.getINIValue(config, "open_files_limit"),
		// 性能调整
		KeyBufferSize:        s.getINIValue(config, "key_buffer_size"),
		SortBufferSize:       s.getINIValue(config, "sort_buffer_size"),
		ReadBufferSize:       s.getINIValue(config, "read_buffer_size"),
		ReadRndBufferSize:    s.getINIValue(config, "read_rnd_buffer_size"),
		JoinBufferSize:       s.getINIValue(config, "join_buffer_size"),
		ThreadCacheSize:      s.getINIValue(config, "thread_cache_size"),
		ThreadStack:          s.getINIValue(config, "thread_stack"),
		TmpTableSize:         s.getINIValue(config, "tmp_table_size"),
		MaxHeapTableSize:     s.getINIValue(config, "max_heap_table_size"),
		MyisamSortBufferSize: s.getINIValue(config, "myisam_sort_buffer_size"),
		// InnoDB
		InnodbBufferPoolSize:      s.getINIValue(config, "innodb_buffer_pool_size"),
		InnodbLogBufferSize:       s.getINIValue(config, "innodb_log_buffer_size"),
		InnodbFlushLogAtTrxCommit: s.getINIValue(config, "innodb_flush_log_at_trx_commit"),
		InnodbLockWaitTimeout:     s.getINIValue(config, "innodb_lock_wait_timeout"),
		InnodbMaxDirtyPagesPct:    s.getINIValue(config, "innodb_max_dirty_pages_pct"),
		InnodbReadIoThreads:       s.getINIValue(config, "innodb_read_io_threads"),
		InnodbWriteIoThreads:      s.getINIValue(config, "innodb_write_io_threads"),
		// 日志
		SlowQueryLog:  s.getINIValue(config, "slow_query_log"),
		LongQueryTime: s.getINIValue(config, "long_query_time"),
	}

	service.Success(w, tune)
}

// UpdateConfigTune 更新 MySQL 配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := app.Root + "/server/mysql/conf/my.cnf"
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 更新常规设置
	config = s.setINIValue(config, "port", req.Port)
	config = s.setINIValue(config, "max_connections", req.MaxConnections)
	config = s.setINIValue(config, "max_connect_errors", req.MaxConnectErrors)
	config = s.setINIValue(config, "default_storage_engine", req.DefaultStorageEngine)
	config = s.setINIValue(config, "table_open_cache", req.TableOpenCache)
	config = s.setINIValue(config, "max_allowed_packet", req.MaxAllowedPacket)
	config = s.setINIValue(config, "open_files_limit", req.OpenFilesLimit)
	// 更新性能调整
	config = s.setINIValue(config, "key_buffer_size", req.KeyBufferSize)
	config = s.setINIValue(config, "sort_buffer_size", req.SortBufferSize)
	config = s.setINIValue(config, "read_buffer_size", req.ReadBufferSize)
	config = s.setINIValue(config, "read_rnd_buffer_size", req.ReadRndBufferSize)
	config = s.setINIValue(config, "join_buffer_size", req.JoinBufferSize)
	config = s.setINIValue(config, "thread_cache_size", req.ThreadCacheSize)
	config = s.setINIValue(config, "thread_stack", req.ThreadStack)
	config = s.setINIValue(config, "tmp_table_size", req.TmpTableSize)
	config = s.setINIValue(config, "max_heap_table_size", req.MaxHeapTableSize)
	config = s.setINIValue(config, "myisam_sort_buffer_size", req.MyisamSortBufferSize)
	// 更新 InnoDB
	config = s.setINIValue(config, "innodb_buffer_pool_size", req.InnodbBufferPoolSize)
	config = s.setINIValue(config, "innodb_log_buffer_size", req.InnodbLogBufferSize)
	config = s.setINIValue(config, "innodb_flush_log_at_trx_commit", req.InnodbFlushLogAtTrxCommit)
	config = s.setINIValue(config, "innodb_lock_wait_timeout", req.InnodbLockWaitTimeout)
	config = s.setINIValue(config, "innodb_max_dirty_pages_pct", req.InnodbMaxDirtyPagesPct)
	config = s.setINIValue(config, "innodb_read_io_threads", req.InnodbReadIoThreads)
	config = s.setINIValue(config, "innodb_write_io_threads", req.InnodbWriteIoThreads)
	// 更新日志
	config = s.setINIValue(config, "slow_query_log", req.SlowQueryLog)
	config = s.setINIValue(config, "long_query_time", req.LongQueryTime)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}

// getINIValue 从 INI 格式内容中获取指定键的值
func (s *App) getINIValue(content string, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

// setINIValue 在 INI 格式内容中设置指定键的值
func (s *App) setINIValue(content string, key string, value string) string {
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	lines := strings.Split(content, "\n")
	found := false
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "[") {
			result = append(result, line)
			continue
		}
		checkLine := trimmed
		if strings.HasPrefix(checkLine, ";") {
			checkLine = strings.TrimSpace(checkLine[1:])
		} else if strings.HasPrefix(checkLine, "#") {
			checkLine = strings.TrimSpace(checkLine[1:])
		}
		parts := strings.SplitN(checkLine, "=", 2)
		if len(parts) != 2 {
			result = append(result, line)
			continue
		}
		k := strings.TrimSpace(parts[0])
		if k == key {
			if found {
				continue
			}
			found = true
			// 值为空时注释掉该配置项
			if value == "" {
				if !strings.HasPrefix(trimmed, ";") && !strings.HasPrefix(trimmed, "#") {
					result = append(result, "#"+line)
				} else {
					result = append(result, line)
				}
				continue
			}
			result = append(result, key+" = "+value)
		} else {
			result = append(result, line)
		}
	}
	if !found && value != "" {
		result = append(result, key+" = "+value)
	}
	return strings.Join(result, "\n")
}
