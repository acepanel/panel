package service

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/utils/collect"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/api"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/db"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/tools"
	"github.com/acepanel/panel/pkg/types"
)

type HomeService struct {
	t           *gotext.Locale
	api         *api.API
	conf        *config.Config
	taskRepo    biz.TaskRepo
	websiteRepo biz.WebsiteRepo
	appRepo     biz.AppRepo
	settingRepo biz.SettingRepo
	cronRepo    biz.CronRepo
	backupRepo  biz.BackupRepo
}

func NewHomeService(t *gotext.Locale, conf *config.Config, task biz.TaskRepo, website biz.WebsiteRepo, appRepo biz.AppRepo, setting biz.SettingRepo, cron biz.CronRepo, backupRepo biz.BackupRepo) *HomeService {
	return &HomeService{
		t:           t,
		api:         api.NewAPI(app.Version, app.Locale),
		conf:        conf,
		taskRepo:    task,
		websiteRepo: website,
		appRepo:     appRepo,
		settingRepo: setting,
		cronRepo:    cron,
		backupRepo:  backupRepo,
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
		"boot_time":      hostInfo.BootTime,
		"uptime":         hostInfo.Uptime,
		"nets":           nets,
		"disks":          disks,
	})
}

func (s *HomeService) CountInfo(w http.ResponseWriter, r *http.Request) {
	websiteCount, err := s.websiteRepo.Count()
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the total number of websites: %v", err))
		return
	}

	mysqlInstalled, _ := s.appRepo.IsInstalled("slug = ?", "mysql")
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
		postgres, err := db.NewPostgres("postgres", "", "127.0.0.1", 5432)
		if err == nil {
			defer postgres.Close()
			databases, err := postgres.Databases()
			if err == nil {
				databaseCount += len(databases)
			}
		}
	}

	var ftpCount int
	ftpInstalled, _ := s.appRepo.IsInstalled("slug = ?", "pureftpd")
	if ftpInstalled {
		listRaw, err := shell.Execf("pure-pw list")
		if len(listRaw) != 0 && err == nil {
			listArr := strings.Split(listRaw, "\n")
			ftpCount = len(listArr)
		}
	}

	cronCount, err := s.cronRepo.Count()
	if err != nil {
		cronCount = -1
	}

	Success(w, chix.M{
		"website":  websiteCount,
		"database": databaseCount,
		"ftp":      ftpCount,
		"cron":     cronCount,
	})
}

func (s *HomeService) InstalledDbAndPhp(w http.ResponseWriter, r *http.Request) {
	mysqlInstalled, _ := s.appRepo.IsInstalled("slug = ?", "mysql")
	postgresqlInstalled, _ := s.appRepo.IsInstalled("slug = ?", "postgresql")
	php, _ := s.appRepo.GetInstalledAll("slug like ?", "php%")

	var phpData []types.LVInt
	var dbData []types.LV
	phpData = append(phpData, types.LVInt{Value: 0, Label: s.t.Get("Not used")})
	dbData = append(dbData, types.LV{Value: "0", Label: s.t.Get("Not used")})
	for _, p := range php {
		// 过滤 phpmyadmin
		match := regexp.MustCompile(`php(\d+)`).FindStringSubmatch(p.Slug)
		if len(match) == 0 {
			continue
		}

		item, _ := s.appRepo.Get(p.Slug)
		phpData = append(phpData, types.LVInt{Value: cast.ToInt(strings.ReplaceAll(p.Slug, "php", "")), Label: item.Name})
	}

	if mysqlInstalled {
		dbData = append(dbData, types.LV{Value: "mysql", Label: "MySQL"})
	}
	if postgresqlInstalled {
		dbData = append(dbData, types.LV{Value: "postgresql", Label: "PostgreSQL"})
	}

	Success(w, chix.M{
		"php": phpData,
		"db":  dbData,
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
	ver, url, checksum := panel.Version, download.URL, download.Checksum

	app.Status = app.StatusUpgrade
	if err = s.backupRepo.UpdatePanel(ver, url, checksum); err != nil {
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
