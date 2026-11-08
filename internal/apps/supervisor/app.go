package supervisor

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix/v2"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t    *gotext.Locale
	log  *slog.Logger
	name string
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		t:    t,
		log:  log,
		name: lo.Ternary(os.IsRHEL(), "supervisord", "supervisor"),
	}
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
	r.Get("/processes/{process}/setting", s.GetProcessSetting)
	r.Post("/processes/{process}/setting", s.UpdateProcessSetting)
	r.Post("/processes", s.CreateProcess)
	r.Delete("/processes/{process}", s.DeleteProcess)
}

// Service 获取服务名称
func (s *App) Service(w http.ResponseWriter, r *http.Request) {
	service.Success(w, s.name)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, s.name)
	return types.AggregateAppStatus(ok)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	path := mainConfPath()
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("path", path))
	common.ServeConfig(w, path)
}

// UpdateConfig 保存配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "supervisor", mainConfPath(), 0644, s.name)
}

// Processes 进程列表
func (s *App) Processes(w http.ResponseWriter, r *http.Request) {
	out, err := shell.Execf(r.Context(), `supervisorctl status`)
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

	s.log.Info(s.t.Get("supervisor process list viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.Uint64("page", cast.ToUint64(r.URL.Query().Get("page"))))

	service.Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

// StartProcess 启动进程
func (s *App) StartProcess(w http.ResponseWriter, r *http.Request) {
	s.control(w, r, "start", s.t.Get("supervisor process started"))
}

// StopProcess 停止进程
func (s *App) StopProcess(w http.ResponseWriter, r *http.Request) {
	s.control(w, r, "stop", s.t.Get("supervisor process stopped"))
}

// RestartProcess 重启进程
func (s *App) RestartProcess(w http.ResponseWriter, r *http.Request) {
	s.control(w, r, "restart", s.t.Get("supervisor process restarted"))
}

// control 对单个进程执行 supervisorctl 变更命令
func (s *App) control(w http.ResponseWriter, r *http.Request, action, msg string) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 变更不跟随请求取消，restart 更是 stop+start 两步，中途取消会把进程停在已停止态
	if out, err := shell.Execf(context.WithoutCancel(r.Context()), `supervisorctl %s '%s'`, action, req.Process); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v %s", err, out)
		return
	}

	s.log.Info(msg, slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("process", req.Process))

	service.Success(w, nil)
}

// ProcessLog 进程日志
func (s *App) ProcessLog(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	logPath, err := s.processLog(req.Process)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
		return
	}

	service.Success(w, logPath)
}

// ProcessConfig 获取进程配置原文
func (s *App) ProcessConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	path := confPath(name)
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("process", name), slog.String("path", path))

	service.Success(w, config)
}

// UpdateProcessConfig 保存进程配置原文
func (s *App) UpdateProcessConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateProcessConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	path := confPath(name)
	if err = io.Write(path, req.Config, 0644); err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	s.log.Info(s.t.Get("supervisor process config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("process", name), slog.String("path", path))

	s.reload(r.Context(), w, name)
}

// GetProcessSetting 获取进程可视化参数
func (s *App) GetProcessSetting(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	config, err := io.Read(confPath(programName(req.Process)))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	setting := new(ProcessSetting)
	readSetting(config, setting)

	service.Success(w, setting)
}

// UpdateProcessSetting 保存进程可视化参数
func (s *App) UpdateProcessSetting(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateProcessSetting](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	name := programName(req.Process)
	path := confPath(name)
	config, err := io.Read(path)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(path, writeSetting(config, &req.ProcessSetting), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("supervisor process setting updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("process", name), slog.String("path", path), slog.String("user", req.User), slog.String("numprocs", req.NumProcs))

	s.reload(r.Context(), w, name)
}

// CreateProcess 添加进程
func (s *App) CreateProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[CreateProcess](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	path := confPath(req.Name)
	if io.Exists(path) {
		service.Error(w, http.StatusConflict, s.t.Get("process %s already exists", req.Name))
		return
	}

	num := cast.ToString(req.Num)
	config := `[program:` + req.Name + `]
command=` + req.Command + `
process_name=` + processNameExpr(num) + `
directory=` + req.Path + `
autostart=true
autorestart=true
user=` + req.User + `
numprocs=` + num + `
redirect_stderr=true
stdout_logfile=/var/log/supervisor/` + req.Name + `.log
stdout_logfile_maxbytes=2MB
`

	if err = io.Write(path, config, 0644); err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 三步序列中途取消会让新进程注册了却没起来
	ctx := context.WithoutCancel(r.Context())
	_, _ = shell.Execf(ctx, `supervisorctl reread`)
	_, _ = shell.Execf(ctx, `supervisorctl update`)
	_, _ = shell.Execf(ctx, `supervisorctl start '%s:'`, req.Name)

	s.log.Info(s.t.Get("supervisor process created"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("process", req.Name), slog.String("user", req.User), slog.String("directory", req.Path), slog.Int("numprocs", req.Num), slog.String("path", path))

	service.Success(w, nil)
}

// DeleteProcess 删除进程
func (s *App) DeleteProcess(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ProcessName](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 停进程→删配置→reread/update 中途取消会让配置已删而 supervisord 仍挂着该进程
	ctx := context.WithoutCancel(r.Context())
	name := programName(req.Process)
	if out, err := shell.Execf(ctx, `supervisorctl stop '%s:'`, name); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v %s", err, out)
		return
	}

	logPath, err := s.processLog(req.Process)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get log path for process %s: %v", req.Process, err))
		return
	}

	path := confPath(name)
	if err = io.Remove(path); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Remove(logPath); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	_, _ = shell.Execf(ctx, `supervisorctl reread`)
	_, _ = shell.Execf(ctx, `supervisorctl update`)

	s.log.Warn(s.t.Get("supervisor process deleted"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "supervisor"), slog.String("process", name), slog.String("path", path), slog.String("log_path", logPath))

	service.Success(w, nil)
}

func (s *App) processLog(process string) (string, error) {
	config, err := io.Read(confPath(programName(process)))
	if err != nil {
		return "", err
	}

	return confval.Supervisor.Get(config, "stdout_logfile"), nil
}

func (s *App) reload(ctx context.Context, w http.ResponseWriter, name string) {
	// reread→update→restart 中途取消会让进程停在半启动态
	ctx = context.WithoutCancel(ctx)
	_, _ = shell.Execf(ctx, `supervisorctl reread`)
	_, _ = shell.Execf(ctx, `supervisorctl update`)
	_, _ = shell.Execf(ctx, `supervisorctl restart '%s:'`, name)

	service.Success(w, nil)
}

// programName 从 supervisorctl 的显示名还原出程序名，program 名本身不允许含 ":"，切分无歧义
func programName(process string) string {
	name, _, _ := strings.Cut(process, ":")
	return name
}
