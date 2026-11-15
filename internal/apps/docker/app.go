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
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

// cgroupDriverOpt exec-opts 里指定 cgroup 驱动的选项前缀
const cgroupDriverOpt = "native.cgroupdriver="

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
	common.SaveConfig(w, r, s.log, s.t, "docker", "/etc/docker/daemon.json", 0644, common.Restart("docker"))
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
		if after, ok := strings.CutPrefix(opt, cgroupDriverOpt); ok {
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

	// 配置文件可能不存在、为空或格式错误，都按空配置处理
	config := make(map[string]any)
	if content, readErr := io.Read(configPath); readErr == nil {
		_ = json.Unmarshal([]byte(content), &config)
	}
	if config == nil {
		config = make(map[string]any)
	}

	s.applySettings(config, settings)

	newContent, err := json.MarshalIndent(config, "", "  ")
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

// applySettings 把面板里的设置合并进 daemon.json 的配置
func (s *App) applySettings(config map[string]any, settings Settings) {
	// 未设置的项从文件里删掉，不留空值
	set := func(m map[string]any, key string, value any, keep bool) {
		if keep {
			m[key] = value
		} else {
			delete(m, key)
		}
	}

	set(config, "registry-mirrors", settings.RegistryMirrors, len(settings.RegistryMirrors) > 0)
	set(config, "insecure-registries", settings.InsecureRegistries, len(settings.InsecureRegistries) > 0)
	set(config, "live-restore", true, settings.LiveRestore)
	set(config, "log-driver", settings.LogDriver, settings.LogDriver != "")
	set(config, "hosts", settings.Hosts, len(settings.Hosts) > 0)
	set(config, "data-root", settings.DataRoot, settings.DataRoot != "")
	set(config, "storage-driver", settings.StorageDriver, settings.StorageDriver != "")
	set(config, "dns", settings.DNS, len(settings.DNS) > 0)
	set(config, "firewall-backend", settings.FirewallBackend, settings.FirewallBackend != "")
	set(config, "iptables", settings.Iptables, settings.Iptables != nil)
	set(config, "ip6tables", settings.Ip6tables, settings.Ip6tables != nil)
	set(config, "ip-forward", settings.IpForward, settings.IpForward != nil)
	set(config, "ipv6", settings.IPv6, settings.IPv6 != nil)
	set(config, "bip", settings.Bip, settings.Bip != "")

	// log-opts 与 exec-opts 只动面板管理的子项，用户手写的其它子项保留
	logOpts, _ := config["log-opts"].(map[string]any)
	if logOpts == nil {
		logOpts = make(map[string]any)
	}
	set(logOpts, "max-size", settings.LogOpts.MaxSize, settings.LogOpts.MaxSize != "")
	set(logOpts, "max-file", settings.LogOpts.MaxFile, settings.LogOpts.MaxFile != "")
	set(config, "log-opts", logOpts, len(logOpts) > 0)

	execOpts := lo.Reject(cast.ToStringSlice(config["exec-opts"]), func(opt string, _ int) bool {
		return strings.HasPrefix(opt, cgroupDriverOpt)
	})
	if settings.CgroupDriver != "" {
		execOpts = append(execOpts, cgroupDriverOpt+settings.CgroupDriver)
	}
	set(config, "exec-opts", execOpts, len(execOpts) > 0)
}
