package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"go.yaml.in/yaml/v4"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
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

func NewApp(
	t *gotext.Locale,
	log *slog.Logger,
	databaseServerRepo biz.DatabaseServerRepo,
	settingRepo biz.SettingRepo,
) *App {
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
	raw, err := shell.Execf(r.Context(), `mongosh --quiet --eval "JSON.stringify(db.serverStatus())" -u admin -p %s --authenticationDatabase admin 2>/dev/null`, shell.Quote(password))
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
	common.SaveConfig(w, r, s.log, s.t, "mongodb", s.configPath(), 0644, common.Restart("mongod"))
}

// GetConfigTune 获取配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	cfg, _ := confval.LoadYAML(s.configPath())

	tune := ConfigTune{
		DbPath:        confval.GetYAML(cfg, "storage.dbPath"),
		CacheSizeGB:   confval.GetYAML(cfg, "storage.wiredTiger.engineConfig.cacheSizeGB"),
		Port:          confval.GetYAML(cfg, "net.port"),
		BindIp:        confval.GetYAML(cfg, "net.bindIp"),
		SystemLogPath: confval.GetYAML(cfg, "systemLog.path"),
		Authorization: confval.GetYAML(cfg, "security.authorization"),
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

	cfg, err := confval.LoadYAML(s.configPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	confval.SetYAMLNested(cfg, "storage.dbPath", req.DbPath)
	confval.SetYAMLNested(cfg, "storage.wiredTiger.engineConfig.cacheSizeGB", req.CacheSizeGB)
	confval.SetYAMLNested(cfg, "net.port", req.Port)
	confval.SetYAMLNested(cfg, "net.bindIp", req.BindIp)
	confval.SetYAMLNested(cfg, "systemLog.path", req.SystemLogPath)
	confval.SetYAMLNested(cfg, "security.authorization", req.Authorization)

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

	port := cast.ToUint(confval.GetYAML(cfg, "net.port"))
	if port == 0 {
		port = 27017
	}
	if err = s.databaseServerRepo.UpdatePort("local_mongodb", port); err != nil {
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

	// 新密码以 JSON 字面量嵌进脚本，整段脚本再整体引用，密码里的引号与 $ 都不会被 JS 或 shell 解释
	literal, err := json.Marshal(req.Password)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	oldPassword, _ := s.settingRepo.Get(biz.SettingKeyMongoDBAdminPassword)

	// 回退路径会临时起一个无认证 mongod，中途取消轻则服务不起，重则留下裸奔实例
	ctx := context.WithoutCancel(r.Context())

	// 尝试用旧密码连接修改
	_, err = shell.Execf(ctx, `mongosh --quiet -u admin -p %s --authenticationDatabase admin --eval %s`, shell.Quote(oldPassword), shell.Quote(fmt.Sprintf("db.changeUserPassword('admin', %s)", literal)))
	fallback := err != nil
	if fallback {
		// 回退：停止服务，无认证模式修改
		_ = systemctl.Stop(ctx, "mongod")
		_, _ = shell.Execf(ctx, `su -s /bin/bash mongod -c "mongod --config %s --noauth --fork --logpath /tmp/mongod_reset.log"`, s.configPath())
		_, resetErr := shell.Execf(ctx, `mongosh --quiet --eval %s`, shell.Quote(fmt.Sprintf("db.getSiblingDB('admin').changeUserPassword('admin', %s)", literal)))
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
