package docker

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t   *gotext.Locale
	log *slog.Logger
}

func NewApp(t *gotext.Locale, log *slog.Logger) *App {
	return &App{
		t:   t,
		log: log,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/settings", s.GetSettings)
	r.Post("/settings", s.UpdateSettings)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "docker")
	return types.AggregateAppStatus(ok)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "docker"), slog.String("path", "/etc/docker/daemon.json"))
	common.ServeConfig(w, "/etc/docker/daemon.json")
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "docker", "/etc/docker/daemon.json", 0644, "docker")
}

// GetSettings 获取 Docker 设置
func (s *App) GetSettings(w http.ResponseWriter, r *http.Request) {
	configPath := "/etc/docker/daemon.json"

	content, err := io.Read(configPath)
	if err != nil && !os.IsNotExist(err) {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "docker"), slog.String("path", configPath))

	var daemonConfig DaemonConfig
	if err = json.Unmarshal([]byte(content), &daemonConfig); err != nil {
		service.Success(w, Settings{}) // 配置文件可能不存在、为空或格式错误，返回默认设置
		return
	}

	settings := Settings{
		RegistryMirrors:    daemonConfig.RegistryMirrors,
		InsecureRegistries: daemonConfig.InsecureRegistries,
		LiveRestore:        daemonConfig.LiveRestore,
		LogDriver:          daemonConfig.LogDriver,
		Hosts:              daemonConfig.Hosts,
		DataRoot:           daemonConfig.DataRoot,
		StorageDriver:      daemonConfig.StorageDriver,
		DNS:                daemonConfig.DNS,
		FirewallBackend:    daemonConfig.FirewallBackend,
		Iptables:           daemonConfig.Iptables,
		Ip6tables:          daemonConfig.Ip6tables,
		IpForward:          daemonConfig.IpForward,
		IPv6:               daemonConfig.IPv6,
		Bip:                daemonConfig.Bip,
	}

	// 解析 log-opts
	if daemonConfig.LogOpts != nil {
		settings.LogOpts = LogOpts{
			MaxSize: daemonConfig.LogOpts["max-size"],
			MaxFile: daemonConfig.LogOpts["max-file"],
		}
	}

	// 从 exec-opts 中提取 cgroup-driver
	for _, opt := range daemonConfig.ExecOpts {
		if after, ok := strings.CutPrefix(opt, "native.cgroupdriver="); ok {
			settings.CgroupDriver = after
			break
		}
	}

	service.Success(w, settings)
}

// UpdateSettings 更新 Docker 设置
func (s *App) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateSettings](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	configPath := "/etc/docker/daemon.json"
	settings := req.Settings

	// 读取现有配置
	var existingConfig map[string]any
	content, err := io.Read(configPath)
	if err == nil && content != "" {
		if err = json.Unmarshal([]byte(content), &existingConfig); err != nil {
			existingConfig = make(map[string]any)
		}
	} else {
		existingConfig = make(map[string]any)
	}

	// 更新设置字段
	if len(settings.RegistryMirrors) > 0 {
		existingConfig["registry-mirrors"] = settings.RegistryMirrors
	} else {
		delete(existingConfig, "registry-mirrors")
	}

	if len(settings.InsecureRegistries) > 0 {
		existingConfig["insecure-registries"] = settings.InsecureRegistries
	} else {
		delete(existingConfig, "insecure-registries")
	}

	if settings.LiveRestore {
		existingConfig["live-restore"] = true
	} else {
		delete(existingConfig, "live-restore")
	}

	if settings.LogDriver != "" {
		existingConfig["log-driver"] = settings.LogDriver
	} else {
		delete(existingConfig, "log-driver")
	}

	// 日志配置
	if settings.LogOpts.MaxSize != "" || settings.LogOpts.MaxFile != "" {
		logOpts := make(map[string]string)
		if settings.LogOpts.MaxSize != "" {
			logOpts["max-size"] = settings.LogOpts.MaxSize
		}
		if settings.LogOpts.MaxFile != "" {
			logOpts["max-file"] = settings.LogOpts.MaxFile
		}
		existingConfig["log-opts"] = logOpts
	} else {
		delete(existingConfig, "log-opts")
	}

	// cgroup-driver
	if settings.CgroupDriver != "" {
		existingConfig["exec-opts"] = []string{"native.cgroupdriver=" + settings.CgroupDriver}
	} else {
		delete(existingConfig, "exec-opts")
	}

	if len(settings.Hosts) > 0 {
		existingConfig["hosts"] = settings.Hosts
	} else {
		delete(existingConfig, "hosts")
	}

	if settings.DataRoot != "" {
		existingConfig["data-root"] = settings.DataRoot
	} else {
		delete(existingConfig, "data-root")
	}

	if settings.StorageDriver != "" {
		existingConfig["storage-driver"] = settings.StorageDriver
	} else {
		delete(existingConfig, "storage-driver")
	}

	if len(settings.DNS) > 0 {
		existingConfig["dns"] = settings.DNS
	} else {
		delete(existingConfig, "dns")
	}

	// 防火墙后端
	if settings.FirewallBackend != "" {
		existingConfig["firewall-backend"] = settings.FirewallBackend
	} else {
		delete(existingConfig, "firewall-backend")
	}

	if settings.Iptables != nil {
		existingConfig["iptables"] = *settings.Iptables
	} else {
		delete(existingConfig, "iptables")
	}

	if settings.Ip6tables != nil {
		existingConfig["ip6tables"] = *settings.Ip6tables
	} else {
		delete(existingConfig, "ip6tables")
	}

	if settings.IpForward != nil {
		existingConfig["ip-forward"] = *settings.IpForward
	} else {
		delete(existingConfig, "ip-forward")
	}

	if settings.IPv6 != nil {
		existingConfig["ipv6"] = *settings.IPv6
	} else {
		delete(existingConfig, "ipv6")
	}

	if settings.Bip != "" {
		existingConfig["bip"] = settings.Bip
	} else {
		delete(existingConfig, "bip")
	}

	// 序列化并写入文件
	newContent, err := json.MarshalIndent(existingConfig, "", "  ")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = io.Write(configPath, string(newContent), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "docker"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "docker"), slog.String("path", configPath), slog.String("data_root", settings.DataRoot), slog.String("storage_driver", settings.StorageDriver), slog.String("cgroup_driver", settings.CgroupDriver), slog.String("firewall_backend", settings.FirewallBackend))

	service.Success(w, nil)
}
