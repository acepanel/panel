package route

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/wire"
	"github.com/libtnb/validator/contrib/openapi"

	"github.com/acepanel/panel/v3/internal/middleware"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/config"
)

var ProviderSet = wire.NewSet(wire.Struct(new(Services), "*"), NewEndpoints)

// Services 汇总所有路由所需的服务，Wire 会在生成期校验依赖完整性。
type Services struct {
	Alert                 *service.AlertService
	App                   *service.AppService
	Backup                *service.BackupService
	BackupStorage         *service.BackupStorageService
	Cert                  *service.CertService
	CertAccount           *service.CertAccountService
	CertDNS               *service.CertDNSService
	Container             *service.ContainerService
	ContainerCompose      *service.ContainerComposeService
	ContainerImage        *service.ContainerImageService
	ContainerNetwork      *service.ContainerNetworkService
	ContainerVolume       *service.ContainerVolumeService
	Cron                  *service.CronService
	Database              *service.DatabaseService
	DatabaseElasticsearch *service.DatabaseElasticsearchService
	DatabaseRedis         *service.DatabaseRedisService
	DatabaseServer        *service.DatabaseServerService
	DatabaseUser          *service.DatabaseUserService
	Environment           *service.EnvironmentService
	EnvironmentDotnet     *service.EnvironmentDotnetService
	EnvironmentGo         *service.EnvironmentGoService
	EnvironmentJava       *service.EnvironmentJavaService
	EnvironmentNodejs     *service.EnvironmentNodejsService
	EnvironmentPHP        *service.EnvironmentPHPService
	EnvironmentPython     *service.EnvironmentPythonService
	File                  *service.FileService
	FileShare             *service.FileShareService
	Firewall              *service.FirewallService
	FirewallScan          *service.FirewallScanService
	Home                  *service.HomeService
	Log                   *service.LogService
	Monitor               *service.MonitorService
	Notify                *service.NotifyService
	Process               *service.ProcessService
	Project               *service.ProjectService
	Safe                  *service.SafeService
	Setting               *service.SettingService
	SSH                   *service.SSHService
	Systemctl             *service.SystemctlService
	Tamper                *service.TamperService
	Task                  *service.TaskService
	Template              *service.TemplateService
	ToolboxBenchmark      *service.ToolboxBenchmarkService
	ToolboxDisk           *service.ToolboxDiskService
	ToolboxLog            *service.ToolboxLogService
	ToolboxMigration      *service.ToolboxMigrationService
	ToolboxNetwork        *service.ToolboxNetworkService
	ToolboxSSH            *service.ToolboxSSHService
	ToolboxSystem         *service.ToolboxSystemService
	User                  *service.UserService
	UserPasskey           *service.UserPasskeyService
	UserToken             *service.UserTokenService
	WebHook               *service.WebHookService
	Website               *service.WebsiteService
	WebsiteStat           *service.WebsiteStatService
	Waf                   *service.WafService
	Ws                    *service.WsService
}

func NewEndpoints(s *Services) []Endpoints {
	return []Endpoints{
		UserRoutes(s.UserPasskey, s.User),
		UserPasskeyRoutes(s.UserPasskey),
		UserTokenRoutes(s.UserToken),
		SafeRoutes(s.Safe),
		TaskRoutes(s.Task),
		HomeRoutes(s.Home),
		WebsiteRoutes(s.Website),
		WebsiteStatRoutes(s.WebsiteStat),
		ProjectRoutes(s.Project),
		DatabaseRoutes(s.Database),
		DatabaseServerRoutes(s.DatabaseServer),
		DatabaseUserRoutes(s.DatabaseUser),
		DatabaseRedisRoutes(s.DatabaseRedis),
		DatabaseElasticsearchRoutes(s.DatabaseElasticsearch),
		CertRoutes(s.CertAccount, s.CertDNS, s.Cert),
		BackupRoutes(s.Backup),
		BackupStorageRoutes(s.BackupStorage),
		AppRoutes(s.App),
		EnvironmentRoutes(s.EnvironmentDotnet, s.EnvironmentGo, s.EnvironmentJava, s.EnvironmentNodejs, s.EnvironmentPHP, s.EnvironmentPython, s.Environment),
		ContainerRoutes(s.ContainerCompose, s.ContainerImage, s.ContainerNetwork, s.Container, s.ContainerVolume),
		FileRoutes(s.File),
		FileShareRoutes(s.FileShare),
		CronRoutes(s.Cron),
		ProcessRoutes(s.Process),
		FirewallRoutes(s.FirewallScan, s.Firewall),
		SSHRoutes(s.SSH),
		SystemctlRoutes(s.Systemctl),
		SettingRoutes(s.Setting),
		LogRoutes(s.Log),
		MonitorRoutes(s.Monitor),
		WebHookRoutes(s.WebHook),
		NotifyRoutes(s.Notify),
		AlertRoutes(s.Alert),
		TemplateRoutes(s.Template),
		ToolboxNetworkRoutes(s.ToolboxNetwork),
		ToolboxSystemRoutes(s.ToolboxSystem),
		ToolboxBenchmarkRoutes(s.ToolboxBenchmark),
		ToolboxSSHRoutes(s.ToolboxSSH),
		ToolboxDiskRoutes(s.ToolboxDisk),
		ToolboxLogRoutes(s.ToolboxLog),
		ToolboxMigrationRoutes(s.ToolboxMigration),
		TamperRoutes(s.Tamper),
		WafRoutes(s.Waf),
		WsRoutes(s.ToolboxMigration, s.Ws),
	}
}

// ThrottleRule 端点级限流规则。
type ThrottleRule struct {
	Tokens   uint64
	Interval time.Duration
}

// Endpoint 声明一个 HTTP 端点：如何服务，以及（经 Request/Response 样本）如何生成文档。
// 无 Request/Response 的端点（探针、WebSocket）不进 OpenAPI 文档。
type Endpoint struct {
	Method   string
	Path     string // 绝对路径，如 "/api/users"
	Handler  http.HandlerFunc
	Summary  string
	Tags     []string
	Request  any // request.* 样本
	Response any // service.Envelope[...] 样本
	Status   int
	Public   bool          // 登录白名单（MustLogin 放行）
	Throttle *ThrottleRule // 非 nil 时端点级限流
}

// Endpoints 是一个模块对 HTTP 路由的贡献。
type Endpoints []Endpoint

// HTTP 将全部路由贡献注册到 r。
func HTTP(conf *config.Config, groups []Endpoints, r chi.Router) {
	for _, endpoints := range groups {
		for _, e := range endpoints {
			if e.Throttle != nil {
				r.With(middleware.Throttle(conf.HTTP.IPHeader, e.Throttle.Tokens, e.Throttle.Interval)).Method(e.Method, e.Path, e.Handler)
			} else {
				r.Method(e.Method, e.Path, e.Handler)
			}
		}
	}
}

// PublicPaths 收集去重后的登录白名单路径，供 MustLogin 中间件放行。
func PublicPaths(groups []Endpoints) []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0)
	for _, endpoints := range groups {
		for _, e := range endpoints {
			if !e.Public {
				continue
			}
			if _, ok := seen[e.Path]; ok {
				continue
			}
			seen[e.Path] = struct{}{}
			paths = append(paths, e.Path)
		}
	}

	return paths
}

// SpecJSON 从每个带文档样本的端点组装 OpenAPI 3 文档。
func SpecJSON(groups []Endpoints, title string) ([]byte, error) {
	g := openapi.New(title, buildVersion(),
		openapi.WithType(time.Time{}, &openapi.Schema{Type: "string", Format: "date-time"}),
	)

	for _, endpoints := range groups {
		for _, e := range endpoints {
			if e.Request == nil && e.Response == nil {
				continue
			}
			if err := g.Add(e.Method, e.Path, openapi.Op{
				Summary:  e.Summary,
				Tags:     e.Tags,
				Request:  e.Request,
				Response: e.Response,
				Status:   e.Status,
			}); err != nil {
				return nil, err
			}
		}
	}

	return g.JSON()
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}
