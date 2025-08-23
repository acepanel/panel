package mysql

import (
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/db"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/systemctl"
	"github.com/tnborg/panel/pkg/tools"
	"github.com/tnborg/panel/pkg/types"
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

func (s *App) Route(r fiber.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Post("/clear_error_log", s.ClearErrorLog)
	r.Get("/slow_log", s.SlowLog)
	r.Post("/clear_slow_log", s.ClearSlowLog)
	r.Get("/root_password", s.GetRootPassword)
	r.Post("/root_password", s.SetRootPassword)
}

// GetConfig 获取配置
func (s *App) GetConfig(c fiber.Ctx) error {
	config, err := io.Read(app.Root + "/server/mysql/conf/my.cnf")
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(app.Root+"/server/mysql/conf/my.cnf", req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart("mysqld"); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to restart MySQL: %v", err))
	}

	return service.Success(c, nil)
}

// Load 获取负载
func (s *App) Load(c fiber.Ctx) error {
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))

	}
	if len(rootPassword) == 0 {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("MySQL root password is empty"))
	}

	status, _ := systemctl.Status("mysqld")
	if !status {
		return service.Success(c, []types.NV{})
	}

	if err = os.Setenv("MYSQL_PWD", rootPassword); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to set MYSQL_PWD env: %v", err))
	}
	raw, err := shell.Execf(`mysqladmin -u root extended-status`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get MySQL status: %v", err))
	}
	if err = os.Unsetenv("MYSQL_PWD"); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to unset MYSQL_PWD env: %v", err))
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

	return service.Success(c, load)
}

// ClearErrorLog 清空错误日志
func (s *App) ClearErrorLog(c fiber.Ctx) error {
	if err := systemctl.LogClear("mysqld"); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// SlowLog 获取慢查询日志
func (s *App) SlowLog(c fiber.Ctx) error {
	return service.Success(c, fmt.Sprintf("%s/server/mysql/mysql-slow.log", app.Root))
}

// ClearSlowLog 清空慢查询日志
func (s *App) ClearSlowLog(c fiber.Ctx) error {
	if _, err := shell.Execf("cat /dev/null > %s/server/mysql/mysql-slow.log", app.Root); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// GetRootPassword 获取root密码
func (s *App) GetRootPassword(c fiber.Ctx) error {
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to load MySQL root password: %v", err))
	}

	return service.Success(c, rootPassword)
}

// SetRootPassword 设置root密码
func (s *App) SetRootPassword(c fiber.Ctx) error {
	req, err := service.Bind[SetRootPassword](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	oldRootPassword, _ := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	mysql, err := db.NewMySQL("root", oldRootPassword, s.getSock(), "unix")
	if err != nil {
		// 尝试安全模式直接改密
		if err = db.MySQLResetRootPassword(req.Password); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	} else {
		defer func(mysql *db.MySQL) {
			_ = mysql.Close()
		}(mysql)
		if err = mysql.UserPassword("root", req.Password, "localhost"); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	}
	if err = s.settingRepo.Set(biz.SettingKeyMySQLRootPassword, req.Password); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
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
