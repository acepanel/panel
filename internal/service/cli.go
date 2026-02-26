package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	stdos "os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/collect"
	"github.com/libtnb/utils/hash"
	"github.com/libtnb/utils/str"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/ntp"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type CliService struct {
	hr                 string
	t                  *gotext.Locale
	api                *api.API
	conf               *config.Config
	db                 *gorm.DB
	appRepo            biz.AppRepo
	cacheRepo          biz.CacheRepo
	userRepo           biz.UserRepo
	userPasskeyRepo    biz.UserPasskeyRepo
	settingRepo        biz.SettingRepo
	backupRepo         biz.BackupRepo
	websiteRepo        biz.WebsiteRepo
	databaseServerRepo biz.DatabaseServerRepo
	certRepo           biz.CertRepo
	certAccountRepo    biz.CertAccountRepo
	hash               hash.Hasher
}

func NewCliService(t *gotext.Locale, conf *config.Config, db *gorm.DB, appRepo biz.AppRepo, cache biz.CacheRepo, user biz.UserRepo, userPasskey biz.UserPasskeyRepo, setting biz.SettingRepo, backup biz.BackupRepo, website biz.WebsiteRepo, databaseServer biz.DatabaseServerRepo, cert biz.CertRepo, certAccount biz.CertAccountRepo) *CliService {
	return &CliService{
		hr:                 `+----------------------------------------------------`,
		api:                api.NewAPI(app.Version, app.Locale),
		t:                  t,
		conf:               conf,
		db:                 db,
		appRepo:            appRepo,
		cacheRepo:          cache,
		userRepo:           user,
		userPasskeyRepo:    userPasskey,
		settingRepo:        setting,
		backupRepo:         backup,
		websiteRepo:        website,
		databaseServerRepo: databaseServer,
		certRepo:           cert,
		certAccountRepo:    certAccount,
		hash:               hash.NewArgon2id(),
	}
}

func (s *CliService) Status(ctx context.Context, cmd *cli.Command) error {
	status, err := systemctl.Status("acepanel")
	if err != nil {
		return err
	}

	statusStr := s.t.Get("unknown")
	switch status {
	case true:
		statusStr = s.t.Get("running")
	case false:
		statusStr = s.t.Get("stopped")
	}

	fmt.Println(s.t.Get("AcePanel service status: %s", statusStr))

	return nil
}

func (s *CliService) Restart(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Restart("acepanel"); err != nil {
		return err
	}
	fmt.Println(s.t.Get("AcePanel service restarted"))
	return nil
}

func (s *CliService) Stop(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Stop("acepanel"); err != nil {
		return err
	}
	fmt.Println(s.t.Get("AcePanel service stopped"))
	return nil
}

func (s *CliService) Start(ctx context.Context, cmd *cli.Command) error {
	if err := systemctl.Start("acepanel"); err != nil {
		return err
	}
	fmt.Println(s.t.Get("AcePanel service started"))
	return nil
}

func (s *CliService) Update(ctx context.Context, cmd *cli.Command) error {
	channel, _ := s.settingRepo.Get(biz.SettingKeyChannel)
	panel, err := s.api.LatestVersion(channel)
	if err != nil {
		return errors.New(s.t.Get("Failed to get latest version: %v", err))
	}

	download := collect.First(panel.Downloads)
	if download == nil {
		return errors.New(s.t.Get("Download URL is empty"))
	}

	url := fmt.Sprintf("https://%s%s", s.conf.App.DownloadEndpoint, download.URL)
	checksum := fmt.Sprintf("https://%s%s", s.conf.App.DownloadEndpoint, download.Checksum)

	return s.backupRepo.UpdatePanel(panel.Version, url, checksum)
}

func (s *CliService) Sync(ctx context.Context, cmd *cli.Command) error {
	if err := s.cacheRepo.UpdateCategories(); err != nil {
		return errors.New(s.t.Get("Failed to synchronize categories data: %v", err))
	}
	if err := s.cacheRepo.UpdateApps(); err != nil {
		return errors.New(s.t.Get("Failed to synchronize app data: %v", err))
	}
	if err := s.cacheRepo.UpdateEnvironments(); err != nil {
		return errors.New(s.t.Get("Failed to synchronize app data: %v", err))
	}
	if err := s.cacheRepo.UpdateTemplates(); err != nil {
		return errors.New(s.t.Get("Failed to synchronize app data: %v", err))
	}

	fmt.Println(s.t.Get("Data synchronized successfully"))
	return nil
}

func (s *CliService) Fix(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.FixPanel()
}

func (s *CliService) Info(ctx context.Context, cmd *cli.Command) error {
	user := new(biz.User)
	if err := s.db.First(user).Error; err != nil {
		return errors.New(s.t.Get("Failed to get user info: %v", err))
	}

	password := str.Random(16)
	hashed, err := s.hash.Make(password)
	if err != nil {
		return errors.New(s.t.Get("Failed to generate password: %v", err))
	}
	user.Username = str.Random(8)
	user.Password = hashed

	if err = s.db.Save(user).Error; err != nil {
		return errors.New(s.t.Get("Failed to save user info: %v", err))
	}

	protocol := "http"
	if s.conf.HTTP.TLS {
		protocol = "https"
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
	fmt.Println(s.t.Get("Password: %s", password))
	fmt.Println(s.t.Get("Port: %d", port))
	fmt.Println(s.t.Get("Entrance: %s", entrance))

	lv4, err := tools.GetLocalIPv4()
	if err == nil {
		fmt.Println(s.t.Get("Local IPv4: %s://%s:%d%s", protocol, lv4, port, entrance))
	}
	lv6, err := tools.GetLocalIPv6()
	if err == nil {
		fmt.Println(s.t.Get("Local IPv6: %s://[%s]:%d%s", protocol, lv6, port, entrance))
	}
	rv4, err := tools.GetPublicIPv4()
	if err == nil {
		fmt.Println(s.t.Get("Public IPv4: %s://%s:%d%s", protocol, rv4, port, entrance))
	}
	rv6, err := tools.GetPublicIPv6()
	if err == nil {
		fmt.Println(s.t.Get("Public IPv6: %s://[%s]:%d%s", protocol, rv6, port, entrance))
	}

	fmt.Println(s.t.Get("Please choose the appropriate address to access the panel based on your network situation"))
	fmt.Println(s.t.Get("If you cannot access, please check whether the server's security group and firewall allow port %d", port))
	fmt.Println(s.t.Get("If you still cannot access, try running `acepanel https off` to turn off panel HTTPS"))
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
		}
		return errors.New(s.t.Get("Failed to get user: %v", err))
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
		}
		return errors.New(s.t.Get("Failed to get user: %v", err))
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

func (s *CliService) UserTwoFA(ctx context.Context, cmd *cli.Command) error {
	user := new(biz.User)
	username := cmd.Args().Get(0)
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	if err := s.db.Where("username", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("User not exists"))
		}
		return errors.New(s.t.Get("Failed to get user: %v", err))
	}

	// 已开启，关闭2FA
	if user.TwoFA != "" {
		user.TwoFA = ""
		if err := s.db.Save(user).Error; err != nil {
			return errors.New(s.t.Get("Failed to change 2FA status: %v", err))
		}
		fmt.Println(s.t.Get("2FA disabled for user %s", username))
		return nil
	}
	// 未开启，开启2FA
	_, url, secret, err := s.userRepo.GenerateTwoFA(user.ID)
	if err != nil {
		return errors.New(s.t.Get("Failed to generate 2FA: %v", err))
	}
	fmt.Println(s.t.Get("2FA url: %s", url))
	reader := bufio.NewReader(stdos.Stdin)
	fmt.Print(s.t.Get("Please enter the 2FA code: "))
	code, err := reader.ReadString('\n')
	if err != nil {
		return errors.New(s.t.Get("Failed to read input: %v", err))
	}
	if err = s.userRepo.UpdateTwoFA(user.ID, strings.TrimSpace(code), secret); err != nil {
		return errors.New(s.t.Get("Failed to update 2FA: %v", err))
	}

	return nil
}

func (s *CliService) UserPasskey(ctx context.Context, cmd *cli.Command) error {
	user := new(biz.User)
	username := cmd.Args().Get(0)
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	if err := s.db.Where("username", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("User not exists"))
		}
		return errors.New(s.t.Get("Failed to get user: %v", err))
	}

	if err := s.userPasskeyRepo.DeleteAllByUserID(user.ID); err != nil {
		return errors.New(s.t.Get("Failed to clear passkeys: %v", err))
	}

	fmt.Println(s.t.Get("All passkeys cleared for user %s", username))
	return nil
}

func (s *CliService) HTTPSOn(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.TLS = true

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS enabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSOff(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.TLS = false

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSGenerate(ctx context.Context, cmd *cli.Command) error {
	var names []string
	if lv4, err := tools.GetLocalIPv4(); err == nil {
		if !slices.Contains(names, lv4) {
			names = append(names, lv4)
		}
	}
	if lv6, err := tools.GetLocalIPv6(); err == nil {
		if !slices.Contains(names, lv6) {
			names = append(names, lv6)
		}
	}
	if rv4, err := tools.GetPublicIPv4(); err == nil {
		if !slices.Contains(names, rv4) {
			names = append(names, rv4)
		}
	}
	if rv6, err := tools.GetPublicIPv6(); err == nil {
		if !slices.Contains(names, rv6) {
			names = append(names, rv6)
		}
	}

	var crt, key []byte
	var err error

	if s.conf.HTTP.ACME {
		ip, err := s.settingRepo.Get(biz.SettingKeyPublicIPs)
		if err != nil {
			return err
		}
		var ips []string
		if err = json.Unmarshal([]byte(ip), &ips); err != nil || len(ips) == 0 {
			return errors.New(s.t.Get("Please set the panel IP in settings first for ACME certificate generation"))
		}

		var user biz.User
		if err = s.db.First(&user).Error; err != nil {
			return errors.New(s.t.Get("Failed to get a panel user: %v", err))
		}
		account, err := s.certAccountRepo.GetDefault(user.ID)
		if err != nil {
			return errors.New(s.t.Get("Failed to get ACME account: %v", err))
		}
		crt, key, err = s.certRepo.ObtainPanel(account, ips)
		if err == nil {
			fmt.Println(s.t.Get("Successfully obtained panel certificate via ACME"))
		} else {
			fmt.Println(s.t.Get("Failed to obtain panel certificate via ACME, using self-signed certificate"))
		}
	}

	if crt == nil || key == nil {
		crt, key, err = cert.GenerateSelfSigned(names)
		if err != nil {
			return err
		}
	}

	if err = io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), string(crt), 0600); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), string(key), 0600); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS certificate generated"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) EntranceOn(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.Entrance = "/" + str.Random(6)

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance enabled"))
	fmt.Println(s.t.Get("Entrance: %s", conf.HTTP.Entrance))
	return s.Restart(ctx, cmd)
}

func (s *CliService) EntranceOff(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.Entrance = "/"

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Entrance disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindDomainOff(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.BindDomain = nil

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind domain disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindIPOff(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.BindIP = nil

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind IP disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) BindUAOff(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.BindUA = nil

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind UA disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) Port(ctx context.Context, cmd *cli.Command) error {
	port := cast.ToUint(cmd.Args().First())
	if port < 1 || port > 65535 {
		return errors.New(s.t.Get("Port range error"))
	}

	conf, err := config.Load()
	if err != nil {
		return err
	}

	if port != conf.HTTP.Port {
		if os.TCPPortInUse(port) {
			return errors.New(s.t.Get("Port already in use"))
		}
	}

	conf.HTTP.Port = port

	// 放行端口
	fw := firewall.NewFirewall()
	if ok, _ := fw.Status(); ok {
		err = fw.Port(firewall.FireInfo{
			Type:      firewall.TypeNormal,
			PortStart: port,
			PortEnd:   port,
			Protocol:  firewall.ProtocolTCPUDP,
			Strategy:  firewall.StrategyAccept,
			Direction: firewall.DirectionIn,
		}, firewall.OperationAdd)
		if err != nil {
			return err
		}
	}

	if err = config.Save(conf); err != nil {
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
		PHP:     cmd.Uint("php"),
		DB:      false,
	}

	website, err := s.websiteRepo.Create(ctx, req)
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

	if err = s.websiteRepo.Delete(ctx, req); err != nil {
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

	if err = s.websiteRepo.Delete(ctx, req); err != nil {
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
		Port:     cmd.Uint("port"),
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
	_ = s.backupRepo.Create(ctx, biz.BackupTypeWebsite, cmd.String("name"), cmd.Uint("storage"))
	return nil
}

func (s *CliService) BackupDatabase(ctx context.Context, cmd *cli.Command) error {
	_ = s.backupRepo.Create(ctx, biz.BackupType(cmd.String("type")), cmd.String("name"), cmd.Uint("storage"))
	return nil
}

func (s *CliService) BackupPanel(ctx context.Context, cmd *cli.Command) error {
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start backup [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Backup type: panel"))
	if err := s.backupRepo.CreatePanel(); err != nil {
		return errors.New(s.t.Get("Backup failed: %v", err))
	}
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Backup successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) BackupClear(ctx context.Context, cmd *cli.Command) error {

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start cleaning [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Cleaning type: %s", cmd.String("type")))
	fmt.Println(s.t.Get("|-Cleaning target: %s", cmd.String("file")))
	fmt.Println(s.t.Get("|-Keep count: %d", cmd.Uint("keep")))

	if cmd.Uint("storage") != 0 {
		if err := s.backupRepo.ClearStorageExpired(cmd.Uint("storage"), biz.BackupType(cmd.String("type")), cmd.String("file"), cmd.Uint("keep")); err != nil {
			return errors.New(s.t.Get("Cleaning failed: %v", err))
		}
	} else {
		path := s.backupRepo.GetDefaultPath(biz.BackupType(cmd.String("type")))
		if err := s.backupRepo.ClearExpired(path, cmd.String("file"), cmd.Uint("keep")); err != nil {
			return errors.New(s.t.Get("Cleaning failed: %v", err))
		}
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
	path := filepath.Join(app.Root, "sites", website.Name, "log")

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start log rotation [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Rotation type: website"))
	fmt.Println(s.t.Get("|-Rotation target: %s", website.Name))

	var files []string
	zipPath, err := s.backupRepo.CutoffLog(path, filepath.Join(path, "access.log"))
	if err != nil {
		return err
	}
	files = append(files, zipPath)
	zipPath, err = s.backupRepo.CutoffLog(path, filepath.Join(path, "error.log"))
	if err != nil {
		return err
	}
	files = append(files, zipPath)

	// 上传到远程存储
	if cmd.Uint("storage") != 0 {
		if err = s.backupRepo.CutoffUpload(cmd.Uint("storage"), biz.BackupTypeWebsite, website.Name, files); err != nil {
			return err
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Rotation successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) CutoffContainer(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")

	// 获取容器日志路径
	logPath, err := shell.Execf("docker inspect --format='{{.LogPath}}' '%s'", name)
	if err != nil {
		return errors.New(s.t.Get("Failed to get container log path: %v", err))
	}
	logPath = strings.TrimSpace(logPath)
	if logPath == "" {
		return errors.New(s.t.Get("Container %s has no log file", name))
	}

	savePath := filepath.Join(app.Root, "server/cutoff/container", name)

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start log rotation [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Rotation type: container"))
	fmt.Println(s.t.Get("|-Rotation target: %s", name))

	zipPath, err := s.backupRepo.CutoffLog(savePath, logPath)
	if err != nil {
		return err
	}

	// 上传到远程存储
	if cmd.Uint("storage") != 0 {
		if err = s.backupRepo.CutoffUpload(cmd.Uint("storage"), "container", name, []string{zipPath}); err != nil {
			return err
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Rotation successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) CutoffClear(ctx context.Context, cmd *cli.Command) error {
	typ := cmd.String("type")
	name := cmd.String("name")
	keep := cmd.Uint("keep")
	storageID := cmd.Uint("storage")

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("★ Start cleaning rotated logs [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	fmt.Println(s.t.Get("|-Cleaning type: %s", typ))
	fmt.Println(s.t.Get("|-Cleaning target: %s", name))
	fmt.Println(s.t.Get("|-Keep count: %d", keep))

	switch typ {
	case "website":
		website, err := s.websiteRepo.GetByName(name)
		if err != nil {
			return err
		}
		path := filepath.Join(app.Root, "sites", website.Name, "log")
		if err = s.backupRepo.ClearExpired(path, "access_", keep); err != nil {
			return err
		}
		if err = s.backupRepo.ClearExpired(path, "error_", keep); err != nil {
			return err
		}
	case "container":
		path := filepath.Join(app.Root, "server/cutoff/container", name)
		if err := s.backupRepo.ClearExpired(path, "", keep); err != nil {
			return err
		}
	default:
		return errors.New(s.t.Get("Unsupported rotation type: %s", typ))
	}

	// 清理远程存储过期日志
	if storageID != 0 {
		if err := s.backupRepo.ClearStorageExpired(storageID, biz.BackupType(typ), name, keep); err != nil {
			return err
		}
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
		{Key: biz.SettingKeyBackupPath, Value: filepath.Join(app.Root, "backup")},
		{Key: biz.SettingKeyWebsitePath, Value: filepath.Join(app.Root, "sites")},
		{Key: biz.SettingKeyProjectPath, Value: filepath.Join(app.Root, "projects")},
		{Key: biz.SettingKeyContainerSock, Value: "/var/run/docker.sock"},
		{Key: biz.SettingKeyWebsiteTLSVersions, Value: `["TLSv1.2","TLSv1.3"]`},
		{Key: biz.SettingKeyWebsiteCipherSuites, Value: `ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:DHE-RSA-CHACHA20-POLY1305`},
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

	value, err := hash.NewArgon2id().Make(str.Random(32))
	if err != nil {
		return errors.New(s.t.Get("Initialization failed: %v", err))
	}

	_, err = s.userRepo.Create(ctx, "admin", value, str.Random(8)+"@yourdomain.com")
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
	conf.HTTP.ACME = acme

	// 随机默认端口
checkPort:
	port := uint(rand.IntN(50000) + 10000) // 10000-60000
	if os.TCPPortInUse(port) {
		goto checkPort
	}
	conf.HTTP.Port = port

	// 放行端口
	fw := firewall.NewFirewall()
	_ = fw.Port(firewall.FireInfo{
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
