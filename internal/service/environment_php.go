package service

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/types"
)

type EnvironmentPHPService struct {
	t               *gotext.Locale
	conf            *config.Config
	environmentRepo biz.EnvironmentRepo
	taskRepo        biz.TaskRepo
}

func NewEnvironmentPHPService(t *gotext.Locale, conf *config.Config, environmentRepo biz.EnvironmentRepo, taskRepo biz.TaskRepo) *EnvironmentPHPService {
	return &EnvironmentPHPService{
		t:               t,
		conf:            conf,
		environmentRepo: environmentRepo,
		taskRepo:        taskRepo,
	}
}

func (s *EnvironmentPHPService) SetCli(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	binPath := fmt.Sprintf("%s/server/php/%d/bin", app.Root, req.Version)
	if err = io.LinkCLIBinaries(binPath, []string{"php"}); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) PHPInfo(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	// 使用 php-cgi 执行 phpinfo() 获取 HTML 格式输出
	output, err := shell.Execf("echo '<?php phpinfo();' | %s/server/php/%d/bin/php-cgi -q", app.Root, req.Version)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, output)
}

func (s *EnvironmentPHPService) GetConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	ini, err := io.Read(fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, req.Version))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, ini)
}

func (s *EnvironmentPHPService) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPUpdateConfig](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, req.Version), req.Config, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) GetFPMConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	ini, err := io.Read(fmt.Sprintf("%s/server/php/%d/etc/php-fpm.conf", app.Root, req.Version))
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, ini)
}

func (s *EnvironmentPHPService) UpdateFPMConfig(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPUpdateConfig](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/php/%d/etc/php-fpm.conf", app.Root, req.Version), req.Config, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) Load(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	var raw map[string]any
	client := resty.New().SetTimeout(10 * time.Second)
	defer func(client *resty.Client) { _ = client.Close() }(client)
	_, err = client.R().SetResult(&raw).Get(fmt.Sprintf("http://127.0.0.1/phpfpm_status/%d?json", req.Version))
	if err != nil {
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
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	Success(w, fmt.Sprintf("%s/server/php/%d/var/log/php-fpm.log", app.Root, req.Version))
}

func (s *EnvironmentPHPService) SlowLog(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	Success(w, fmt.Sprintf("%s/server/php/%d/var/log/slow.log", app.Root, req.Version))
}

func (s *EnvironmentPHPService) ClearLog(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	if _, err = shell.Execf("cat /dev/null > %s/server/php/%d/var/log/php-fpm.log", app.Root, req.Version); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) ClearSlowLog(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	if _, err = shell.Execf("cat /dev/null > %s/server/php/%d/var/log/slow.log", app.Root, req.Version); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) ModuleList(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	modules := s.getModules(req.Version)
	raw, err := shell.Execf("%s/server/php/%d/bin/php -m", app.Root, req.Version)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	moduleMap := make(map[string]*types.EnvironmentPHPModule)
	for i := range modules {
		moduleMap[modules[i].Slug] = &modules[i]
	}

	rawModuleList := strings.SplitSeq(raw, "\n")
	for item := range rawModuleList {
		if ext, exists := moduleMap[item]; exists && !strings.Contains(item, "[") && item != "" {
			ext.Installed = true
		}
	}

	Success(w, modules)
}

func (s *EnvironmentPHPService) InstallModule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPModule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	if !s.checkModule(req.Version, req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Slug))
		return
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php_exts/%s.sh' | bash -s -- 'install' '%d'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug), req.Version)
	officials := []string{"fileinfo", "exif", "imap", "pgsql", "pdo_pgsql", "zip", "bz2", "readline", "snmp", "ldap", "enchant", "pspell", "calendar", "gmp", "sysvmsg", "sysvsem", "sysvshm", "xsl", "intl", "gettext"}
	if slices.Contains(officials, req.Slug) {
		cmd = fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php_exts/official.sh' | bash -s -- 'install' '%d' '%s'`, s.conf.App.DownloadEndpoint, req.Version, req.Slug)
	}

	task := new(biz.Task)
	task.Name = s.t.Get("Install PHP-%d %s module", req.Version, req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) UninstallModule(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPModule](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	if !s.checkModule(req.Version, req.Slug) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("module %s does not exist", req.Slug))
		return
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php_exts/%s.sh' | bash -s -- 'uninstall' '%d'`, s.conf.App.DownloadEndpoint, url.PathEscape(req.Slug), req.Version)
	officials := []string{"fileinfo", "exif", "imap", "pgsql", "pdo_pgsql", "zip", "bz2", "readline", "snmp", "ldap", "enchant", "pspell", "calendar", "gmp", "sysvmsg", "sysvsem", "sysvshm", "xsl", "intl", "gettext"}
	if slices.Contains(officials, req.Slug) {
		cmd = fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://%s/php_exts/official.sh' | bash -s -- 'uninstall' '%d' '%s'`, s.conf.App.DownloadEndpoint, req.Version, req.Slug)
	}

	task := new(biz.Task)
	task.Name = s.t.Get("Uninstall PHP-%d %s module", req.Version, req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	if err = s.taskRepo.Push(task); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *EnvironmentPHPService) getModules(version uint) []types.EnvironmentPHPModule {
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

	raw, _ := shell.Execf("%s/server/php/%d/bin/php -m", app.Root, version)
	moduleMap := make(map[string]*types.EnvironmentPHPModule)
	for i := range modules {
		moduleMap[modules[i].Slug] = &modules[i]
	}

	rawModuleList := strings.SplitSeq(raw, "\n")
	for item := range rawModuleList {
		if ext, exists := moduleMap[item]; exists && !strings.Contains(item, "[") && item != "" {
			ext.Installed = true
		}
	}

	return modules
}

func (s *EnvironmentPHPService) checkModule(version uint, slug string) bool {
	modules := s.getModules(version)

	for _, item := range modules {
		if item.Slug == slug {
			return true
		}
	}

	return false
}

// GetConfigTune 获取 PHP 配置调整参数
func (s *EnvironmentPHPService) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	iniPath := fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, req.Version)
	fpmPath := fmt.Sprintf("%s/server/php/%d/etc/php-fpm.conf", app.Root, req.Version)

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

	tune := request.EnvironmentPHPConfigTune{
		// php.ini 常规设置
		ShortOpenTag:   s.getINIValue(ini, "short_open_tag"),
		DateTimezone:   s.getINIValue(ini, "date.timezone"),
		DisplayErrors:  s.getINIValue(ini, "display_errors"),
		ErrorReporting: s.getINIValue(ini, "error_reporting"),
		// php.ini 禁用函数
		DisableFunctions: s.getINIValue(ini, "disable_functions"),
		// php.ini 上传限制
		UploadMaxFilesize: s.getINIValue(ini, "upload_max_filesize"),
		PostMaxSize:       s.getINIValue(ini, "post_max_size"),
		MaxFileUploads:    s.getINIValue(ini, "max_file_uploads"),
		MemoryLimit:       s.getINIValue(ini, "memory_limit"),
		// php.ini 超时限制
		MaxExecutionTime: s.getINIValue(ini, "max_execution_time"),
		MaxInputTime:     s.getINIValue(ini, "max_input_time"),
		MaxInputVars:     s.getINIValue(ini, "max_input_vars"),
		// Session 相关
		SessionSaveHandler:    s.getINIValue(ini, "session.save_handler"),
		SessionSavePath:       s.getINIValue(ini, "session.save_path"),
		SessionGcMaxlifetime:  s.getINIValue(ini, "session.gc_maxlifetime"),
		SessionCookieLifetime: s.getINIValue(ini, "session.cookie_lifetime"),
		// php-fpm.conf 配置
		Pm:                s.getINIValue(fpm, "pm"),
		PmMaxChildren:     s.getINIValue(fpm, "pm.max_children"),
		PmStartServers:    s.getINIValue(fpm, "pm.start_servers"),
		PmMinSpareServers: s.getINIValue(fpm, "pm.min_spare_servers"),
		PmMaxSpareServers: s.getINIValue(fpm, "pm.max_spare_servers"),
	}

	Success(w, tune)
}

// UpdateConfigTune 更新 PHP 配置调整参数
func (s *EnvironmentPHPService) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPConfigTune](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	iniPath := fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, req.Version)
	fpmPath := fmt.Sprintf("%s/server/php/%d/etc/php-fpm.conf", app.Root, req.Version)

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

	// 更新 php.ini 配置
	ini = s.setINIValue(ini, "short_open_tag", req.ShortOpenTag)
	ini = s.setINIValue(ini, "date.timezone", req.DateTimezone)
	ini = s.setINIValue(ini, "display_errors", req.DisplayErrors)
	ini = s.setINIValue(ini, "error_reporting", req.ErrorReporting)
	ini = s.setINIValue(ini, "disable_functions", req.DisableFunctions)
	ini = s.setINIValue(ini, "upload_max_filesize", req.UploadMaxFilesize)
	ini = s.setINIValue(ini, "post_max_size", req.PostMaxSize)
	ini = s.setINIValue(ini, "max_execution_time", req.MaxExecutionTime)
	ini = s.setINIValue(ini, "max_input_time", req.MaxInputTime)
	ini = s.setINIValue(ini, "memory_limit", req.MemoryLimit)
	ini = s.setINIValue(ini, "max_input_vars", req.MaxInputVars)
	ini = s.setINIValue(ini, "max_file_uploads", req.MaxFileUploads)
	ini = s.setINIValue(ini, "session.save_handler", req.SessionSaveHandler)
	ini = s.setINIValue(ini, "session.save_path", req.SessionSavePath)
	ini = s.setINIValue(ini, "session.gc_maxlifetime", req.SessionGcMaxlifetime)
	ini = s.setINIValue(ini, "session.cookie_lifetime", req.SessionCookieLifetime)

	// 更新 php-fpm.conf 配置
	fpm = s.setINIValue(fpm, "pm", req.Pm)
	fpm = s.setINIValue(fpm, "pm.max_children", req.PmMaxChildren)
	fpm = s.setINIValue(fpm, "pm.start_servers", req.PmStartServers)
	fpm = s.setINIValue(fpm, "pm.min_spare_servers", req.PmMinSpareServers)
	fpm = s.setINIValue(fpm, "pm.max_spare_servers", req.PmMaxSpareServers)

	if err = io.Write(iniPath, ini, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Write(fpmPath, fpm, 0644); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// CleanSession 清理 PHP Session 文件
func (s *EnvironmentPHPService) CleanSession(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.EnvironmentPHPVersion](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if !s.environmentRepo.IsInstalled("php", fmt.Sprintf("%d", req.Version)) {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("PHP-%d is not installed", req.Version))
		return
	}

	iniPath := fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, req.Version)
	ini, err := io.Read(iniPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	handler := s.getINIValue(ini, "session.save_handler")
	if handler != "files" {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("Session save handler is not files, cannot clean"))
		return
	}

	savePath := s.getINIValue(ini, "session.save_path")
	if savePath == "" {
		savePath = "/tmp"
	}

	if _, err = shell.Execf("find '%s' -name 'sess_*' -type f -delete", savePath); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

// getINIValue 从 INI 格式内容中获取指定键的值
func (s *EnvironmentPHPService) getINIValue(content string, key string) string {
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		// 跳过注释行和空行
		if trimmed == "" || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// 跳过 section 行
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
func (s *EnvironmentPHPService) setINIValue(content string, key string, value string) string {
	// 过滤值中的换行符，防止破坏 INI 文件结构
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")

	lines := strings.Split(content, "\n")
	found := false
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 跳过空行和 section 行
		if trimmed == "" || strings.HasPrefix(trimmed, "[") {
			result = append(result, line)
			continue
		}
		// 处理注释行（可能是被注释掉的配置）
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
				// 如果已经找到并替换过，跳过重复行
				continue
			}
			found = true
			// 值为空时注释掉该配置项
			if value == "" {
				if !strings.HasPrefix(trimmed, ";") && !strings.HasPrefix(trimmed, "#") {
					result = append(result, ";"+line)
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
