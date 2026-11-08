package redis

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/apps/common"
	"github.com/acepanel/panel/v3/internal/apps/confval"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

type App struct {
	t                  *gotext.Locale
	log                *slog.Logger
	databaseServerRepo biz.DatabaseServerRepo
	taskRepo           biz.TaskRepo
	slug               string // 服务名与配置目录名，如 redis、valkey
	name               string // 展示名，如 Redis、Valkey
}

func NewApp(t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, taskRepo biz.TaskRepo) *App {
	return New("redis", "Redis", t, log, databaseServerRepo, taskRepo)
}

func New(slug, name string, t *gotext.Locale, log *slog.Logger, databaseServerRepo biz.DatabaseServerRepo, taskRepo biz.TaskRepo) *App {
	return &App{
		t:                  t,
		log:                log,
		databaseServerRepo: databaseServerRepo,
		taskRepo:           taskRepo,
		slug:               slug,
		name:               name,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
	r.Get("/config_tune", s.GetConfigTune)
	r.Post("/config_tune", s.UpdateConfigTune)
	// 性能诊断
	r.Get("/slow_log", s.SlowLog)
	r.Post("/slow_log/reset", s.ResetSlowLog)
	r.Get("/clients", s.ClientList)
	r.Post("/clients/kill", s.KillClient)
	r.Get("/memory", s.MemoryStatus)
	r.Post("/bigkeys", s.ScanBigKeys)
}

func (s *App) Status(ctx context.Context) string {
	ok, _ := systemctl.Status(ctx, s.slug)
	return types.AggregateAppStatus(ok)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status(r.Context(), s.slug)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get %s status: %v", s.name, err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	config, err := io.Read(s.confPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	var env []string
	if password := confval.Directive.Get(config, "requirepass"); password != "" {
		env = append(env, "REDISCLI_AUTH="+password)
	}

	raw, err := shell.ExecfWithEnv(r.Context(), env, "%s-cli info", s.slug)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get %s info: %v", s.name, err))
		return
	}

	dataRaw := lo.SliceToMap(strings.Split(raw, "\n"), func(item string) (string, string) {
		parts := strings.Split(item, ":")
		if len(parts) != 2 {
			return "", ""
		}
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	})

	data := []types.NV{
		{Name: s.t.Get("TCP Port"), Value: dataRaw["tcp_port"]},
		{Name: s.t.Get("Uptime in Days"), Value: dataRaw["uptime_in_days"]},
		{Name: s.t.Get("Connected Clients"), Value: dataRaw["connected_clients"]},
		{Name: s.t.Get("Total Allocated Memory"), Value: dataRaw["used_memory_human"]},
		{Name: s.t.Get("Total Memory Usage"), Value: dataRaw["used_memory_rss_human"]},
		{Name: s.t.Get("Peak Memory Usage"), Value: dataRaw["used_memory_peak_human"]},
		{Name: s.t.Get("Memory Fragmentation Ratio"), Value: dataRaw["mem_fragmentation_ratio"]},
		{Name: s.t.Get("Total Connections Received"), Value: dataRaw["total_connections_received"]},
		{Name: s.t.Get("Total Commands Processed"), Value: dataRaw["total_commands_processed"]},
		{Name: s.t.Get("Commands Per Second"), Value: dataRaw["instantaneous_ops_per_sec"]},
		{Name: s.t.Get("Keyspace Hits"), Value: dataRaw["keyspace_hits"]},
		{Name: s.t.Get("Keyspace Misses"), Value: dataRaw["keyspace_misses"]},
		{Name: s.t.Get("Latest Fork Time (ms)"), Value: dataRaw["latest_fork_usec"]},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.confPath()))
	common.ServeConfig(w, s.confPath())
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	common.SaveConfig(w, r, s.log, s.t, s.slug, s.confPath(), 0644, s.slug)
}

// GetConfigTune 获取配置调整参数
func (s *App) GetConfigTune(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(s.confPath())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	tune := ConfigTune{
		Bind:            confval.Directive.Get(config, "bind"),
		Port:            confval.Directive.Get(config, "port"),
		Databases:       confval.Directive.Get(config, "databases"),
		Requirepass:     confval.Directive.Get(config, "requirepass"),
		Timeout:         confval.Directive.Get(config, "timeout"),
		TCPKeepalive:    confval.Directive.Get(config, "tcp-keepalive"),
		Maxmemory:       confval.Directive.Get(config, "maxmemory"),
		MaxmemoryPolicy: confval.Directive.Get(config, "maxmemory-policy"),
		Appendonly:      confval.Directive.Get(config, "appendonly"),
		Appendfsync:     confval.Directive.Get(config, "appendfsync"),
	}

	s.log.Info(s.t.Get("app config viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("path", s.confPath()))

	service.Success(w, tune)
}

// UpdateConfigTune 更新配置调整参数
func (s *App) UpdateConfigTune(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ConfigTune](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	confPath := s.confPath()
	config, err := io.Read(confPath)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	config = confval.Directive.Set(config, "bind", req.Bind)
	config = confval.Directive.Set(config, "port", req.Port)
	config = confval.Directive.Set(config, "databases", req.Databases)
	config = confval.Directive.Set(config, "requirepass", req.Requirepass)
	config = confval.Directive.Set(config, "timeout", req.Timeout)
	config = confval.Directive.Set(config, "tcp-keepalive", req.TCPKeepalive)
	config = confval.Directive.Set(config, "maxmemory", req.Maxmemory)
	config = confval.Directive.Set(config, "maxmemory-policy", req.MaxmemoryPolicy)
	config = confval.Directive.Set(config, "appendonly", req.Appendonly)
	config = confval.Directive.Set(config, "appendfsync", req.Appendfsync)

	if err = io.Write(confPath, config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart(r.Context(), s.slug); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 同步密码到数据库服务器记录
	_ = s.databaseServerRepo.UpdatePassword("local_"+s.slug, req.Requirepass)

	s.log.Info(s.t.Get("app config tune saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("bind", req.Bind), slog.String("port", req.Port), slog.String("databases", req.Databases), slog.String("maxmemory", req.Maxmemory), slog.String("maxmemory_policy", req.MaxmemoryPolicy), slog.String("appendonly", req.Appendonly), slog.String("appendfsync", req.Appendfsync))

	service.Success(w, nil)
}

// SlowLog 获取慢日志
func (s *App) SlowLog(w http.ResponseWriter, r *http.Request) {
	conn, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer conn.Close()

	reply, err := conn.Exec("SLOWLOG", "GET", 50)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	rows, err := redigo.Values(reply, nil)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	entries := make([]SlowLogEntry, 0, len(rows))
	for _, row := range rows {
		item, itemErr := redigo.Values(row, nil)
		if itemErr != nil || len(item) < 4 {
			continue
		}
		entry := SlowLogEntry{}
		entry.ID, _ = redigo.Int64(item[0], nil)
		if ts, tsErr := redigo.Int64(item[1], nil); tsErr == nil {
			entry.Time = time.Unix(ts, 0).Format(time.DateTime)
		}
		entry.DurationUs, _ = redigo.Int64(item[2], nil)
		if cmd, cmdErr := redigo.Strings(item[3], nil); cmdErr == nil {
			entry.Command = strings.Join(cmd, " ")
		}
		if len(item) > 4 {
			entry.Client, _ = redigo.String(item[4], nil)
		}
		entries = append(entries, entry)
	}

	s.log.Info(s.t.Get("app log viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.String("log", "slowlog"))

	service.Success(w, entries)
}

// ResetSlowLog 重置慢日志
func (s *App) ResetSlowLog(w http.ResponseWriter, r *http.Request) {
	conn, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer conn.Close()

	if _, err = conn.Exec("SLOWLOG", "RESET"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Warn(s.t.Get("app slow log reset"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug))

	service.Success(w, nil)
}

// ClientList 获取客户端连接列表
func (s *App) ClientList(w http.ResponseWriter, r *http.Request) {
	conn, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer conn.Close()

	raw, err := redigo.String(conn.Exec("CLIENT", "LIST"))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	clients := make([]Client, 0)
	for line := range strings.SplitSeq(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		fields := map[string]string{}
		for kv := range strings.FieldsSeq(line) {
			if key, value, found := strings.Cut(kv, "="); found {
				fields[key] = value
			}
		}
		clients = append(clients, Client{
			ID:   fields["id"],
			Addr: fields["addr"],
			Name: fields["name"],
			DB:   fields["db"],
			Age:  fields["age"],
			Idle: fields["idle"],
			Cmd:  fields["cmd"],
		})
	}

	service.Success(w, clients)
}

// KillClient 踢除客户端连接
func (s *App) KillClient(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[ClientKill](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	conn, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer conn.Close()

	killed, err := redigo.Int64(conn.Exec("CLIENT", "KILL", "ID", req.ID))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	if killed == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("client %d not found", req.ID))
		return
	}

	s.log.Warn(s.t.Get("app client killed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.Int64("client_id", req.ID))

	service.Success(w, nil)
}

// MemoryStatus 获取内存诊断信息
func (s *App) MemoryStatus(w http.ResponseWriter, r *http.Request) {
	conn, err := s.connect(r.Context())
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	defer conn.Close()

	memory := Memory{Items: make([]types.NV, 0)}
	memory.Doctor, err = redigo.String(conn.Exec("MEMORY", "DOCTOR"))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	// 空库与无问题时返回的是电影梗彩蛋文案，无展示价值
	if strings.HasPrefix(memory.Doctor, "Hi Sam,") || strings.Contains(memory.Doctor, "can't find any memory issue") {
		memory.Doctor = ""
	}

	// 未限制内存上限时给出预警
	if maxmemory, mmErr := redigo.Strings(conn.Exec("CONFIG", "GET", "maxmemory")); mmErr == nil && len(maxmemory) == 2 && maxmemory[1] == "0" {
		memory.NoLimit = true
	}

	reply, err := conn.Exec("MEMORY", "STATS")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	values, err := redigo.Values(reply, nil)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	stats := map[string]string{}
	for i := 0; i+1 < len(values); i += 2 {
		key, keyErr := redigo.String(values[i], nil)
		if keyErr != nil {
			continue
		}
		// 值可能为整数或字符串，嵌套数组（如 db.0）跳过
		if v, vErr := redigo.Int64(values[i+1], nil); vErr == nil {
			stats[key] = cast.ToString(v)
			continue
		}
		if v, vErr := redigo.String(values[i+1], nil); vErr == nil {
			stats[key] = v
		}
	}

	// 各版本键名存在差异，仅展示存在的指标
	items := []struct {
		key   string
		name  string
		bytes bool
	}{
		{"peak.allocated", s.t.Get("Peak Allocated"), true},
		{"total.allocated", s.t.Get("Total Allocated"), true},
		{"startup.allocated", s.t.Get("Startup Allocated"), true},
		{"dataset.bytes", s.t.Get("Dataset Size"), true},
		{"dataset.percentage", s.t.Get("Dataset Percentage"), false},
		{"keys.count", s.t.Get("Keys Count"), false},
		{"keys.bytes-per-key", s.t.Get("Bytes Per Key"), true},
		{"allocator-fragmentation.ratio", s.t.Get("Allocator Fragmentation Ratio"), false},
		{"fragmentation", s.t.Get("Fragmentation Ratio"), false},
	}
	for _, item := range items {
		value, ok := stats[item.key]
		if !ok {
			continue
		}
		if item.bytes {
			value = tools.FormatBytes(cast.ToFloat64(value))
		}
		memory.Items = append(memory.Items, types.NV{Name: item.name, Value: value})
	}

	service.Success(w, memory)
}

// ScanBigKeys 扫描大 Key（异步任务）
func (s *App) ScanBigKeys(w http.ResponseWriter, r *http.Request) {
	task := new(biz.Task)
	task.Key = s.slug + ":bigkeys"
	task.Name = s.t.Get("Scan %s big keys", s.name)
	task.Status = biz.TaskStatusWaiting
	task.Shell = fmt.Sprintf(`pw=$(sed -n 's/^requirepass[[:space:]]\+//p' %s | head -n 1); [ -n "$pw" ] && export REDISCLI_AUTH="$pw"; %s-cli --bigkeys`, shell.Quote(s.confPath()), s.slug)
	if err := s.taskRepo.Push(task); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("app big keys scan queued"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", s.slug), slog.Uint64("task_id", uint64(task.ID)))

	service.Success(w, nil)
}

// connect 从配置文件读取端口与密码建立连接
func (s *App) connect(ctx context.Context) (*db.Redis, error) {
	config, err := io.Read(s.confPath())
	if err != nil {
		return nil, err
	}
	port := confval.Directive.Get(config, "port")
	if port == "" {
		port = "6379"
	}
	password := confval.Directive.Get(config, "requirepass")

	return db.NewRedis(ctx, "", password, "127.0.0.1:"+port)
}

func (s *App) confPath() string {
	return filepath.Join(app.Root, "server", s.slug, s.slug+".conf")
}
