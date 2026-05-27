package service

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/utils/collect"
	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

type HomeService struct {
	t                  *gotext.Locale
	api                *api.API
	conf               *config.Config
	taskRepo           biz.TaskRepo
	websiteRepo        biz.WebsiteRepo
	projectRepo        biz.ProjectRepo
	appRepo            biz.AppRepo
	environmentRepo    biz.EnvironmentRepo
	settingRepo        biz.SettingRepo
	databaseServerRepo biz.DatabaseServerRepo
	cronRepo           biz.CronRepo
	backupRepo         biz.BackupRepo
	containerRepo      biz.ContainerRepo
}

func NewHomeService(t *gotext.Locale, conf *config.Config, task biz.TaskRepo, website biz.WebsiteRepo, project biz.ProjectRepo, appRepo biz.AppRepo, environment biz.EnvironmentRepo, setting biz.SettingRepo, databaseServer biz.DatabaseServerRepo, cron biz.CronRepo, backupRepo biz.BackupRepo, container biz.ContainerRepo) *HomeService {
	return &HomeService{
		t:                  t,
		api:                api.NewAPI(app.Version, app.Locale),
		conf:               conf,
		taskRepo:           task,
		websiteRepo:        website,
		projectRepo:        project,
		appRepo:            appRepo,
		environmentRepo:    environment,
		settingRepo:        setting,
		databaseServerRepo: databaseServer,
		cronRepo:           cron,
		backupRepo:         backupRepo,
		containerRepo:      container,
	}
}

func (s *HomeService) Panel(w http.ResponseWriter, r *http.Request) {
	name, _ := s.settingRepo.Get(biz.SettingKeyName)
	if name == "" {
		name = s.t.Get("AcePanel")
	}
	hiddenMenu, _ := s.settingRepo.GetSlice(biz.SettingHiddenMenu)
	customLogo, _ := s.settingRepo.Get(biz.SettingKeyCustomLogo)

	Success(w, chix.M{
		"name":        name,
		"locale":      s.conf.App.Locale,
		"hidden_menu": hiddenMenu,
		"custom_logo": customLogo,
	})
}

func (s *HomeService) Apps(w http.ResponseWriter, r *http.Request) {
	apps, err := s.appRepo.GetHomeShow()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get home apps: %v", err))
		return
	}

	Success(w, apps)
}

func (s *HomeService) Current(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.HomeCurrent](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	Success(w, tools.CurrentInfo(req.Nets, req.Disks))
}

func (s *HomeService) SystemInfo(w http.ResponseWriter, r *http.Request) {
	hostInfo, err := host.Info()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get system info: %v", err))
		return
	}

	// 所有网卡名称
	netInterfaces, _ := net.Interfaces()
	nets := lo.Map(netInterfaces, func(v net.Interface, _ int) types.LV {
		return types.LV{Value: v.Name, Label: v.Name}
	})
	// 所有硬盘名称
	partitions, _ := disk.Partitions(false)
	disks := lo.Map(partitions, func(v disk.PartitionStat, _ int) types.LV {
		return types.LV{Value: v.Device, Label: fmt.Sprintf("%s (%s)", v.Device, v.Mountpoint)}
	})

	// 系统是否支持
	osSupported := true
	if os.IsRHEL() {
		minVer := version.Must(version.NewVersion("9.0"))
		currentVerStr := strings.Split(hostInfo.PlatformVersion, " ")[0]
		currentVer, err := version.NewVersion(currentVerStr)
		if err != nil || currentVer.LessThan(minVer) {
			osSupported = false
		}
	}
	if os.IsDebian() {
		minVer := version.Must(version.NewVersion("12.0"))
		currentVerStr := strings.Split(hostInfo.PlatformVersion, " ")[0]
		currentVer, err := version.NewVersion(currentVerStr)
		if err != nil || currentVer.LessThan(minVer) {
			osSupported = false
		}
	}
	if os.IsUbuntu() {
		minVer := version.Must(version.NewVersion("22.04"))
		currentVerStr := strings.Split(hostInfo.PlatformVersion, " ")[0]
		currentVer, err := version.NewVersion(currentVerStr)
		if err != nil || currentVer.LessThan(minVer) {
			osSupported = false
		}
	}

	Success(w, chix.M{
		"procs":          hostInfo.Procs,
		"hostname":       hostInfo.Hostname,
		"panel_version":  app.Version,
		"commit_hash":    app.CommitHash,
		"build_id":       app.BuildID,
		"build_time":     app.BuildTime,
		"build_user":     app.BuildUser,
		"build_host":     app.BuildHost,
		"go_version":     app.GoVersion,
		"kernel_arch":    hostInfo.KernelArch,
		"kernel_version": hostInfo.KernelVersion,
		"os_name":        hostInfo.Platform + " " + hostInfo.PlatformVersion,
		"os_supported":   osSupported,
		"os_eol":         os.IsEOL(),
		"boot_time":      hostInfo.BootTime,
		"uptime":         hostInfo.Uptime,
		"nets":           nets,
		"disks":          disks,
	})
}

func (s *HomeService) CountInfo(w http.ResponseWriter, r *http.Request) {
	websiteCount, err := s.websiteRepo.Count()
	if err != nil {
		websiteCount = -1
	}

	mysqlInstalled, _ := s.appRepo.IsInstalled("slug IN ?", []string{"mysql", "mariadb", "percona"})
	postgresqlInstalled, _ := s.appRepo.IsInstalled("slug = ?", "postgresql")

	var databaseCount int
	if mysqlInstalled {
		rootPassword, _ := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
		mysql, err := db.NewMySQL("root", rootPassword, "/tmp/mysql.sock", "unix")
		if err == nil {
			defer mysql.Close()
			databases, err := mysql.Databases()
			if err == nil {
				databaseCount += len(databases)
			}
		}
	}
	if postgresqlInstalled {
		server, err := s.databaseServerRepo.GetByName("local_postgresql")
		if err == nil {
			postgres, err := db.NewPostgres(server.Username, server.Password, server.Host, server.Port)
			if err == nil {
				defer postgres.Close()
				databases, err := postgres.Databases()
				if err == nil {
					databaseCount += len(databases)
				}
			}
		}
	}

	projectCount, err := s.projectRepo.Count()
	if err != nil {
		projectCount = -1
	}

	cronCount, err := s.cronRepo.Count()
	if err != nil {
		cronCount = -1
	}

	containerCount := -1
	containers, err := s.containerRepo.ListAll()
	if err == nil {
		containerCount = len(containers)
	}

	Success(w, chix.M{
		"website":   websiteCount,
		"database":  databaseCount,
		"project":   projectCount,
		"cron":      cronCount,
		"container": containerCount,
	})
}

func (s *HomeService) InstalledEnvironment(w http.ResponseWriter, r *http.Request) {
	mysqlInstalled, _ := s.appRepo.IsInstalled("slug IN ?", []string{"mysql", "mariadb", "percona"})
	postgresqlInstalled, _ := s.appRepo.IsInstalled("slug = ?", "postgresql")
	rsyncInstalled, _ := s.appRepo.IsInstalled("slug = ?", "rsync")

	// Go 版本
	goData := lo.Map(s.environmentRepo.InstalledSlugs("go"), func(slug string, _ int) types.LV {
		return types.LV{Value: slug, Label: fmt.Sprintf("Go %s", s.environmentRepo.InstalledVersion("go", slug))}
	})

	// Java 版本
	javaData := lo.Map(s.environmentRepo.InstalledSlugs("java"), func(slug string, _ int) types.LV {
		return types.LV{Value: slug, Label: fmt.Sprintf("Java %s", s.environmentRepo.InstalledVersion("java", slug))}
	})

	// Node.js 版本
	nodejsData := lo.Map(s.environmentRepo.InstalledSlugs("nodejs"), func(slug string, _ int) types.LV {
		return types.LV{Value: slug, Label: fmt.Sprintf("Node.js %s", s.environmentRepo.InstalledVersion("nodejs", slug))}
	})

	// PHP 版本
	phpData := lo.Map(s.environmentRepo.InstalledSlugs("php"), func(slug string, _ int) types.LVInt {
		return types.LVInt{Value: cast.ToInt(slug), Label: fmt.Sprintf("PHP %s", s.environmentRepo.InstalledVersion("php", slug))}
	})

	// Python 版本
	pythonData := lo.Map(s.environmentRepo.InstalledSlugs("python"), func(slug string, _ int) types.LV {
		return types.LV{Value: slug, Label: fmt.Sprintf("Python %s", s.environmentRepo.InstalledVersion("python", slug))}
	})

	// .NET 版本
	dotnetData := lo.Map(s.environmentRepo.InstalledSlugs("dotnet"), func(slug string, _ int) types.LV {
		return types.LV{Value: slug, Label: fmt.Sprintf(".NET %s", s.environmentRepo.InstalledVersion("dotnet", slug))}
	})

	// 数据库
	var dbData []types.LV
	dbData = append(dbData, types.LV{Value: "0", Label: s.t.Get("Not used")})
	if mysqlInstalled {
		dbData = append(dbData, types.LV{Value: "mysql", Label: "MySQL"})
	}
	if postgresqlInstalled {
		dbData = append(dbData, types.LV{Value: "postgresql", Label: "PostgreSQL"})
	}

	webserver, _ := s.settingRepo.Get(biz.SettingKeyWebserver)
	Success(w, chix.M{
		"webserver": webserver,
		"go":        goData,
		"java":      javaData,
		"nodejs":    nodejsData,
		"php":       phpData,
		"python":    pythonData,
		"dotnet":    dotnetData,
		"db":        dbData,
		"rsync":     rsyncInstalled,
	})
}

func (s *HomeService) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		Error(w, http.StatusForbidden, s.t.Get("unable to check for updates in offline mode"))
		return
	}

	current := app.Version
	channel, _ := s.settingRepo.Get(biz.SettingKeyChannel)
	latest, err := s.api.LatestVersion(channel)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the latest version: %v", err))
		return
	}

	v1, err := version.NewVersion(current)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse version: %v", err))
		return
	}
	v2, err := version.NewVersion(latest.Version)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse version: %v", err))
		return
	}
	if v1.GreaterThanOrEqual(v2) {
		Success(w, chix.M{
			"update": false,
		})
		return
	}

	Success(w, chix.M{
		"update": true,
	})
}

func (s *HomeService) UpdateInfo(w http.ResponseWriter, r *http.Request) {
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		Error(w, http.StatusForbidden, s.t.Get("unable to check for updates in offline mode"))
		return
	}

	current := app.Version
	channel, _ := s.settingRepo.Get(biz.SettingKeyChannel)
	latest, err := s.api.LatestVersion(channel)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the latest version: %v", err))
		return
	}

	v1, err := version.NewVersion(current)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse version: %v", err))
		return
	}
	v2, err := version.NewVersion(latest.Version)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to parse version: %v", err))
		return
	}
	if v1.GreaterThanOrEqual(v2) {
		Error(w, http.StatusInternalServerError, s.t.Get("the current version is the latest version"))
		return
	}

	versions, err := s.api.IntermediateVersions(channel)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the update information: %v", err))
		return
	}

	Success(w, versions)
}

func (s *HomeService) Update(w http.ResponseWriter, r *http.Request) {
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		Error(w, http.StatusForbidden, s.t.Get("unable to update in offline mode"))
		return
	}

	if s.taskRepo.HasRunningTask() {
		Error(w, http.StatusInternalServerError, s.t.Get("background task is running, updating is prohibited, please try again later"))
		return
	}

	channel, _ := s.settingRepo.Get(biz.SettingKeyChannel)
	panel, err := s.api.LatestVersion(channel)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the latest version: %v", err))
		return
	}

	download := collect.First(panel.Downloads)
	if download == nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the latest version download link"))
		return
	}

	url := fmt.Sprintf("https://%s%s", s.conf.App.DownloadEndpoint, download.URL)
	checksum := fmt.Sprintf("https://%s%s", s.conf.App.DownloadEndpoint, download.Checksum)

	app.Status = app.StatusUpgrade
	if err = s.backupRepo.UpdatePanel(panel.Version, url, checksum); err != nil {
		app.Status = app.StatusFailed
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	app.Status = app.StatusNormal
	Success(w, nil)
	tools.RestartPanel()
}

func (s *HomeService) Restart(w http.ResponseWriter, r *http.Request) {
	if s.taskRepo.HasRunningTask() {
		Error(w, http.StatusInternalServerError, s.t.Get("background task is running, restart is prohibited, please try again later"))
		return
	}

	tools.RestartPanel()
	Success(w, nil)
}

func (s *HomeService) RestartServer(w http.ResponseWriter, r *http.Request) {
	if s.taskRepo.HasRunningTask() {
		Error(w, http.StatusInternalServerError, s.t.Get("background task is running, restart is prohibited, please try again later"))
		return
	}

	tools.RestartServer()
	Success(w, nil)
}

func (s *HomeService) TopProcesses(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.HomeTopProcesses](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	tp := tools.CollectTopProcesses()
	switch req.Type {
	case "cpu":
		Success(w, tp.CPU)
	case "memory":
		Success(w, tp.Memory)
	case "disk_io":
		Success(w, tp.DiskIO)
	default:
		Error(w, http.StatusUnprocessableEntity, s.t.Get("invalid type"))
	}
}

// RuntimeInfo 返回 Go Runtime 统计数据
func (s *HomeService) RuntimeInfo(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	Success(w, chix.M{
		"uptime":          time.Since(app.StartTime).Seconds(),
		"goroutines":      runtime.NumGoroutine(),
		"go_version":      runtime.Version(),
		"num_cpu":         runtime.NumCPU(),
		"num_cgo_call":    runtime.NumCgoCall(),
		"memory_alloc":    mem.Alloc,
		"memory_total":    mem.TotalAlloc,
		"memory_sys":      mem.Sys,
		"memory_lookups":  mem.Lookups,
		"memory_mallocs":  mem.Mallocs,
		"memory_frees":    mem.Frees,
		"heap_alloc":      mem.HeapAlloc,
		"heap_sys":        mem.HeapSys,
		"heap_idle":       mem.HeapIdle,
		"heap_inuse":      mem.HeapInuse,
		"heap_released":   mem.HeapReleased,
		"heap_objects":    mem.HeapObjects,
		"stack_inuse":     mem.StackInuse,
		"stack_sys":       mem.StackSys,
		"mspan_inuse":     mem.MSpanInuse,
		"mspan_sys":       mem.MSpanSys,
		"mcache_inuse":    mem.MCacheInuse,
		"mcache_sys":      mem.MCacheSys,
		"buck_hash_sys":   mem.BuckHashSys,
		"gc_sys":          mem.GCSys,
		"other_sys":       mem.OtherSys,
		"gc_next":         mem.NextGC,
		"gc_last":         mem.LastGC,
		"gc_pause_total":  mem.PauseTotalNs,
		"gc_num":          mem.NumGC,
		"gc_num_forced":   mem.NumForcedGC,
		"gc_cpu_fraction": mem.GCCPUFraction,
	})
}

// GoroutineInfo Goroutine 信息
type GoroutineInfo struct {
	ID    int    `json:"id"`
	State string `json:"state"`
	Stack string `json:"stack"`
}

// goroutineHeaderRe 匹配 goroutine 头部行
var goroutineHeaderRe = regexp.MustCompile(`^goroutine (\d+) \[([^\]]+)\]:$`)

// Goroutines 返回所有 Goroutine 堆栈信息
func (s *HomeService) Goroutines(w http.ResponseWriter, r *http.Request) {
	buf := make([]byte, 1<<16) // 64KB 起步
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, len(buf)*2)
	}

	raw := string(buf)
	goroutines := lo.FilterMap(strings.Split(raw, "\n\n"), func(block string, _ int) (GoroutineInfo, bool) {
		block = strings.TrimSpace(block)
		if block == "" {
			return GoroutineInfo{}, false
		}
		lines := strings.SplitN(block, "\n", 2)
		matches := goroutineHeaderRe.FindStringSubmatch(lines[0])
		if matches == nil {
			return GoroutineInfo{}, false
		}
		id, _ := strconv.Atoi(matches[1])
		stack := ""
		if len(lines) > 1 {
			stack = lines[1]
		}
		return GoroutineInfo{ID: id, State: matches[2], Stack: stack}, true
	})

	Success(w, goroutines)
}
