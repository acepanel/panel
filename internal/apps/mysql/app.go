package mysql

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

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
	"github.com/acepanel/panel/pkg/types"
)

type App struct {
	t           *gotext.Locale
	settingRepo biz.SettingRepo
}

func NewApp(t *gotext.Locale, setting biz.SettingRepo) *App {
	return &App{
		t:           t,
		settingRepo: setting,
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
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))
		return

	}
	if len(rootPassword) == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("MySQL root password is empty"))
		return
	}

	status, _ := systemctl.Status("mysqld")
	if !status {
		service.Success(w, []types.NV{})
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
