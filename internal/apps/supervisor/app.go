package supervisor

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/do/v2"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t    *gotext.Locale
	name string
}

func NewApp(i do.Injector) (*App, error) {
	t := do.MustInvoke[*gotext.Locale](i)
	var name string
	if os.IsRHEL() {
		name = "supervisord"
	} else {
		name = "supervisor"
	}

	return &App{
		t:    t,
		name: name,
	}, nil
}

func (s *App) Route(r chi.Router) {
	r.Get("/service", s.Service)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/processes", s.Processes)
	r.Post("/processes/{process}/start", s.StartProcess)
	r.Post("/processes/{process}/stop", s.StopProcess)
	r.Post("/processes/{process}/restart", s.RestartProcess)
	r.Get("/processes/{process}/log", s.ProcessLog)
	r.Get("/processes/{process}", s.ProcessConfig)
	r.Post("/processes/{process}", s.UpdateProcessConfig)
	r.Delete("/processes/{process}", s.DeleteProcess)
	r.Post("/processes", s.CreateProcess)
}

// Service 获取服务名称
func (s *App) Service(w http.ResponseWriter, r *http.Request) {
	service.Success(w, s.name)
}

func (s *App) Status() string {
	ok, _ := systemctl.Status(s.name)
	return types.AggregateAppStatus(ok)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	var config string
	var err error
	if os.IsRHEL() {
		config, err = io.Read(`/etc/supervisord.conf`)
	} else {
		config, err = io.Read(`/etc/supervisor/supervisord.conf`)
	}

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

	if os.IsRHEL() {
		err = io.Write(`/etc/supervisord.conf`, req.Config, 0644)
	} else {
		err = io.Write(`/etc/supervisor/supervisord.conf`, req.Config, 0644)
	}

	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(s.name); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to restart %s: %v", s.name, err))
		return
	}

	service.Success(w, nil)
}

// Processes 进程列表
func (s *App) Processes(w http.ResponseWriter, r *http.Request) {
	out, err := shell.Execf(`supervisorctl status`)
	if err != nil && out == "" {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var processes []Process
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		p := Process{
			Name:   fields[0],
			Status: fields[1],
			Pid:    "-",
			Uptime: "-",
		}
		// RUNNING 行格式：name RUNNING pid 1234, uptime 1:23:45 （超过 1 天为 "1 day, 1:23:45"）
		if p.Status == "RUNNING" {
			if _, rest, ok := strings.Cut(line, "pid "); ok {
				if pid, _, ok := strings.Cut(rest, ","); ok {
					p.Pid = pid
				}
			}
			if _, uptime, ok := strings.Cut(line, "uptime "); ok {
				p.Uptime = uptime
			}
		}
		processes = append(processes, p)
	}

	paged, total := service.Paginate(r, processes)

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// StartProcess 启动进程
func (s *App) StartProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if out, err := shell.Execf(`supervisorctl start '%s'`, req.Process); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v %s", err, out)
		return
	}

	service.Success(w, nil)
}

// StopProcess 停止进程
func (s *App) StopProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if out, err := shell.Execf(`supervisorctl stop '%s'`, req.Process); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v %s", err, out)
		return
	}

	service.Success(w, nil)
}

// RestartProcess 重启进程
func (s *App) RestartProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if out, err := shell.Execf(`supervisorctl restart '%s'`, req.Process); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v %s", err, out)
		return
	}

	service.Success(w, nil)
}

// ProcessLog 进程日志
func (s *App) ProcessLog(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	var logPath string
	if os.IsRHEL() {
		logPath, err = shell.Execf(`cat '/etc/supervisord.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, name)
	} else {
		logPath, err = shell.Execf(`cat '/etc/supervisor/conf.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, name)
	}

	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
		return
	}

	service.Success(w, logPath)
}

// ProcessConfig 获取进程配置
func (s *App) ProcessConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	var config string
	if os.IsRHEL() {
		config, err = io.Read(`/etc/supervisord.d/` + name + `.conf`)
	} else {
		config, err = io.Read(`/etc/supervisor/conf.d/` + name + `.conf`)
	}

	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

// UpdateProcessConfig 保存进程配置
func (s *App) UpdateProcessConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateProcessConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	if os.IsRHEL() {
		err = io.Write(`/etc/supervisord.d/`+name+`.conf`, req.Config, 0644)
	} else {
		err = io.Write(`/etc/supervisor/conf.d/`+name+`.conf`, req.Config, 0644)
	}

	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	_, _ = shell.Execf(`supervisorctl reread`)
	_, _ = shell.Execf(`supervisorctl update`)
	_, _ = shell.Execf(`supervisorctl restart '%s:'`, name)

	service.Success(w, nil)
}

// CreateProcess 添加进程
func (s *App) CreateProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[CreateProcess](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	processName := `%(program_name)s`
	if req.Num > 1 {
		processName = `%(program_name)s_%(process_num)02d`
	}

	config := `[program:` + req.Name + `]
command=` + req.Command + `
process_name=` + processName + `
directory=` + req.Path + `
autostart=true
autorestart=true
user=` + req.User + `
numprocs=` + cast.ToString(req.Num) + `
redirect_stderr=true
stdout_logfile=/var/log/supervisor/` + req.Name + `.log
stdout_logfile_maxbytes=2MB
`

	var confPath string
	if os.IsRHEL() {
		confPath = `/etc/supervisord.d/` + req.Name + `.conf`
	} else {
		confPath = `/etc/supervisor/conf.d/` + req.Name + `.conf`
	}

	if io.Exists(confPath) {
		service.Error(w, http.StatusConflict, s.t.Get("process %s already exists", req.Name))
		return
	}

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	_, _ = shell.Execf(`supervisorctl reread`)
	_, _ = shell.Execf(`supervisorctl update`)
	_, _ = shell.Execf(`supervisorctl start '%s:'`, req.Name)

	service.Success(w, nil)
}

// DeleteProcess 删除进程
func (s *App) DeleteProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	if out, err := shell.Execf(`supervisorctl stop '%s:'`, name); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v %s", err, out)
		return
	}

	var logPath string
	if os.IsRHEL() {
		logPath, err = shell.Execf(`cat '/etc/supervisord.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, name)
		if err != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
			return
		}
		if err = io.Remove(`/etc/supervisord.d/` + name + `.conf`); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	} else {
		logPath, err = shell.Execf(`cat '/etc/supervisor/conf.d/%s.conf' | grep stdout_logfile= | awk -F "=" '{print $2}'`, name)
		if err != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
			return
		}
		if err = io.Remove(`/etc/supervisor/conf.d/` + name + `.conf`); err != nil {
			service.Error(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}

	if err = io.Remove(logPath); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	_, _ = shell.Execf(`supervisorctl reread`)
	_, _ = shell.Execf(`supervisorctl update`)

	service.Success(w, nil)
}

func programName(process string) string {
	name, _, _ := strings.Cut(process, ":")
	return name
}
