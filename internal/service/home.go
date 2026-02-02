package service

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/utils/collect"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/api"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/db"
	"github.com/acepanel/panel/pkg/os"
	"github.com/acepanel/panel/pkg/tools"
	"github.com/acepanel/panel/pkg/types"
)

type HomeService struct {
	t               *gotext.Locale
	api             *api.API
	conf            *config.Config
	taskRepo        biz.TaskRepo
	websiteRepo     biz.WebsiteRepo
	projectRepo     biz.ProjectRepo
	appRepo         biz.AppRepo
	environmentRepo biz.EnvironmentRepo
	settingRepo     biz.SettingRepo
	cronRepo        biz.CronRepo
	backupRepo      biz.BackupRepo
}

func NewHomeService(t *gotext.Locale, conf *config.Config, task biz.TaskRepo, website biz.WebsiteRepo, project biz.ProjectRepo, appRepo biz.AppRepo, environment biz.EnvironmentRepo, setting biz.SettingRepo, cron biz.CronRepo, backupRepo biz.BackupRepo) *HomeService {
	return &HomeService{
		t:               t,
		api:             api.NewAPI(app.Version, app.Locale),
		conf:            conf,
		taskRepo:        task,
		websiteRepo:     website,
		projectRepo:     project,
		appRepo:         appRepo,
		environmentRepo: environment,
		settingRepo:     setting,
		cronRepo:        cron,
		backupRepo:      backupRepo,
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
	var nets []types.LV
	netInterfaces, _ := net.Interfaces()
	for _, v := range netInterfaces {
		nets = append(nets, types.LV{
			Value: v.Name,
			Label: v.Name,
		})
	}
	// 所有硬盘名称
	var disks []types.LV
	partitions, _ := disk.Partitions(false)
	for _, v := range partitions {
		disks = append(disks, types.LV{
			Value: v.Device,
			Label: fmt.Sprintf("%s (%s)", v.Device, v.Mountpoint),
		})
	}

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
		postgresPassword, _ := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
		postgres, err := db.NewPostgres("postgres", postgresPassword, "127.0.0.1", 5432)
		if err == nil {
			defer postgres.Close()
			databases, err := postgres.Databases()
			if err == nil {
				databaseCount += len(databases)
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

	Success(w, chix.M{
		"website":  websiteCount,
		"database": databaseCount,
		"project":  projectCount,
		"cron":     cronCount,
	})
}

func (s *HomeService) InstalledEnvironment(w http.ResponseWriter, r *http.Request) {
	mysqlInstalled, _ := s.appRepo.IsInstalled("slug IN ?", []string{"mysql", "mariadb", "percona"})
	postgresqlInstalled, _ := s.appRepo.IsInstalled("slug = ?", "postgresql")

	// Go 版本
	var goData []types.LV
	for _, slug := range s.environmentRepo.InstalledSlugs("go") {
		ver := s.environmentRepo.InstalledVersion("go", slug)
		goData = append(goData, types.LV{Value: slug, Label: fmt.Sprintf("Go %s", ver)})
	}

	// Java 版本
	var javaData []types.LV
	for _, slug := range s.environmentRepo.InstalledSlugs("java") {
		ver := s.environmentRepo.InstalledVersion("java", slug)
		javaData = append(javaData, types.LV{Value: slug, Label: fmt.Sprintf("Java %s", ver)})
	}

	// Node.js 版本
	var nodejsData []types.LV
	for _, slug := range s.environmentRepo.InstalledSlugs("nodejs") {
		ver := s.environmentRepo.InstalledVersion("nodejs", slug)
		nodejsData = append(nodejsData, types.LV{Value: slug, Label: fmt.Sprintf("Node.js %s", ver)})
	}

	// PHP 版本
	var phpData []types.LVInt
	for _, slug := range s.environmentRepo.InstalledSlugs("php") {
		ver := s.environmentRepo.InstalledVersion("php", slug)
		phpData = append(phpData, types.LVInt{Value: cast.ToInt(slug), Label: fmt.Sprintf("PHP %s", ver)})
	}

	// Python 版本
	var pythonData []types.LV
	for _, slug := range s.environmentRepo.InstalledSlugs("python") {
		ver := s.environmentRepo.InstalledVersion("python", slug)
		pythonData = append(pythonData, types.LV{Value: slug, Label: fmt.Sprintf("Python %s", ver)})
	}

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
		"db":        dbData,
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
