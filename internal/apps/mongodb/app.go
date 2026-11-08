package mongodb

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"go.yaml.in/yaml/v4"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t                  *gotext.Locale
	log                *slog.Logger
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, settingRepo biz.SettingRepo) *App {
	return &App{
		t:                  t,
		log:                log,
		settingRepo:        settingRepo,
		databaseServerRepo: databaseServerRepo,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	r.Get("/admin_password", s.GetAdminPassword)
	r.Post("/admin_password", s.SetAdminPassword)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, "mongod")
	return types.AggregateAppStatus(ok)
}

// Load 获取 MongoDB 运行状态
func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, _ := systemctl.Status(r.Context(), "mongod")
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	password, _ := s.settingRepo.Get(biz.SettingKeyMongoDBAdminPassword)
	raw, err := shell.Execf(r.Context(), `mongosh --quiet --eval "JSON.stringify(db.serverStatus())" -u admin -p '%s' --authenticationDatabase admin 2>/dev/null`, password)
	if err != nil {
		service.Success(w, []types.NV{})
		return
	}

	var status2 struct {
		Uptime      int `json:"uptime"`
		Connections struct {
			Current      int `json:"current"`
			TotalCreated int `json:"totalCreated"`
		} `json:"connections"`
		OpCounters struct {
			Query  int `json:"query"`
			Insert int `json:"insert"`
			Update int `json:"update"`
			Delete int `json:"delete"`
		} `json:"opcounters"`
		Mem struct {
			Resident int `json:"resident"`
		} `json:"mem"`
		StorageEngine struct {
			Name string `json:"name"`
		} `json:"storageEngine"`
		Version string `json:"version"`
	}
	if err = json.Unmarshal([]byte(raw), &status2); err != nil {
		service.Success(w, []types.NV{})
		return
	}

	data := []types.NV{
		{Name: s.t.Get("Version"), Value: status2.Version},
		{Name: s.t.Get("Uptime (seconds)"), Value: cast.ToString(status2.Uptime)},
		{Name: s.t.Get("Current Connections"), Value: cast.ToString(status2.Connections.Current)},
		{Name: s.t.Get("Total Connections Created"), Value: cast.ToString(status2.Connections.TotalCreated)},
		{Name: s.t.Get("Query Operations"), Value: cast.ToString(status2.OpCounters.Query)},
		{Name: s.t.Get("Insert Operations"), Value: cast.ToString(status2.OpCounters.Insert)},
		{Name: s.t.Get("Update Operations"), Value: cast.ToString(status2.OpCounters.Update)},
		{Name: s.t.Get("Delete Operations"), Value: cast.ToString(status2.OpCounters.Delete)},
		{Name: s.t.Get("Resident Memory (MB)"), Value: cast.ToString(status2.Mem.Resident)},
		{Name: s.t.Get("Storage Engine"), Value: status2.StorageEngine.Name},
	}

	service.Success(w, data)
}

// GetConfig 获取配置
func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "mongodb"), slog.String("path", s.configPath()))
	common.ServeConfig(w, s.configPath())
}

// UpdateConfig 更新配置
func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, "mongodb", s.configPath(), 0644, "mongod")
}

// GetConfigTune 获取配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	_ = yaml.Unmarshal([]byte(raw), &cfg)
	if cfg == nil {
		cfg = make(map[string]any)
	}

	tune := ConfigTune{
		DbPath:        s.getYAMLValue(cfg, "storage.dbPath"),
		CacheSizeGB:   s.getYAMLValue(cfg, "storage.wiredTiger.engineConfig.cacheSizeGB"),
		Port:          s.getYAMLValue(cfg, "net.port"),
		BindIp:        s.getYAMLValue(cfg, "net.bindIp"),
		SystemLogPath: s.getYAMLValue(cfg, "systemLog.path"),
		Authorization: s.getYAMLValue(cfg, "security.authorization"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "mongodb"), slog.String("path", s.configPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	raw, _ := io.Read(s.configPath())
	var cfg map[string]any
	if err = yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		cfg = make(map[string]any)
	}

	s.setNestedYAMLValue(cfg, "storage.dbPath", req.DbPath)
	s.setNestedYAMLValue(cfg, "storage.wiredTiger.engineConfig.cacheSizeGB", req.CacheSizeGB)
	s.setNestedYAMLValue(cfg, "net.port", req.Port)
	s.setNestedYAMLValue(cfg, "net.bindIp", req.BindIp)
	s.setNestedYAMLValue(cfg, "systemLog.path", req.SystemLogPath)
	s.setNestedYAMLValue(cfg, "security.authorization", req.Authorization)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if err = io.Write(s.configPath(), string(data), 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), "mongod"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "mongodb"), slog.String("path", s.configPath()), slog.String("db_path", req.DbPath), slog.String("cache_size_gb", req.CacheSizeGB), slog.String("port", req.Port), slog.String("bind_ip", req.BindIp), slog.String("system_log_path", req.SystemLogPath), slog.String("authorization", req.Authorization))

	service.Success(w, nil)
}

// GetAdminPassword 获取 admin 密码
func (s *App) GetAdminPassword(w http.ResponseWriter, r *http.Request) {
	password, err := s.settingRepo.Get(biz.SettingKeyMongoDBAdminPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get MongoDB admin password: %v", err))
		return
	}

	s.log.Info(s.t.Get("app password viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "mongodb"), slog.String("username", "admin"))

	service.Success(w, password)
}

// SetAdminPassword 设置 admin 密码
func (s *App) SetAdminPassword(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[SetAdminPassword](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldPassword, _ := s.settingRepo.Get(biz.SettingKeyMongoDBAdminPassword)

	// 回退路径会临时起一个无认证 mongod，中途取消轻则服务不起，重则留下裸奔实例
	ctx := context.WithoutCancel(r.Context())

	// 尝试用旧密码连接修改
	_, err = shell.Execf(ctx, `mongosh --quiet -u admin -p '%s' --authenticationDatabase admin --eval "db.changeUserPassword('admin', '%s')"`, oldPassword, req.Password)
	fallback := err != nil
	if fallback {
		// 回退：停止服务，无认证模式修改
		_ = systemctl.Stop(ctx, "mongod")
		_, _ = shell.Execf(ctx, `su -s /bin/bash mongod -c "mongod --config %s --noauth --fork --logpath /tmp/mongod_reset.log"`, s.configPath())
		_, resetErr := shell.Execf(ctx, `mongosh --quiet --eval "db.getSiblingDB('admin').changeUserPassword('admin', '%s')"`, req.Password)
		_, _ = shell.Execf(ctx, `su -s /bin/bash mongod -c "mongod --config %s --shutdown" 2>/dev/null; pkill -f 'mongod --config.*--noauth' 2>/dev/null`, s.configPath())
		_ = systemctl.Start(ctx, "mongod")
		if resetErr != nil {
			service.Error(w, http.StatusInternalServerError, s.t.Get("failed to set MongoDB admin password: %v", resetErr))
			return
		}
	}

	if err = s.settingRepo.Set(biz.SettingKeyMongoDBAdminPassword, req.Password); err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to save MongoDB admin password: %v", err))
		return
	}

	_ = s.databaseServerRepo.UpdatePassword("local_mongodb", req.Password)

	s.log.Info(s.t.Get("app password updated"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", "mongodb"), slog.String("username", "admin"), slog.Bool("fallback", fallback))

	service.Success(w, nil)
}

// configPath 返回配置文件路径
func (s *App) configPath() string {
	return app.Root + "/server/mongodb/mongod.conf"
}

// getYAMLValue 获取嵌套 YAML 值，支持 dot notation
func (s *App) getYAMLValue(cfg map[string]any, key string) string {
	parts := strings.SplitN(key, ".", 2)
	val, ok := cfg[parts[0]]
	if !ok {
		return ""
	}
	if len(parts) == 1 {
		return cast.ToString(val)
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return ""
	}
	return s.getYAMLValue(nested, parts[1])
}

// setNestedYAMLValue 设置嵌套 YAML 值，逐层创建 map
func (s *App) setNestedYAMLValue(cfg map[string]any, key string, value string) {
	if value == "" {
		return
	}
	parts := strings.SplitN(key, ".", 2)
	if len(parts) == 1 {
		cfg[parts[0]] = value
		return
	}
	nested, ok := cfg[parts[0]].(map[string]any)
	if !ok {
		nested = make(map[string]any)
		cfg[parts[0]] = nested
	}
	s.setNestedYAMLValue(nested, parts[1], value)
}
