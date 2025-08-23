package php

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/go-resty/resty/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/tnborg/panel/internal/app"
	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/service"
	"github.com/tnborg/panel/pkg/io"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/types"
)

type App struct {
	version  uint
	t        *gotext.Locale
	taskRepo biz.TaskRepo
}

func NewApp(t *gotext.Locale, task biz.TaskRepo) *App {
	return &App{
		t:        t,
		taskRepo: task,
	}
}

func (s *App) Route(version uint) func(r fiber.Router) {
	return func(r fiber.Router) {
		php := new(App)
		php.version = version
		php.t = s.t
		php.taskRepo = s.taskRepo
		r.Post("/set_cli", php.SetCli)
		r.Get("/config", php.GetConfig)
		r.Post("/config", php.UpdateConfig)
		r.Get("/fpm_config", php.GetFPMConfig)
		r.Post("/fpm_config", php.UpdateFPMConfig)
		r.Get("/load", php.Load)
		r.Get("/error_log", php.ErrorLog)
		r.Get("/slow_log", php.SlowLog)
		r.Post("/clear_error_log", php.ClearErrorLog)
		r.Post("/clear_slow_log", php.ClearSlowLog)
		r.Get("/extensions", php.ExtensionList)
		r.Post("/extensions", php.InstallExtension)
		r.Delete("/extensions", php.UninstallExtension)
	}
}

func (s *App) SetCli(c fiber.Ctx) error {
	if _, err := shell.Execf("ln -sf %s/server/php/%d/bin/php /usr/bin/php", app.Root, s.version); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) GetConfig(c fiber.Ctx) error {
	config, err := io.Read(fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, s.version))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(fmt.Sprintf("%s/server/php/%d/etc/php.ini", app.Root, s.version), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) GetFPMConfig(c fiber.Ctx) error {
	config, err := io.Read(fmt.Sprintf("%s/server/php/%d/etc/php-fpm.conf", app.Root, s.version))
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, config)
}

func (s *App) UpdateFPMConfig(c fiber.Ctx) error {
	req, err := service.Bind[UpdateConfig](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if err = io.Write(fmt.Sprintf("%s/server/php/%d/etc/php-fpm.conf", app.Root, s.version), req.Config, 0644); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) Load(c fiber.Ctx) error {
	var raw map[string]any
	client := resty.New().SetTimeout(10 * time.Second)
	_, err := client.R().SetResult(&raw).Get(fmt.Sprintf("http://127.0.0.1/phpfpm_status/%d?json", s.version))
	if err != nil {
		return service.Success(c, []types.NV{})
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

	return service.Success(c, loads)
}

func (s *App) ErrorLog(c fiber.Ctx) error {
	return service.Success(c, fmt.Sprintf("%s/server/php/%d/var/log/php-fpm.log", app.Root, s.version))
}

func (s *App) SlowLog(c fiber.Ctx) error {
	return service.Success(c, fmt.Sprintf("%s/server/php/%d/var/log/slow.log", app.Root, s.version))
}

func (s *App) ClearErrorLog(c fiber.Ctx) error {
	if _, err := shell.Execf("cat /dev/null > %s/server/php/%d/var/log/php-fpm.log", app.Root, s.version); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) ClearSlowLog(c fiber.Ctx) error {
	if _, err := shell.Execf("cat /dev/null > %s/server/php/%d/var/log/slow.log", app.Root, s.version); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) ExtensionList(c fiber.Ctx) error {
	extensions := s.getExtensions()
	raw, err := shell.Execf("%s/server/php/%d/bin/php -m", app.Root, s.version)
	if err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	extensionMap := make(map[string]*Extension)
	for i := range extensions {
		extensionMap[extensions[i].Slug] = &extensions[i]
	}

	rawExtensionList := strings.Split(raw, "\n")
	for _, item := range rawExtensionList {
		if ext, exists := extensionMap[item]; exists && !strings.Contains(item, "[") && item != "" {
			ext.Installed = true
		}
	}

	return service.Success(c, extensions)
}

func (s *App) InstallExtension(c fiber.Ctx) error {
	req, err := service.Bind[ExtensionSlug](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if !s.checkExtension(req.Slug) {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("extension %s does not exist", req.Slug))
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://dl.cdn.haozi.net/panel/php_exts/%s.sh' | bash -s -- 'install' '%d' >> '/tmp/%s.log' 2>&1`, url.PathEscape(req.Slug), s.version, req.Slug)
	officials := []string{"fileinfo", "exif", "imap", "pgsql", "pdo_pgsql", "zip", "bz2", "readline", "snmp", "ldap", "enchant", "pspell", "calendar", "gmp", "sysvmsg", "sysvsem", "sysvshm", "xsl", "intl", "gettext"}
	if slices.Contains(officials, req.Slug) {
		cmd = fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://dl.cdn.haozi.net/panel/php_exts/official.sh' | bash -s -- 'install' '%d' '%s' >> '/tmp/%s.log' 2>&1`, s.version, req.Slug, req.Slug)
	}

	task := new(biz.Task)
	task.Name = s.t.Get("Install PHP-%d %s extension", s.version, req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	task.Log = "/tmp/" + req.Slug + ".log"
	if err = s.taskRepo.Push(task); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) UninstallExtension(c fiber.Ctx) error {
	req, err := service.Bind[ExtensionSlug](c)
	if err != nil {
		return service.Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	if !s.checkExtension(req.Slug) {
		return service.Error(c, http.StatusUnprocessableEntity, s.t.Get("extension %s does not exist", req.Slug))
	}

	cmd := fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://dl.cdn.haozi.net/panel/php_exts/%s.sh' | bash -s -- 'uninstall' '%d' >> '/tmp/%s.log' 2>&1`, url.PathEscape(req.Slug), s.version, req.Slug)
	officials := []string{"fileinfo", "exif", "imap", "pgsql", "pdo_pgsql", "zip", "bz2", "readline", "snmp", "ldap", "enchant", "pspell", "calendar", "gmp", "sysvmsg", "sysvsem", "sysvshm", "xsl", "intl", "gettext"}
	if slices.Contains(officials, req.Slug) {
		cmd = fmt.Sprintf(`curl -sSLm 10 --retry 3 'https://dl.cdn.haozi.net/panel/php_exts/official.sh' | bash -s -- 'uninstall' '%d' '%s' >> '/tmp/%s.log' 2>&1`, s.version, req.Slug, req.Slug)
	}

	task := new(biz.Task)
	task.Name = s.t.Get("Uninstall PHP-%d %s extension", s.version, req.Slug)
	task.Status = biz.TaskStatusWaiting
	task.Shell = cmd
	task.Log = "/tmp/" + req.Slug + ".log"
	if err = s.taskRepo.Push(task); err != nil {
		return service.Error(c, http.StatusInternalServerError, "%v", err)
	}

	return service.Success(c, nil)
}

func (s *App) getExtensions() []Extension {
	extensions := []Extension{
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
			Description: s.t.Get("PhpRedis connects to and operates on data in Redis databases (requires the igbinary extension installed above)"),
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
			Description: s.t.Get("IMAP extension allows PHP to read, search, delete, download, and manage emails"),
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
			Description: s.t.Get("Swoole is a PHP extension for building high-performance asynchronous concurrent servers"),
		},
	}

	// Swow 不支持 PHP 8.0 以下版本且目前不支持 PHP 8.4
	if cast.ToUint(s.version) >= 80 && cast.ToUint(s.version) < 84 {
		extensions = append(extensions, Extension{
			Name:        "Swow",
			Slug:        "Swow",
			Description: s.t.Get("Swow is a PHP extension for building high-performance asynchronous concurrent servers"),
		})
	}
	// PHP 8.4 移除了 pspell 和 imap 并且不再建议使用
	if cast.ToUint(s.version) >= 84 {
		extensions = slices.DeleteFunc(extensions, func(extension Extension) bool {
			return extension.Slug == "pspell" || extension.Slug == "imap"
		})
	}

	raw, _ := shell.Execf("%s/server/php/%d/bin/php -m", app.Root, s.version)
	extensionMap := make(map[string]*Extension)
	for i := range extensions {
		extensionMap[extensions[i].Slug] = &extensions[i]
	}

	rawExtensionList := strings.Split(raw, "\n")
	for _, item := range rawExtensionList {
		if ext, exists := extensionMap[item]; exists && !strings.Contains(item, "[") && item != "" {
			ext.Installed = true
		}
	}

	return extensions
}

func (s *App) checkExtension(slug string) bool {
	extensions := s.getExtensions()

	for _, item := range extensions {
		if item.Slug == slug {
			return true
		}
	}

	return false
}
