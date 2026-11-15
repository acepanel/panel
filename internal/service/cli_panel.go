package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"

	"github.com/libtnb/utils/str"
	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/ntp"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/tools"
)

func (s *CliService) Update(ctx context.Context, cmd *cli.Command) error {
	release, err := s.backupRepo.LatestPanel()
	if err != nil {
		return errors.New(s.t.Get("Failed to get latest version: %v", err))
	}
	if release.URL == "" {
		return errors.New(s.t.Get("Download URL is empty"))
	}

	if err = s.backupRepo.UpdatePanel(ctx, release, func(msg string) {
		fmt.Println("|-" + msg)
	}); err != nil {
		return err
	}
	tools.RestartPanel(ctx)
	return nil
}

func (s *CliService) Sync(ctx context.Context, cmd *cli.Command) error {
	if key, err := s.cacheRepo.UpdateAll(); err != nil {
		return errors.New(map[biz.CacheKey]string{
			biz.CacheKeyCategories:  s.t.Get("Failed to synchronize categories data: %v", err),
			biz.CacheKeyApps:        s.t.Get("Failed to synchronize app data: %v", err),
			biz.CacheKeyEnvironment: s.t.Get("Failed to synchronize environment data: %v", err),
			biz.CacheKeyTemplates:   s.t.Get("Failed to synchronize template data: %v", err),
		}[key])
	}

	fmt.Println(s.t.Get("Data synchronized successfully"))
	return nil
}

func (s *CliService) Fix(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.FixPanel(ctx)
}

func (s *CliService) Info(ctx context.Context, cmd *cli.Command) error {
	// 未指定用户名时取第一个用户，兼容单用户场景
	var user *biz.User
	var err error
	if username := cmd.String("username"); username != "" {
		user, err = s.userRepo.GetByUsername(username)
	} else {
		user, err = s.userRepo.First()
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("User not exists"))
		}
		return errors.New(s.t.Get("Failed to get user info: %v", err))
	}

	// 判断是否首次运行或使用了 -f 参数
	force := cmd.Bool("force")
	infoRan, _ := s.settingRepo.Get(biz.SettingKeyInfoRan)
	isFirstRun := infoRan == ""

	var password string
	if isFirstRun || force {
		password = str.Random(16)
		user.Username = str.Random(8)
		if err = s.userRepo.UpdateUsername(ctx, user.ID, user.Username); err != nil {
			return errors.New(s.t.Get("Failed to save user info: %v", err))
		}
		if err = s.userRepo.UpdatePassword(ctx, user.ID, password); err != nil {
			return errors.New(s.t.Get("Failed to save user info: %v", err))
		}

		// 标记 info 命令已运行过
		if err = s.settingRepo.Set(biz.SettingKeyInfoRan, "1"); err != nil {
			return errors.New(s.t.Get("Failed to save setting: %v", err))
		}
	}

	port := s.conf.HTTP.Port
	if port == 0 {
		return errors.New(s.t.Get("Failed to get port"))
	}
	entrance := s.conf.HTTP.Entrance
	if entrance == "" {
		return errors.New(s.t.Get("Failed to get entrance"))
	}

	fmt.Println(s.t.Get("Username: %s", user.Username))
	if isFirstRun || force {
		fmt.Println(s.t.Get("Password: %s", password))
	} else {
		fmt.Println(s.t.Get("Password: ******* (use -f to force reset)"))
	}
	fmt.Println(s.t.Get("Port: %d", port))
	fmt.Println(s.t.Get("Entrance: %s", entrance))
	s.printAccessAddresses() //nolint:contextcheck

	fmt.Println(s.t.Get("Please choose the appropriate address to access the panel based on your network situation"))
	fmt.Println(s.t.Get("If you cannot access, please check whether the server's security group and firewall allow port %d", port))
	fmt.Println(s.t.Get("If you still cannot access, try running `acepanel https off` to turn off panel HTTPS"))
	fmt.Println(s.t.Get("Warning: After turning off panel HTTPS, the security of the panel will be greatly reduced, please operate with caution"))

	return nil
}

func (s *CliService) Port(ctx context.Context, cmd *cli.Command) error {
	port := cmd.Uint("port")
	if port == 0 {
		return errors.New(s.t.Get("Please specify the port"))
	}
	if port > 65535 {
		return errors.New(s.t.Get("Port range error"))
	}

	conf, err := config.Load()
	if err != nil {
		return err
	}

	if port != conf.HTTP.Port {
		//nolint:contextcheck
		if os.TCPPortInUse(port) {
			return errors.New(s.t.Get("Port already in use"))
		}
	}

	conf.HTTP.Port = port

	// 放行端口，查询失败不能当作防火墙没运行，否则会静默跳过放行把面板关在防火墙外面
	fw := firewall.NewFirewall()
	running, err := fw.Status(ctx)
	if err != nil {
		return err
	}
	if running {
		if err = fw.Port(ctx, firewall.FireInfo{
			Type:      firewall.TypeNormal,
			PortStart: port,
			PortEnd:   port,
			Protocol:  firewall.ProtocolTCPUDP,
			Strategy:  firewall.StrategyAccept,
			Direction: firewall.DirectionIn,
		}, firewall.OperationAdd); err != nil {
			return err
		}
	}

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Port changed to %d", port))
	return s.Restart(ctx, cmd)
}

func (s *CliService) SyncTime(ctx context.Context, cmd *cli.Command) error {
	now, err := ntp.Now()
	if err != nil {
		return err
	}

	if err = ntp.UpdateSystemTime(ctx, now); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Time synchronized successfully"))
	return nil
}

func (s *CliService) ClearTask(ctx context.Context, cmd *cli.Command) error {
	if err := s.db.Model(&biz.Task{}).
		Where("status", biz.TaskStatusRunning).Or("status", biz.TaskStatusWaiting).
		Update("status", biz.TaskStatusFailed).
		Error; err != nil {
		return errors.New(s.t.Get("Failed to clear tasks: %v", err))
	}

	fmt.Println(s.t.Get("Tasks cleared successfully"))
	return nil
}

func (s *CliService) Init(ctx context.Context, cmd *cli.Command) error {
	var check biz.User
	if err := s.db.First(&check).Error; err == nil {
		return errors.New(s.t.Get("Already initialized"))
	}

	ips := make([]string, 0)
	acme := false
	rv6, err := tools.GetPublicIPv6()
	if err == nil {
		ips = append(ips, rv6)
		acme = true
	}
	rv4, err := tools.GetPublicIPv4()
	if err == nil {
		ips = append(ips, rv4)
		acme = true
	}
	ip, err := json.Marshal(ips)
	if err != nil {
		ip = []byte("[]")
	}

	settings := []biz.Setting{
		{Key: biz.SettingKeyPublicIPs, Value: string(ip)},
		{Key: biz.SettingKeyName, Value: "AcePanel"},
		{Key: biz.SettingKeyChannel, Value: "stable"},
		{Key: biz.SettingKeyVersion, Value: app.Version},
		{Key: biz.SettingKeyMonitor, Value: "true"},
		{Key: biz.SettingKeyMonitorDays, Value: "30"},
		{Key: biz.SettingKeyMonitorInterval, Value: "1"},
		{Key: biz.SettingKeyBackupPath, Value: filepath.Join(app.Root, "backup")},
		{Key: biz.SettingKeyBackupFormat, Value: "tar.xz"},
		{Key: biz.SettingKeyWebsitePath, Value: filepath.Join(app.Root, "sites")},
		{Key: biz.SettingKeyProjectPath, Value: filepath.Join(app.Root, "projects")},
		{Key: biz.SettingKeyContainerSock, Value: "/var/run/docker.sock"},
		{Key: biz.SettingKeyWebsiteTLSVersions, Value: `["TLSv1.2","TLSv1.3"]`},
		{Key: biz.SettingKeyWebsiteListenIPv6, Value: "false"},
		{Key: biz.SettingKeyOfflineMode, Value: "false"},
		{Key: biz.SettingKeyAutoUpdate, Value: "true"},
		{Key: biz.SettingHiddenMenu, Value: "[]"},
		{Key: biz.SettingKeyScanAware, Value: "false"},
		{Key: biz.SettingKeyScanAwareDays, Value: "30"},
		{Key: biz.SettingKeyWebsiteStatDays, Value: "30"},
		{Key: biz.SettingKeyWebsiteStatErrBufMax, Value: "10000"},
		{Key: biz.SettingKeyWebsiteStatUVMaxKeys, Value: "1000000"},
		{Key: biz.SettingKeyWebsiteStatIPMaxKeys, Value: "500000"},
		{Key: biz.SettingKeyWebsiteStatDetailMaxKeys, Value: "50000"},
		{Key: biz.SettingKeyWebsiteStatBodyEnabled, Value: "false"},
	}
	if err = s.db.Create(&settings).Error; err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	_, err = s.userRepo.Create(ctx, "admin", str.Random(32), str.Random(8)+"@yourdomain.com")
	if err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.App.Key = str.Random(32)
	conf.App.APIEndpoint = "api.acepanel.net"
	conf.App.DownloadEndpoint = "dl.acepanel.net"
	conf.HTTP.Entrance = "/" + str.Random(6)
	if acme {
		conf.HTTP.TLS = "acme"
	}

	// 随机默认端口
checkPort:
	// 10000-60000
	port := uint(rand.IntN(50000) + 10000) //nolint:gosec
	//nolint:contextcheck
	if os.TCPPortInUse(port) {
		goto checkPort
	}
	conf.HTTP.Port = port

	// 放行端口
	fw := firewall.NewFirewall()
	_ = fw.Port(ctx, firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: port,
		PortEnd:   port,
		Protocol:  firewall.ProtocolTCPUDP,
		Strategy:  firewall.StrategyAccept,
		Direction: firewall.DirectionIn,
	}, firewall.OperationAdd)

	if err = config.Save(conf); err != nil {
		return err
	}

	s.conf = conf // 更新配置，否则后续签发证书不会使用ACME

	if err = s.HTTPSGenerate(ctx, cmd); err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	return nil
}
