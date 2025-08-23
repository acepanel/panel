package supervisor

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/os"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/systemctl"
)

type App struct {
	t    *gotext.Locale
	name string
}

func NewApp(t *gotext.Locale) *App {
	var name string
	if os.IsRHEL() {
		name = "supervisord"
	} else {
		name = "supervisor"
	}

	return &App{
		t:    t,
		name: name,
	}
}

func (s *App) Route(r fiber.Router) {
	r.Get("/service", s.Service)
	r.Post("/clear_log", s.ClearLog)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/processes", s.Processes)
	r.Post("/processes/{process}/start", s.StartProcess)
	r.Post("/processes/{process}/stop", s.StopProcess)
	r.Post("/processes/{process}/restart", s.RestartProcess)
	r.Get("/processes/{process}/log", s.ProcessLog)
	r.Post("/processes/{process}/clear_log", s.ClearProcessLog)
	r.Get("/processes/{process}", s.ProcessConfig)
	r.Post("/processes/{process}", s.UpdateProcessConfig)
	r.Delete("/processes/{process}", s.DeleteProcess)
	r.Post("/processes", s.CreateProcess)
}

// Service 获取服务名称
func (s *App) Service(c fiber.Ctx) error {
	return service.Success(c, s.name)
}

// ClearLog 清空日志
func (s *App) ClearLog(c fiber.Ctx) error {
	if _, err := shell.Execf(`cat /dev/null > /var/log/supervisor/supervisord.log`); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// GetConfig 获取配置
func (s *App) GetConfig(c fiber.Ctx) error {
	var config string
	var err error
	if os.IsRHEL() {
		config, err = io.Read(`/etc/supervisord.conf`)
	} else {
		config, err = io.Read(`/etc/supervisor/supervisord.conf`)
	}

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

	if os.IsRHEL() {
		err = io.Write(`/etc/supervisord.conf`, req.Config, 0644)
	} else {
		err = io.Write(`/etc/supervisor/supervisord.conf`, req.Config, 0644)
	}

	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	if err = systemctl.Restart(s.name); err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to restart %s: %v", s.name, err))
	}

	return service.Success(c, nil)
}

// Processes 进程列表
func (s *App) Processes(c fiber.Ctx) error {
	out, err := shell.Execf(`supervisorctl status | awk '{print $1}'`)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	var processes []Process
	for _, line := range strings.Split(out, "\n") {
		if len(line) == 0 {
			continue
		}

		var p Process
		p.Name = line
		if status, err := shell.Execf(`supervisorctl status '%s' | awk '{print $2}'`, line); err == nil {
			p.Status = status
		}
		if p.Status == "RUNNING" {
			pid, _ := shell.Execf(`supervisorctl status '%s' | awk '{print $4}'`, line)
			p.Pid = strings.ReplaceAll(pid, ",", "")
			uptime, _ := shell.Execf(`supervisorctl status '%s' | awk '{print $6}'`, line)
			p.Uptime = uptime
		} else {
			p.Pid = "-"
			p.Uptime = "-"
		}
		processes = append(processes, p)
	}

	paged, total := service.Paginate(c, processes)

	return service.Success(c, chix.M{
		"total": total,
		"items": paged,
	})
}

// StartProcess 启动进程
func (s *App) StartProcess(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if out, err := shell.Execf(`supervisorctl start %s`, req.Process); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v %s", err, out)
	}

	return service.Success(c, nil)
}

// StopProcess 停止进程
func (s *App) StopProcess(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if out, err := shell.Execf(`supervisorctl stop %s`, req.Process); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v %s", err, out)
	}

	return service.Success(c, nil)
}

// RestartProcess 重启进程
func (s *App) RestartProcess(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if out, err := shell.Execf(`supervisorctl restart %s`, req.Process); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v %s", err, out)
	}

	return service.Success(c, nil)
}

// ProcessLog 进程日志
func (s *App) ProcessLog(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	var logPath string
	if os.IsRHEL() {
		logPath, err = shell.Execf(`cat '/etc/supervisord.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, req.Process)
	} else {
		logPath, err = shell.Execf(`cat '/etc/supervisor/conf.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, req.Process)
	}

	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
	}

	return service.Success(c, logPath)
}

// ClearProcessLog 清空进程日志
func (s *App) ClearProcessLog(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	var logPath string
	if os.IsRHEL() {
		logPath, err = shell.Execf(`cat '/etc/supervisord.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, req.Process)
	} else {
		logPath, err = shell.Execf(`cat '/etc/supervisor/conf.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, req.Process)
	}

	if err != nil {
		return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
	}

	if _, err = shell.Execf(`cat /dev/null > '%s'`, logPath); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

// ProcessConfig 获取进程配置
func (s *App) ProcessConfig(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	var config string
	if os.IsRHEL() {
		config, err = io.Read(`/etc/supervisord.d/` + req.Process + `.conf`)
	} else {
		config, err = io.Read(`/etc/supervisor/conf.d/` + req.Process + `.conf`)
	}

	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

// UpdateProcessConfig 保存进程配置
func (s *App) UpdateProcessConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateProcessConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if os.IsRHEL() {
		err = io.Write(`/etc/supervisord.d/`+req.Process+`.conf`, req.Config, 0644)
	} else {
		err = io.Write(`/etc/supervisor/conf.d/`+req.Process+`.conf`, req.Config, 0644)
	}

	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	_, _ = shell.Execf(`supervisorctl reread`)
	_, _ = shell.Execf(`supervisorctl update`)
	_, _ = shell.Execf(`supervisorctl restart '%s'`, req.Process)

	return service.Success(c, nil)
}

// CreateProcess 添加进程
func (s *App) CreateProcess(c fiber.Ctx) error {
	req, err := service.Bind[CreateProcess](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	config := `[program:` + req.Name + `]
command=` + req.Command + `
process_name=%(program_name)s
directory=` + req.Path + `
autostart=true
autorestart=true
user=` + req.User + `
numprocs=` + cast.ToString(req.Num) + `
redirect_stderr=true
stdout_logfile=/var/log/supervisor/` + req.Name + `.log
stdout_logfile_maxbytes=2MB
`

	if os.IsRHEL() {
		err = io.Write(`/etc/supervisord.d/`+req.Name+`.conf`, config, 0644)
	} else {
		err = io.Write(`/etc/supervisor/conf.d/`+req.Name+`.conf`, config, 0644)
	}

	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	_, _ = shell.Execf(`supervisorctl reread`)
	_, _ = shell.Execf(`supervisorctl update`)
	_, _ = shell.Execf(`supervisorctl start '%s'`, req.Name)

	return service.Success(c, nil)
}

// DeleteProcess 删除进程
func (s *App) DeleteProcess(c fiber.Ctx) error {
	req, err := service.Bind[ProcessName](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if out, err := shell.Execf(`supervisorctl stop '%s'`, req.Process); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v %s", err, out)
	}

	var logPath string
	if os.IsRHEL() {
		logPath, err = shell.Execf(`cat '/etc/supervisord.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, req.Process)
		if err != nil {
			return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
		}
		if err = io.Remove(`/etc/supervisord.d/` + req.Process + `.conf`); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	} else {
		logPath, err = shell.Execf(`cat '/etc/supervisor/conf.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, req.Process)
		if err != nil {
			return service.Error(c, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
		}
		if err = io.Remove(`/etc/supervisor/conf.d/` + req.Process + `.conf`); err != nil {
			return service.Error(c, http.StatusInternalServerError, "%v", err)
		}
	}

	if err = io.Remove(logPath); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}
	_, _ = shell.Execf(`supervisorctl reread`)
	_, _ = shell.Execf(`supervisorctl update`)

	return service.Success(c, nil)
}
