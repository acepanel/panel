package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/str"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/fastcgi"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/lsapi"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
	"github.com/acepanel/panel/v3/pkg/webserver"
	"github.com/acepanel/panel/v3/pkg/webserver/openlitespeed"
)

type EnvironmentPHPService struct {
	t               *gotext.Locale
	log             *slog.Logger
	conf            *config.Config
	environmentRepo *biz.EnvironmentUsecase
	taskRepo        *biz.TaskUsecase
}

// phpTuneField 把配置键对应到调优请求的字段
type phpTuneField struct {
	key   string
	value *string
}

func NewEnvironmentPHPService(
	environmentUsecase *biz.EnvironmentUsecase,
	taskUsecase *biz.TaskUsecase,
	conf *config.Config,
	t *gotext.Locale,
	log *slog.Logger,
) *EnvironmentPHPService {
	return &EnvironmentPHPService{
		t:               t,
		log:             log,
		conf:            conf,
		environmentRepo: environmentUsecase,
		taskRepo:        taskUsecase,
	}
}

func (s *EnvironmentPHPService) SetCli(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	binPath, err := linkEnvironmentCLI("php", strconv.FormatUint(uint64(version), 10))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("environment cli set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("environment_type", "php"), slog.String("slug", fmt.Sprintf("php%d", version)), slog.String("bin_path", binPath))

	Success(w, nil)
}

func (s *EnvironmentPHPService) PHPInfo(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	// 使用 php-cgi 执行 phpinfo() 获取 HTML 格式输出
	output, err := shell.Execf(r.Context(), "echo '<?php phpinfo();' | %s -q", s.phpPath(version, "bin/php-cgi"))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("phpinfo viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)))

	Success(w, output)
}

func (s *EnvironmentPHPService) GetConfig(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	path := s.phpPath(version, "etc/php.ini")
	ini, err := io.Read(path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("php config viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)), slog.String("path", path))

	Success(w, ini)
}

func (s *EnvironmentPHPService) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPUpdateConfig](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.installed(w, req.Version) {
		return
	}

	path := s.phpPath(req.Version, "etc/php.ini")
	if err = io.Write(path, req.Config, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.applyLSAPI(r.Context(), req.Version); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("php config saved"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("path", path))

	Success(w, nil)
}

func (s *EnvironmentPHPService) GetFPMConfig(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	path := s.phpPath(version, "etc/php-fpm.conf")
	ini, err := io.Read(path)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("php-fpm config viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)), slog.String("path", path))

	Success(w, ini)
}

func (s *EnvironmentPHPService) UpdateFPMConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPUpdateConfig](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.installed(w, req.Version) {
		return
	}

	path := s.phpPath(req.Version, "etc/php-fpm.conf")
	if err = io.Write(path, req.Config, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.applyLSAPI(r.Context(), req.Version); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("php-fpm config saved"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("path", path))

	Success(w, nil)
}

func (s *EnvironmentPHPService) Load(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	var raw map[string]any
	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	if _, err := client.R().SetResult(&raw).Get(fmt.Sprintf("http://127.0.0.1/phpfpm_status/%d?json", version)); err != nil {
		Success(w, []types.NV{})
		return
	}

	dataKeys := []string{
		s.t.Get("Application Pool"),
		s.t.Get("Process Manager"),
		s.t.Get("Start Time"),
		s.t.Get("Accepted Connections"),
		s.t.Get("Listen Queue"),
		s.t.Get("Max Listen Queue"),
		s.t.Get("Listen Queue Length"),
		s.t.Get("Idle Processes"),
		s.t.Get("Active Processes"),
		s.t.Get("Total Processes"),
		s.t.Get("Max Active Processes"),
		s.t.Get("Max Children Reached"),
		s.t.Get("Slow Requests"),
	}
	rawKeys := []string{
		"pool",
		"process manager",
		"start time",
		"accepted conn",
		"listen queue",
		"max listen queue",
		"listen queue len",
		"idle processes",
		"active processes",
		"total processes",
		"max active processes",
		"max children reached",
		"slow requests",
	}

	loads := make([]types.NV, 0)
	for i := range dataKeys {
		v, ok := raw[rawKeys[i]]
		if ok {
			loads = append(loads, types.NV{
				Name:  dataKeys[i],
				Value: cast.ToString(v),
			})
		}
	}

	Success(w, loads)
}

func (s *EnvironmentPHPService) Log(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	Success(w, s.phpPath(version, "var/log/php-fpm.log"))
}

func (s *EnvironmentPHPService) SlowLog(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	Success(w, s.phpPath(version, "var/log/slow.log"))
}

func (s *EnvironmentPHPService) ModuleList(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	raw, err := shell.Execf(r.Context(), "%s -m", s.phpPath(version, "bin/php"))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 已加载的模块在 php -m 的输出里各占一行
	loaded := strings.Split(raw, "\n")
	modules := s.modules(version)
	for i := range modules {
		modules[i].Installed = slices.Contains(loaded, modules[i].Slug)
	}

	s.log.Info(s.t.Get("php module list viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)))

	Success(w, modules)
}

func (s *EnvironmentPHPService) InstallModule(w http.ResponseWriter, r *http.Request) {
	req, ok := s.bindModule(w, r)
	if !ok {
		return
	}
	if !s.pushModuleTask(w, req, "install", s.t.Get("Install PHP-%d %s module", req.Version, req.Slug)) {
		return
	}

	s.log.Info(s.t.Get("php module installed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("module", req.Slug))

	Success(w, nil)
}

func (s *EnvironmentPHPService) UninstallModule(w http.ResponseWriter, r *http.Request) {
	req, ok := s.bindModule(w, r)
	if !ok {
		return
	}
	if !s.pushModuleTask(w, req, "uninstall", s.t.Get("Uninstall PHP-%d %s module", req.Version, req.Slug)) {
		return
	}

	s.log.Warn(s.t.Get("php module uninstalled"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("module", req.Slug))

	Success(w, nil)
}

// GetConfigTune 获取 PHP 配置调整参数
func (s *EnvironmentPHPService) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	iniPath := s.phpPath(version, "etc/php.ini")
	fpmPath := s.phpPath(version, "etc/php-fpm.conf")

	ini, err := io.Read(iniPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	fpm, err := io.Read(fpmPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	var tune request.EnvironmentPHPConfigTune
	iniFields, fpmFields := s.tuneFields(&tune)
	for _, field := range iniFields {
		*field.value = confval.PHPINI.Get(ini, field.key)
	}
	for _, field := range fpmFields {
		*field.value = confval.PHPINI.Get(fpm, field.key)
	}

	s.log.Info(s.t.Get("php config tune viewed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)), slog.String("ini_path", iniPath), slog.String("fpm_path", fpmPath))

	Success(w, tune)
}

// UpdateConfigTune 更新 PHP 配置调整参数
func (s *EnvironmentPHPService) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPConfigTune](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.installed(w, req.Version) {
		return
	}

	iniPath := s.phpPath(req.Version, "etc/php.ini")
	fpmPath := s.phpPath(req.Version, "etc/php-fpm.conf")

	ini, err := io.Read(iniPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	fpm, err := io.Read(fpmPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	iniFields, fpmFields := s.tuneFields(req)
	for _, field := range iniFields {
		ini = confval.PHPINI.Set(ini, field.key, *field.value)
	}
	for _, field := range fpmFields {
		fpm = confval.PHPINI.Set(fpm, field.key, *field.value)
	}

	if err = io.Write(iniPath, ini, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Write(fpmPath, fpm, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = s.applyLSAPI(r.Context(), req.Version); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("php config tune saved"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("ini_path", iniPath), slog.String("fpm_path", fpmPath))

	Success(w, nil)
}

// CleanSession 清理 PHP Session 文件
func (s *EnvironmentPHPService) CleanSession(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	iniPath := s.phpPath(version, "etc/php.ini")
	ini, err := io.Read(iniPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	handler := confval.PHPINI.Get(ini, "session.save_handler")
	if handler != "files" {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Session save handler is not files, cannot clean"))
		return
	}

	savePath := confval.PHPINI.Get(ini, "session.save_path")
	if savePath == "" {
		savePath = "/tmp"
	}

	if _, err = shell.Execf(r.Context(), "find '%s' -name 'sess_*' -type f -delete", savePath); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("php sessions cleaned"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)), slog.String("path", savePath))

	Success(w, nil)
}

// Processes 获取 PHP-FPM 工作进程列表
func (s *EnvironmentPHPService) Processes(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	var raw struct {
		Processes []struct {
			PID             int64   `json:"pid"`
			State           string  `json:"state"`
			StartSince      int64   `json:"start since"`
			Requests        int64   `json:"requests"`
			RequestDuration int64   `json:"request duration"`
			Method          string  `json:"request method"`
			URI             string  `json:"request uri"`
			Script          string  `json:"script"`
			LastCPU         float64 `json:"last request cpu"`
			LastMemory      int64   `json:"last request memory"`
		} `json:"processes"`
	}
	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	if _, err := client.R().SetResult(&raw).Get(fmt.Sprintf("http://127.0.0.1/phpfpm_status/%d?json&full", version)); err != nil {
		Success(w, []types.EnvironmentPHPProcess{})
		return
	}

	processes := make([]types.EnvironmentPHPProcess, 0, len(raw.Processes))
	for _, item := range raw.Processes {
		processes = append(processes, types.EnvironmentPHPProcess{
			PID:             item.PID,
			State:           item.State,
			StartSince:      item.StartSince,
			Requests:        item.Requests,
			RequestDuration: item.RequestDuration,
			Method:          item.Method,
			URI:             item.URI,
			Script:          item.Script,
			LastRequestCPU:  item.LastCPU,
			LastRequestMem:  item.LastMemory,
		})
	}

	Success(w, processes)
}

// Opcache 获取 OPcache 状态
func (s *EnvironmentPHPService) Opcache(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	socket, _ := s.probeSocket(version)
	if _, err := os.Stat(socket); err != nil {
		Success(w, types.EnvironmentPHPOpcache{Message: s.t.Get("PHP-%d backend is not running, unable to get OPcache status", version)})
		return
	}

	body, err := s.opcacheProbe(r.Context(), version, "")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get OPcache status: %v", err))
		return
	}

	text := strings.TrimSpace(string(body))
	if text == "null" {
		Success(w, types.EnvironmentPHPOpcache{Message: s.t.Get("OPcache is not enabled. Install the Zend OPcache module in Module Management to significantly improve PHP performance.")})
		return
	}
	var raw map[string]any
	if text != "false" {
		if err = json.Unmarshal(body, &raw); err != nil {
			Error(w, http.StatusInternalServerError, s.t.Get("failed to get OPcache status: %s", text))
			return
		}
	}

	memory := cast.ToStringMap(raw["memory_usage"])
	stats := cast.ToStringMap(raw["opcache_statistics"])
	jit := cast.ToStringMap(raw["jit"])

	result := types.EnvironmentPHPOpcache{
		Enabled:       cast.ToBool(raw["opcache_enabled"]),
		MemoryUsed:    tools.FormatBytes(cast.ToFloat64(memory["used_memory"])),
		MemoryFree:    tools.FormatBytes(cast.ToFloat64(memory["free_memory"])),
		MemoryWasted:  tools.FormatBytes(cast.ToFloat64(memory["wasted_memory"])),
		WastedPercent: math.Round(cast.ToFloat64(memory["current_wasted_percentage"])*100) / 100,
		HitRate:       math.Round(cast.ToFloat64(stats["opcache_hit_rate"])*100) / 100,
		Hits:          cast.ToInt64(stats["hits"]),
		Misses:        cast.ToInt64(stats["misses"]),
		CachedScripts: cast.ToInt64(stats["num_cached_scripts"]),
		CachedKeys:    cast.ToInt64(stats["num_cached_keys"]),
		MaxCachedKeys: cast.ToInt64(stats["max_cached_keys"]),
		OomRestarts:   cast.ToInt64(stats["oom_restarts"]),
		JitEnabled:    cast.ToBool(jit["enabled"]) && cast.ToBool(jit["on"]),
		JitBufferSize: tools.FormatBytes(cast.ToFloat64(jit["buffer_size"])),
		JitBufferFree: tools.FormatBytes(cast.ToFloat64(jit["buffer_free"])),
	}
	if !result.Enabled {
		result.Message = s.t.Get("OPcache is loaded but not running, check the opcache settings in php.ini and the PHP error log")
	}

	Success(w, result)
}

// ResetOpcache 重置 OPcache
func (s *EnvironmentPHPService) ResetOpcache(w http.ResponseWriter, r *http.Request) {
	version, ok := s.bindVersion(w, r)
	if !ok {
		return
	}

	socket, _ := s.probeSocket(version)
	if _, err := os.Stat(socket); err != nil {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d backend is not running", version))
		return
	}

	body, err := s.opcacheProbe(r.Context(), version, "action=reset")
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to reset OPcache: %v", err))
		return
	}

	var raw map[string]any
	if err = json.Unmarshal(body, &raw); err != nil || !cast.ToBool(raw["reset"]) {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to reset OPcache, it may not be enabled"))
		return
	}

	s.log.Info(s.t.Get("php opcache reset"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(version)))

	Success(w, nil)
}

// Composer 获取 Composer 状态
func (s *EnvironmentPHPService) Composer(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	composer := types.EnvironmentPHPComposer{Installed: io.Exists("/usr/local/bin/composer")}
	if composer.Installed {
		out, outErr := shell.ExecfWithEnv(r.Context(), []string{"COMPOSER_ALLOW_SUPERUSER=1"}, "%s /usr/local/bin/composer --version --no-ansi 2>/dev/null", s.phpPath(req.Version, "bin/php"))
		// 输出形如 Composer version 2.8.4 2025-01-01 00:00:00
		if fields := strings.Fields(out); outErr == nil && len(fields) >= 3 {
			composer.Version = fields[2]
		}
		composer.Mirror = s.composerMirror()
	}

	Success(w, composer)
}

// InstallComposer 安装/更新 Composer（异步任务）
func (s *EnvironmentPHPService) InstallComposer(w http.ResponseWriter, r *http.Request) {
	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php/composer.sh' | bash -s -- 'install'`, s.conf.App.DownloadEndpoint)

	task := new(biz.Task)
	task.Key = "php:composer"
	task.Name = s.t.Get("Install Composer")
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err := s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("composer installed"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("path", "/usr/local/bin/composer"))

	Success(w, nil)
}

// SetComposerMirror 设置 Composer 全局镜像源
func (s *EnvironmentPHPService) SetComposerMirror(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPComposerMirror](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !io.Exists("/usr/local/bin/composer") {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Composer is not installed"))
		return
	}

	php := s.phpPath(req.Version, "bin/php")
	env := []string{"COMPOSER_ALLOW_SUPERUSER=1"}
	if req.Mirror == "" {
		// 恢复官方源，未设置过镜像时报错可忽略
		_, _ = shell.ExecfWithEnv(r.Context(), env, "%s /usr/local/bin/composer config -g --unset repos.packagist", php)
		s.log.Info(s.t.Get("composer mirror set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("mirror", redactURL(req.Mirror)))
		Success(w, nil)
		return
	}

	if !strings.HasPrefix(req.Mirror, "https://") && !strings.HasPrefix(req.Mirror, "http://") {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid mirror url"))
		return
	}
	if _, err = shell.ExecfWithEnv(r.Context(), env, "%s /usr/local/bin/composer config -g repos.packagist composer '%s'", php, req.Mirror); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("composer mirror set"), slog.String("type", biz.OperationTypeEnvironment), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.Uint64("version", uint64(req.Version)), slog.String("mirror", redactURL(req.Mirror)))

	Success(w, nil)
}

// bindVersion 绑定路径里的版本并确认已安装，失败时响应已写入
func (s *EnvironmentPHPService) bindVersion(w http.ResponseWriter, r *http.Request) (uint, bool) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return 0, false
	}

	return req.Version, s.installed(w, req.Version)
}

// installed 版本未安装时写入错误响应
func (s *EnvironmentPHPService) installed(w http.ResponseWriter, version uint) bool {
	if s.environmentRepo.IsInstalled("php", strconv.FormatUint(uint64(version), 10)) {
		return true
	}

	Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", version))
	return false
}

func (s *EnvironmentPHPService) phpPath(version uint, rel string) string {
	return fmt.Sprintf("%s/server/php/%d/%s", app.Root, version, rel)
}

// applyLSAPI LSAPI 下 lsphp 是 OLS 的子进程，重载 OLS 会同步外部应用参数并重启 lsphp
func (s *EnvironmentPHPService) applyLSAPI(ctx context.Context, version uint) error {
	if !openlitespeed.LSAPIEnabled(version) {
		return nil
	}
	d, err := webserver.Get(webserver.TypeOpenLiteSpeed)
	if err != nil {
		return err
	}

	return d.Reload(context.WithoutCancel(ctx))
}

// modules 可安装的模块目录，不含安装状态
func (s *EnvironmentPHPService) modules(version uint) []types.EnvironmentPHPModule {
	modules := []types.EnvironmentPHPModule{
		{
			Name:        "fileinfo",
			Slug:        "fileinfo",
			Description: s.t.Get("Fileinfo is a library used to identify file types"),
		},
		{
			Name:        "OPcache",
			Slug:        "Zend OPcache",
			Description: s.t.Get("OPcache stores precompiled PHP script bytecode in shared memory to improve PHP performance"),
		},
		{
			Name:        "igbinary",
			Slug:        "igbinary",
			Description: s.t.Get("Igbinary is a library for serializing and deserializing data"),
		},
		{
			Name:        "Redis",
			Slug:        "redis",
			Description: s.t.Get("PhpRedis connects to and operates on data in Redis databases (requires the igbinary module installed above)"),
		},
		{
			Name:        "Memcached",
			Slug:        "memcached",
			Description: s.t.Get("Memcached is a driver for connecting to Memcached servers"),
		},
		{
			Name:        "APCu",
			Slug:        "apcu",
			Description: s.t.Get("APCu is a user-level cache for PHP, providing fast in-memory key-value storage"),
		},
		{
			Name:        "ImageMagick",
			Slug:        "imagick",
			Description: s.t.Get("ImageMagick is free software for creating, editing, and composing images"),
		},
		{
			Name:        "exif",
			Slug:        "exif",
			Description: s.t.Get("Exif is a library for reading and writing image metadata"),
		},
		{
			Name:        "pgsql",
			Slug:        "pgsql",
			Description: s.t.Get("pgsql is a driver for connecting to PostgreSQL (requires PostgreSQL installed)"),
		},
		{
			Name:        "pdo_pgsql",
			Slug:        "pdo_pgsql",
			Description: s.t.Get("pdo_pgsql is a PDO driver for connecting to PostgreSQL (requires PostgreSQL installed)"),
		},
		{
			Name:        "sqlsrv",
			Slug:        "sqlsrv",
			Description: s.t.Get("sqlsrv is a driver for connecting to SQL Server"),
		},
		{
			Name:        "pdo_sqlsrv",
			Slug:        "pdo_sqlsrv",
			Description: s.t.Get("pdo_sqlsrv is a PDO driver for connecting to SQL Server"),
		},
		{
			Name:        "imap",
			Slug:        "imap",
			Description: s.t.Get("IMAP module allows PHP to read, search, delete, download, and manage emails"),
		},
		{
			Name:        "zip",
			Slug:        "zip",
			Description: s.t.Get("Zip is a library for handling ZIP files"),
		},
		{
			Name:        "bz2",
			Slug:        "bz2",
			Description: s.t.Get("Bzip2 is a library for compressing and decompressing files"),
		},
		{
			Name:        "ssh2",
			Slug:        "ssh2",
			Description: s.t.Get("SSH2 is a library for connecting to SSH servers"),
		},
		{
			Name:        "event",
			Slug:        "event",
			Description: s.t.Get("Event is a library for handling events"),
		},
		{
			Name:        "readline",
			Slug:        "readline",
			Description: s.t.Get("Readline is a library for processing text"),
		},
		{
			Name:        "snmp",
			Slug:        "snmp",
			Description: s.t.Get("SNMP is a protocol for network management"),
		},
		{
			Name:        "ldap",
			Slug:        "ldap",
			Description: s.t.Get("LDAP is a protocol for accessing directory services"),
		},
		{
			Name:        "enchant",
			Slug:        "enchant",
			Description: s.t.Get("Enchant is a spell-checking library"),
		},
		{
			Name:        "pspell",
			Slug:        "pspell",
			Description: s.t.Get("Pspell is a spell-checking library"),
		},
		{
			Name:        "calendar",
			Slug:        "calendar",
			Description: s.t.Get("Calendar is a library for handling dates"),
		},
		{
			Name:        "gmp",
			Slug:        "gmp",
			Description: s.t.Get("GMP is a library for handling large integers"),
		},
		{
			Name:        "xlswriter",
			Slug:        "xlswriter",
			Description: s.t.Get("XLSWriter is a high-performance library for reading and writing Excel files"),
		},
		{
			Name:        "xsl",
			Slug:        "xsl",
			Description: s.t.Get("XSL is a library for processing XML documents"),
		},
		{
			Name:        "intl",
			Slug:        "intl",
			Description: s.t.Get("Intl is a library for handling internationalization and localization"),
		},
		{
			Name:        "gettext",
			Slug:        "gettext",
			Description: s.t.Get("Gettext is a library for handling multilingual support"),
		},
		{
			Name:        "grpc",
			Slug:        "grpc",
			Description: s.t.Get("gRPC is a high-performance, open-source, and general-purpose RPC framework"),
		},
		{
			Name:        "protobuf",
			Slug:        "protobuf",
			Description: s.t.Get("protobuf is a library for serializing and deserializing data"),
		},
		{
			Name:        "rdkafka",
			Slug:        "rdkafka",
			Description: s.t.Get("rdkafka is a library for connecting to Apache Kafka"),
		},
		{
			Name:        "xhprof",
			Slug:        "xhprof",
			Description: s.t.Get("xhprof is a library for performance profiling"),
		},
		{
			Name:        "xdebug",
			Slug:        "xdebug",
			Description: s.t.Get("xdebug is a library for debugging and profiling PHP code"),
		},
		{
			Name:        "yaml",
			Slug:        "yaml",
			Description: s.t.Get("yaml is a library for handling YAML"),
		},
		{
			Name:        "zstd",
			Slug:        "zstd",
			Description: s.t.Get("zstd is a library for compressing and decompressing files"),
		},
		{
			Name:        "sysvmsg",
			Slug:        "sysvmsg",
			Description: s.t.Get("Sysvmsg is a library for handling System V message queues"),
		},
		{
			Name:        "sysvsem",
			Slug:        "sysvsem",
			Description: s.t.Get("Sysvsem is a library for handling System V semaphores"),
		},
		{
			Name:        "sysvshm",
			Slug:        "sysvshm",
			Description: s.t.Get("Sysvshm is a library for handling System V shared memory"),
		},
		{
			Name:        "ionCube",
			Slug:        "ionCube Loader",
			Description: s.t.Get("ionCube is a professional-grade PHP encryption and decryption tool (must be installed after OPcache)"),
		},
		{
			Name:        "Swoole",
			Slug:        "swoole",
			Description: s.t.Get("Swoole is a PHP module for building high-performance asynchronous concurrent servers"),
		},
	}

	// Swow 不支持 PHP 8.0 以下版本
	if version >= 80 {
		modules = append(modules, types.EnvironmentPHPModule{
			Name:        "Swow",
			Slug:        "Swow",
			Description: s.t.Get("Swow is a PHP module for building high-performance asynchronous concurrent servers"),
		})
	}
	// PHP 8.4 移除了 pspell 和 imap 并且不再建议使用
	if version >= 84 {
		modules = slices.DeleteFunc(modules, func(module types.EnvironmentPHPModule) bool {
			return module.Slug == "pspell" || module.Slug == "imap"
		})
	}
	// PHP 8.5 原生支持 OPcache，不再作为扩展提供安装
	if version >= 85 {
		modules = slices.DeleteFunc(modules, func(module types.EnvironmentPHPModule) bool {
			return module.Slug == "Zend OPcache"
		})
	}

	return modules
}

// bindModule 绑定模块请求并确认版本已安装、模块在目录内，失败时响应已写入
func (s *EnvironmentPHPService) bindModule(
	w http.ResponseWriter,
	r *http.Request,
) (*request.EnvironmentPHPModule, bool) {
	req, err := Bind[request.EnvironmentPHPModule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return nil, false
	}
	if !s.installed(w, req.Version) {
		return nil, false
	}
	if !lo.ContainsBy(s.modules(req.Version), func(item types.EnvironmentPHPModule) bool { return item.Slug == req.Slug }) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Slug))
		return nil, false
	}

	return req, true
}

// pushModuleTask 提交模块的安装或卸载任务，失败时响应已写入
func (s *EnvironmentPHPService) pushModuleTask(
	w http.ResponseWriter,
	req *request.EnvironmentPHPModule,
	action, name string,
) bool {
	// 由 official.sh 统一处理的官方模块
	officialModules := []string{"fileinfo", "exif", "imap", "pgsql", "pdo_pgsql", "zip", "bz2", "readline", "snmp", "ldap", "enchant", "pspell", "calendar", "gmp", "sysvmsg", "sysvsem", "sysvshm", "xsl", "intl", "gettext"}
	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php/modules/%s.sh' | bash -s -- '%s' '%d'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug), action, req.Version)
	if slices.Contains(officialModules, req.Slug) {
		cmd = fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php/modules/official.sh' | bash -s -- '%s' '%d' '%s'`, s.conf.App.DownloadEndpoint, action, req.Version, req.Slug)
	}

	task := new(biz.Task)
	task.Key = fmt.Sprintf("php:module:%d:%s", req.Version, req.Slug)
	task.Name = name
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err := s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return false
	}

	return true
}

// tuneFields 列出 php.ini 与 php-fpm.conf 的调优项，顺序即写入顺序
func (s *EnvironmentPHPService) tuneFields(tune *request.EnvironmentPHPConfigTune) (ini, fpm []phpTuneField) {
	ini = []phpTuneField{
		{"short_open_tag", &tune.ShortOpenTag},
		{"date.timezone", &tune.DateTimezone},
		{"display_errors", &tune.DisplayErrors},
		{"error_reporting", &tune.ErrorReporting},
		{"disable_functions", &tune.DisableFunctions},
		{"upload_max_filesize", &tune.UploadMaxFilesize},
		{"post_max_size", &tune.PostMaxSize},
		{"max_execution_time", &tune.MaxExecutionTime},
		{"max_input_time", &tune.MaxInputTime},
		{"memory_limit", &tune.MemoryLimit},
		{"max_input_vars", &tune.MaxInputVars},
		{"max_file_uploads", &tune.MaxFileUploads},
		{"session.save_handler", &tune.SessionSaveHandler},
		{"session.save_path", &tune.SessionSavePath},
		{"session.gc_maxlifetime", &tune.SessionGcMaxlifetime},
		{"session.cookie_lifetime", &tune.SessionCookieLifetime},
		{"opcache.memory_consumption", &tune.OpcacheMemoryConsumption},
		{"opcache.interned_strings_buffer", &tune.OpcacheInternedStringsBuffer},
		{"opcache.max_accelerated_files", &tune.OpcacheMaxAcceleratedFiles},
		{"opcache.revalidate_freq", &tune.OpcacheRevalidateFreq},
		{"opcache.jit", &tune.OpcacheJit},
		{"opcache.jit_buffer_size", &tune.OpcacheJitBufferSize},
	}
	fpm = []phpTuneField{
		{"pm", &tune.Pm},
		{"pm.max_children", &tune.PmMaxChildren},
		{"pm.start_servers", &tune.PmStartServers},
		{"pm.min_spare_servers", &tune.PmMinSpareServers},
		{"pm.max_spare_servers", &tune.PmMaxSpareServers},
	}

	return ini, fpm
}

// probeSocket 探针要连的套接字
func (s *EnvironmentPHPService) probeSocket(version uint) (string, bool) {
	if openlitespeed.LSAPIEnabled(version) {
		return openlitespeed.LSAPISocket(version), true
	}
	return s.phpFPMSocket(version), false
}

func (s *EnvironmentPHPService) phpFPMSocket(version uint) string {
	return fmt.Sprintf("/tmp/php-cgi-%d.sock", version)
}

// opcacheProbe 通过 FastCGI 请求 PHP-FPM 执行 OPcache 探针脚本
func (s *EnvironmentPHPService) opcacheProbe(ctx context.Context, version uint, query string) ([]byte, error) {
	probePath := "/tmp/acepanel_opcache_probe.php"
	probe := `<?php
if (($_GET['action'] ?? '') === 'reset') {
    echo json_encode(['reset' => function_exists('opcache_reset') && opcache_reset()]);
    exit;
}
echo json_encode(function_exists('opcache_get_status') ? opcache_get_status(false) : null);
`
	tmpPath := probePath + "." + str.Random(8)
	if err := io.Write(tmpPath, probe, 0644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpPath, probePath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	socket, viaLSAPI := s.probeSocket(version)
	params := map[string]string{
		"SCRIPT_FILENAME":   probePath,
		"SCRIPT_NAME":       "/acepanel_opcache_probe.php",
		"REQUEST_METHOD":    "GET",
		"QUERY_STRING":      query,
		"SERVER_PROTOCOL":   "HTTP/1.1",
		"GATEWAY_INTERFACE": "CGI/1.1",
		"REMOTE_ADDR":       "127.0.0.1",
		"SERVER_ADDR":       "127.0.0.1",
		"SERVER_PORT":       "80",
		"SERVER_NAME":       "localhost",
	}

	if viaLSAPI {
		return lsapi.Request(timeoutCtx, "unix", socket, params)
	}
	return fastcgi.Request(timeoutCtx, "unix", socket, params)
}

func (s *EnvironmentPHPService) composerMirror() string {
	type repository struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}

	for _, path := range []string{"/root/.config/composer/config.json", "/root/.composer/config.json"} {
		content, err := io.Read(path)
		if err != nil {
			continue
		}
		var cfg struct {
			Repositories json.RawMessage `json:"repositories"`
		}
		if json.Unmarshal([]byte(content), &cfg) != nil {
			continue
		}
		// Composer 2.9 起写成带 name 的数组，旧版本写的是以名字为键的对象
		var repos []repository
		if json.Unmarshal(cfg.Repositories, &repos) != nil {
			var named map[string]repository
			_ = json.Unmarshal(cfg.Repositories, &named)
			for name, repo := range named {
				repo.Name = name
				repos = append(repos, repo)
			}
		}
		for _, repo := range repos {
			if (repo.Name == "packagist" || repo.Name == "packagist.org") && repo.URL != "" {
				return repo.URL
			}
		}
	}

	return ""
}
