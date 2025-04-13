package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-rat/utils/collect"
	"github.com/go-rat/utils/hash"
	"github.com/go-rat/utils/str"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/tnb-labs/panel/internal/app"
	"github.com/tnb-labs/panel/internal/biz"
	"github.com/tnb-labs/panel/internal/http/request"
	"github.com/tnb-labs/panel/pkg/api"
	"github.com/tnb-labs/panel/pkg/cert"
	"github.com/tnb-labs/panel/pkg/firewall"
	"github.com/tnb-labs/panel/pkg/io"
	"github.com/tnb-labs/panel/pkg/ntp"
	"github.com/tnb-labs/panel/pkg/os"
	"github.com/tnb-labs/panel/pkg/systemctl"
	"github.com/tnb-labs/panel/pkg/tools"
	"github.com/tnb-labs/panel/pkg/types"
)

type CliService struct {
	hr                 string
	t                  *gotext.Locale
	api                *api.API
	conf               *koanf.Koanf
	db                 *gorm.DB
	appRepo            biz.AppRepo
	cacheRepo          biz.CacheRepo
	userRepo           biz.UserRepo
	settingRepo        biz.SettingRepo
	backupRepo         biz.BackupRepo
	websiteRepo        biz.WebsiteRepo
	databaseServerRepo biz.DatabaseServerRepo
	hash               hash.Hasher
}

func NewCliService(t *gotext.Locale, conf *koanf.Koanf, db *gorm.DB, appRepo biz.AppRepo, cache biz.CacheRepo, user biz.UserRepo, setting biz.SettingRepo, backup biz.BackupRepo, website biz.WebsiteRepo, databaseServer biz.DatabaseServerRepo) *CliService {
	return &CliService{
		hr:                 `+----------------------------------------------------`,
		api:                api.NewAPI(app.Version, app.Locale),
		t:                  t,
		conf:               conf,
		db:                 db,
		appRepo:            appRepo,
		cacheRepo:          cache,
		userRepo:           user,
		settingRepo:        setting,
		backupRepo:         backup,
		websiteRepo:        website,
		databaseServerRepo: databaseServer,
		hash:               hash.NewArgon2id(),
	}
}

func (s *CliService) Restart(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Restart("panel"); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Panel service restarted"))
	return nil
}

func (s *CliService) Stop(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Stop("panel"); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Panel service stopped"))
	return nil
}

func (s *CliService) Start(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Start("panel"); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Panel service started"))
	return nil
}

func (s *CliService) Update(ctx context.Context, cmd *cli.Command) error {
	panel, err := s.api.LatestVersion()
	if err != nil {
		return errors.New(s.t.Get("Failed to get latest version: %v", err))
	}

	download := collect.First(panel.Downloads)
	if download == nil {
		return errors.New(s.t.Get("Download URL is empty"))
	}

	return s.backupRepo.UpdatePanel(panel.Version, download.URL, download.Checksum)
}

func (s *CliService) Sync(ctx context.Context, cmd *cli.Command) error {
	if err := s.cacheRepo.UpdateApps(); err != nil {
		return errors.New(s.t.Get("Failed to synchronize app data: %v", err))
	}
	if err := s.cacheRepo.UpdateRewrites(); err != nil {
		return errors.New(s.t.Get("Failed to synchronize rewrite rules: %v", err))
	}

	fmt.Println(s.t.Get("Data synchronized successfully"))
	return nil
}

func (s *CliService) Fix(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.FixPanel()
}

func (s *CliService) Info(ctx context.Context, cmd *cli.Command) error {
	user := new(biz.User)
	if err := s.db.Where("id", 1).First(user).Error; err != nil {
		return errors.New(s.t.Get("Failed to get user info: %v", err))
	}

	password := str.Random(16)
	hashed, err := s.hash.Make(password)
	if err != nil {
		return errors.New(s.t.Get("Failed to generate password: %v", err))
	}
	user.Username = str.Random(8)
	user.Password = hashed
	if user.Email == "" {
		user.Email = str.Random(8) + "@example.com"
	}

	if err = s.db.Save(user).Error; err != nil {
		return errors.New(s.t.Get("Failed to save user info: %v", err))
	}

	protocol := "http"
	if s.conf.Bool("http.tls") {
		protocol = "https"
	}

	port := s.conf.String("http.port")
	if port == "" {
		return errors.New(s.t.Get("Failed to get port"))
	}
	entrance := s.conf.String("http.entrance")
	if entrance == "" {
		return errors.New(s.t.Get("Failed to get entrance"))
	}

	fmt.Println(s.t.Get("Username: %s", user.Username))
	fmt.Println(s.t.Get("Password: %s", password))
	fmt.Println(s.t.Get("Port: %s", port))
	fmt.Println(s.t.Get("Entrance: %s", entrance))

	lv4, err := tools.GetLocalIPv4()
	if err == nil {
		fmt.Println(s.t.Get("Local IPv4: %s://%s:%s%s", protocol, lv4, port, entrance))
	}
	lv6, err := tools.GetLocalIPv6()
	if err == nil {
		fmt.Println(s.t.Get("Local IPv6: %s://[%s]:%s%s", protocol, lv6, port, entrance))
	}
	rv4, err := tools.GetPublicIPv4()
	if err == nil {
		fmt.Println(s.t.Get("Public IPv4: %s://%s:%s%s", protocol, rv4, port, entrance))
	}
	rv6, err := tools.GetPublicIPv6()
	if err == nil {
		fmt.Println(s.t.Get("Public IPv6: %s://[%s]:%s%s", protocol, rv6, port, entrance))
	}

	fmt.Println(s.t.Get("Please choose the appropriate address to access the panel based on your network situation"))
	fmt.Println(s.t.Get("If you cannot access, please check whether the server's security group and firewall allow port %s", port))
	fmt.Println(s.t.Get("If you still cannot access, try running panel-cli https off to turn off panel HTTPS"))
	fmt.Println(s.t.Get("Warning: After turning off panel HTTPS, the security of the panel will be greatly reduced, please operate with caution"))

	return nil
}

func (s *CliService) UserList(ctx context.Context, cmd *cli.Command) error {
	users := make([]biz.User, 0)
	if err := s.db.Find(&users).Error; err != nil {
		return errors.New(s.t.Get("Failed to get user list: %v", err))
	}

	for _, user := range users {
		fmt.Println(s.t.Get("ID: %d, Username: %s, Email: %s, Created At: %s", user.ID, user.Username, user.Email, user.CreatedAt.Format(time.DateTime)))
	}

	return nil
}

func (s *CliService) UserName(ctx context.Context, cmd *cli.Command) error {
	user := new(biz.User)
	oldUsername := cmd.Args().Get(0)
	newUsername := cmd.Args().Get(1)
	if oldUsername == "" {
		return errors.New(s.t.Get("Old username cannot be empty"))
	}
	if newUsername == "" {
		return errors.New(s.t.Get("New username cannot be empty"))
	}

	if err := s.db.Where("username", oldUsername).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("User not exists"))
		} else {
			return errors.New(s.t.Get("Failed to get user: %v", err))
		}
	}

	user.Username = newUsername
	if err := s.db.Save(user).Error; err != nil {
		return errors.New(s.t.Get("Failed to change username: %v", err))
	}

	fmt.Println(s.t.Get("Username %s changed to %s successfully", oldUsername, newUsername))
	return nil
}

func (s *CliService) UserPassword(ctx context.Context, cmd *cli.Command) error {
	user := new(biz.User)
	username := cmd.Args().Get(0)
	password := cmd.Args().Get(1)
	if username == "" || password == "" {
		return errors.New(s.t.Get("Username and password cannot be empty"))
	}
	if len(password) < 6 {
		return errors.New(s.t.Get("Password length cannot be less than 6"))
	}

	if err := s.db.Where("username", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("User not exists"))
		} else {
			return errors.New(s.t.Get("Failed to get user: %v", err))
		}
	}

	hashed, err := s.hash.Make(password)
	if err != nil {
		return errors.New(s.t.Get("Failed to generate password: %v", err))
	}
	user.Password = hashed
	if err = s.db.Save(user).Error; err != nil {
		return errors.New(s.t.Get("Failed to change password: %v", err))
	}

	fmt.Println(s.t.Get("Password for user %s changed successfully", username))
	return nil
}

func (s *CliService) HTTPSOn(ctx context.Context, cmd *cli.Command) error {
	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return err
	}

	config.HTTP.TLS = true

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0700); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS enabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSOff(ctx context.Context, cmd *cli.Command) error {
	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return err
	}

	config.HTTP.TLS = false

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0700); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSGenerate(ctx context.Context, cmd *cli.Command) error {
	var names []string
	if lv4, err := tools.GetLocalIPv4(); err == nil {
		names = append(names, lv4)
	}
	if lv6, err := tools.GetLocalIPv6(); err == nil {
		names = append(names, lv6)
	}
	if rv4, err := tools.GetPublicIPv4(); err == nil {
		names = append(names, rv4)
	}
	if rv6, err := tools.GetPublicIPv6(); err == nil {
		names = append(names, rv6)
	}

	crt, key, err := cert.GenerateSelfSigned(names)
	if err != nil {
		return err
	}

	if err = io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), string(crt), 0644); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), string(key), 0644); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS certificate generated"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) EntranceOn(ctx context.Context, cmd *cli.Command) error {
	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return err
	}

	config.HTTP.Entrance = "/" + str.Random(6)

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0700); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance enabled"))
	fmt.Println(s.t.Get("Entrance: %s", config.HTTP.Entrance))
	return s.Restart(ctx, cmd)
}

func (s *CliService) EntranceOff(ctx context.Context, cmd *cli.Command) error {
	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return err
	}

	config.HTTP.Entrance = "/"

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0700); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) Port(ctx context.Context, cmd *cli.Command) error {
	port := cast.ToUint(cmd.Args().First())
	if port < 1 || port > 65535 {
		return errors.New(s.t.Get("Port range error"))
	}

	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return err
	}

	if port != config.HTTP.Port {
		if os.TCPPortInUse(port) {
			return errors.New(s.t.Get("Port already in use"))
		}
	}

	config.HTTP.Port = port

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// 放行端口
	fw := firewall.NewFirewall()
	err = fw.Port(firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: config.HTTP.Port,
		PortEnd:   config.HTTP.Port,
		Direction: firewall.DirectionIn,
		Strategy:  firewall.StrategyAccept,
	}, firewall.OperationAdd)
	if err != nil {
		return err
	}

	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0700); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Port changed to %d", port))
	return s.Restart(ctx, cmd)
}

func (s *CliService) WebsiteCreate(ctx context.Context, cmd *cli.Command) error {
	req := &request.WebsiteCreate{
		Name:    cmd.String("name"),
		Domains: cmd.StringSlice("domains"),
		Listens: cmd.StringSlice("listens"),
		Path:    cmd.String("path"),
		PHP:     int(cmd.Int("php")),
		DB:      false,
	}

	website, err := s.websiteRepo.Create(req)
	if err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s created successfully", website.Name))
	return nil
}

func (s *CliService) WebsiteRemove(ctx context.Context, cmd *cli.Command) error {
	website, err := s.websiteRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}
	req := &request.WebsiteDelete{
		ID: website.ID,
	}

	if err = s.websiteRepo.Delete(req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s removed successfully", website.Name))
	return nil
}

func (s *CliService) WebsiteDelete(ctx context.Context, cmd *cli.Command) error {
	website, err := s.websiteRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}
	req := &request.WebsiteDelete{
		ID:   website.ID,
		Path: true,
		DB:   true,
	}

	if err = s.websiteRepo.Delete(req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s deleted successfully", website.Name))
	return nil
}

func (s *CliService) WebsiteWrite(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.t.Get("Not supported"))
	return nil
}

func (s *CliService) DatabaseAddServer(ctx context.Context, cmd *cli.Command) error {
	req := &request.DatabaseServerCreate{
		Type:     cmd.String("type"),
		Name:     cmd.String("name"),
		Host:     cmd.String("host"),
		Port:     uint(cmd.Uint("port")),
		Username: cmd.String("username"),
		Password: cmd.String("password"),
		Remark:   cmd.String("remark"),
	}

	if err := s.databaseServerRepo.Create(req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Database server %s added successfully", cmd.String("name")))
	return nil
}

func (s *CliService) DatabaseDeleteServer(ctx context.Context, cmd *cli.Command) error {
	server, err := s.databaseServerRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}

	if err = s.databaseServerRepo.Delete(server.ID); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Database server %s deleted successfully", server.Name))
	return nil
}

func (s *CliService) BackupWebsite(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start backup [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Backup type: website"))
	fmt.Println(s.t.Get("|-Backup target: %s", cmd.String("name")))
	if err := s.backupRepo.Create(biz.BackupTypeWebsite, cmd.String("name"), cmd.String("path")); err != nil {
		return errors.New(s.t.Get("Backup failed: %v", err))
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Backup successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) BackupDatabase(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start backup [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Backup type: database"))
	fmt.Println(s.t.Get("|-Database: %s", cmd.String("type")))
	fmt.Println(s.t.Get("|-Backup target: %s", cmd.String("name")))
	if err := s.backupRepo.Create(biz.BackupType(cmd.String("type")), cmd.String("name"), cmd.String("path")); err != nil {
		return errors.New(s.t.Get("Backup failed: %v", err))
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Backup successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) BackupPanel(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start backup [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Backup type: panel"))
	if err := s.backupRepo.Create(biz.BackupTypePanel, "", cmd.String("path")); err != nil {
		return errors.New(s.t.Get("Backup failed: %v", err))
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Backup successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) BackupClear(ctx context.Context, cmd *cli.Command) error {
	path, err := s.backupRepo.GetPath(biz.BackupType(cmd.String("type")))
	if err != nil {
		return err
	}
	if cmd.String("path") != "" {
		path = cmd.String("path")
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start cleaning [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Cleaning type: %s", cmd.String("type")))
	fmt.Println(s.t.Get("|-Cleaning target: %s", cmd.String("file")))
	fmt.Println(s.t.Get("|-Keep count: %d", cmd.Int("save")))
	if err = s.backupRepo.ClearExpired(path, cmd.String("file"), int(cmd.Int("save"))); err != nil {
		return errors.New(s.t.Get("Cleaning failed: %v", err))
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Cleaning successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) CutoffWebsite(ctx context.Context, cmd *cli.Command) error {
	website, err := s.websiteRepo.GetByName(cmd.String("name"))
	if err != nil {
		return err
	}
	path := filepath.Join(app.Root, "wwwlogs")
	if cmd.String("path") != "" {
		path = cmd.String("path")
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start log rotation [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Rotation type: website"))
	fmt.Println(s.t.Get("|-Rotation target: %s", website.Name))
	if err = s.backupRepo.CutoffLog(path, filepath.Join(app.Root, "wwwlogs", website.Name+".log")); err != nil {
		return err
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Rotation successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) CutoffClear(ctx context.Context, cmd *cli.Command) error {
	if cmd.String("type") != "website" {
		return errors.New(s.t.Get("Currently only website log rotation is supported"))
	}
	path := filepath.Join(app.Root, "wwwlogs")
	if cmd.String("path") != "" {
		path = cmd.String("path")
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start cleaning rotated logs [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Cleaning type: %s", cmd.String("type")))
	fmt.Println(s.t.Get("|-Cleaning target: %s", cmd.String("file")))
	fmt.Println(s.t.Get("|-Keep count: %d", cmd.Int("save")))
	if err := s.backupRepo.ClearExpired(path, cmd.String("file"), int(cmd.Int("save"))); err != nil {
		return err
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Cleaning successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) AppInstall(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.Args().First()
	channel := cmd.Args().Get(1)
	if channel == "" || slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.appRepo.Install(channel, slug); err != nil {
		return errors.New(s.t.Get("App install failed: %v", err))
	}

	fmt.Println(s.t.Get("App %s installed successfully", slug))
	return nil
}

func (s *CliService) AppUnInstall(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.Args().First()
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.appRepo.UnInstall(slug); err != nil {
		return errors.New(s.t.Get("App uninstall failed: %v", err))
	}

	fmt.Println(s.t.Get("App %s uninstalled successfully", slug))
	return nil
}

func (s *CliService) AppUpdate(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.Args().First()
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.appRepo.Update(slug); err != nil {
		return errors.New(s.t.Get("App update failed: %v", err))
	}

	fmt.Println(s.t.Get("App %s updated successfully", slug))
	return nil
}

func (s *CliService) AppWrite(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.Args().Get(0)
	channel := cmd.Args().Get(1)
	version := cmd.Args().Get(2)
	if slug == "" || channel == "" || version == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	newApp := new(biz.App)
	if err := s.db.Where("slug", slug).First(newApp).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Failed to get app: %v", err))
		}
	}
	newApp.Slug = slug
	newApp.Channel = channel
	newApp.Version = version
	if err := s.db.Save(newApp).Error; err != nil {
		return errors.New(s.t.Get("Failed to save app: %v", err))
	}

	return nil
}

func (s *CliService) AppRemove(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.Args().First()
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.db.Where("slug", slug).Delete(&biz.App{}).Error; err != nil {
		return errors.New(s.t.Get("Failed to delete app: %v", err))
	}

	return nil
}

func (s *CliService) SyncTime(ctx context.Context, cmd *cli.Command) error {
	now, err := ntp.Now()
	if err != nil {
		return err
	}

	if err = ntp.UpdateSystemTime(now); err != nil {
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

func (s *CliService) GetSetting(ctx context.Context, cmd *cli.Command) error {
	key := cmd.Args().First()
	if key == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	setting := new(biz.Setting)
	if err := s.db.Where("key", key).First(setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Setting not exists"))
		}
		return errors.New(s.t.Get("Failed to get setting: %v", err))
	}

	fmt.Print(setting.Value)
	return nil
}

func (s *CliService) WriteSetting(ctx context.Context, cmd *cli.Command) error {
	key := cmd.Args().Get(0)
	value := cmd.Args().Get(1)
	if key == "" || value == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	setting := new(biz.Setting)
	if err := s.db.Where("key", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Failed to get setting: %v", err))
		}
	}
	setting.Key = biz.SettingKey(key)
	setting.Value = value
	if err := s.db.Save(setting).Error; err != nil {
		return errors.New(s.t.Get("Failed to save setting: %v", err))
	}

	return nil
}

func (s *CliService) RemoveSetting(ctx context.Context, cmd *cli.Command) error {
	key := cmd.Args().First()
	if key == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}

	if err := s.db.Where("key", key).Delete(&biz.Setting{}).Error; err != nil {
		return errors.New(s.t.Get("Failed to delete setting: %v", err))
	}

	return nil
}

func (s *CliService) Init(ctx context.Context, cmd *cli.Command) error {
	var check biz.User
	if err := s.db.First(&check).Error; err == nil {
		return errors.New(s.t.Get("Already initialized"))
	}

	settings := []biz.Setting{
		{Key: biz.SettingKeyName, Value: "耗子面板"},
		{Key: biz.SettingKeyMonitor, Value: "true"},
		{Key: biz.SettingKeyMonitorDays, Value: "30"},
		{Key: biz.SettingKeyBackupPath, Value: filepath.Join(app.Root, "backup")},
		{Key: biz.SettingKeyWebsitePath, Value: filepath.Join(app.Root, "wwwroot")},
		{Key: biz.SettingKeyVersion, Value: app.Version},
		{Key: biz.SettingKeyOfflineMode, Value: "false"},
		{Key: biz.SettingKeyAutoUpdate, Value: "true"},
	}
	if err := s.db.Create(&settings).Error; err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	value, err := hash.NewArgon2id().Make(str.Random(32))
	if err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	_, err = s.userRepo.Create("admin", value)
	if err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	if err = s.HTTPSGenerate(ctx, cmd); err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return err
	}

	config.App.Key = str.Random(32)
	config.HTTP.Entrance = "/" + str.Random(6)

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0700); err != nil {
		return err
	}

	return nil
}
