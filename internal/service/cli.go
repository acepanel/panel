package service

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	"github.com/libtnb/validator"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
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
	"github.com/acepanel/panel/v3/pkg/types"
)

// passwordEnv 交互输入前回退读取的密码环境变量
const passwordEnv = "ACEPANEL_PASSWORD"

type CliService struct {
	hr                 string
	t                  *gotext.Locale
	api                *api.API
	conf               *config.Config
	db                 *gorm.DB
	appRepo            *biz.AppUsecase
	cacheRepo          *biz.CacheUsecase
	userRepo           *biz.UserUsecase
	userPasskeyRepo    *biz.UserPasskeyUsecase
	settingRepo        *biz.SettingUsecase
	backupRepo         *biz.BackupUsecase
	websiteRepo        *biz.WebsiteUsecase
	databaseServerRepo *biz.DatabaseServerUsecase
	certRepo           *biz.CertUsecase
	certAccountRepo    *biz.CertAccountUsecase
	cronRepo           *biz.CronUsecase
	notifyRepo         *biz.NotifyUsecase
	hash               hash.Hasher
	validator          *validator.Validator
}

func NewCliService(appUsecase *biz.AppUsecase, backupUsecase *biz.BackupUsecase, cacheUsecase *biz.CacheUsecase, certAccountUsecase *biz.CertAccountUsecase, certUsecase *biz.CertUsecase, cronUsecase *biz.CronUsecase, databaseServerUsecase *biz.DatabaseServerUsecase, notifyUsecase *biz.NotifyUsecase, settingUsecase *biz.SettingUsecase, userPasskeyUsecase *biz.UserPasskeyUsecase, userUsecase *biz.UserUsecase, websiteUsecase *biz.WebsiteUsecase, conf *config.Config, db *gorm.DB, t *gotext.Locale, v *validator.Validator) *CliService {
	return &CliService{
		hr:                 `+----------------------------------------------------`,
		api:                api.NewAPI(app.Version, app.Locale),
		t:                  t,
		conf:               conf,
		db:                 db,
		validator:          v,
		appRepo:            appUsecase,
		cacheRepo:          cacheUsecase,
		userRepo:           userUsecase,
		userPasskeyRepo:    userPasskeyUsecase,
		settingRepo:        settingUsecase,
		backupRepo:         backupUsecase,
		websiteRepo:        websiteUsecase,
		databaseServerRepo: databaseServerUsecase,
		certRepo:           certUsecase,
		certAccountRepo:    certAccountUsecase,
		cronRepo:           cronUsecase,
		notifyRepo:         notifyUsecase,
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

	if err = s.backupRepo.UpdatePanel(panel.Version, url, checksum, func(msg string) {
		fmt.Println("|-" + msg)
	}); err != nil {
		return err
	}
	tools.RestartPanel()
	return nil
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
	// 未指定用户名时取第一个用户，兼容单用户场景
	user := new(biz.User)
	query := s.db
	if username := cmd.String("username"); username != "" {
		query = query.Where("username", username)
	}
	if err := query.First(user).Error; err != nil {
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
		hashed, err := s.hash.Make(password)
		if err != nil {
			return errors.New(s.t.Get("Failed to generate password: %v", err))
		}
		user.Username = str.Random(8)
		user.Password = hashed

		if err = s.db.Save(user).Error; err != nil {
			return errors.New(s.t.Get("Failed to save user info: %v", err))
		}

		// 标记 info 命令已运行过
		if err = s.settingRepo.Set(biz.SettingKeyInfoRan, "1"); err != nil {
			return errors.New(s.t.Get("Failed to save setting: %v", err))
		}
	}

	protocol := "http"
	if s.conf.HTTP.IsHTTPS() {
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
	if isFirstRun || force {
		fmt.Println(s.t.Get("Password: %s", password))
	} else {
		fmt.Println(s.t.Get("Password: ******* (use -f to force reset)"))
	}
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

	return s.printList(cmd, users, func() {
		for _, user := range users {
			fmt.Println(s.t.Get("ID: %d, Username: %s, Email: %s, Created At: %s", user.ID, user.Username, user.Email, user.CreatedAt.Format(time.DateTime)))
		}
	})
}

func (s *CliService) UserCreate(ctx context.Context, cmd *cli.Command) error {
	username := cmd.Args().Get(0)
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	password, err := s.readSecret(cmd.Args().Get(1), passwordEnv, s.t.Get("Please enter the password: "))
	if err != nil {
		return err
	}
	if len(password) < 6 {
		return errors.New(s.t.Get("Password length cannot be less than 6"))
	}

	// 邮箱仅用于占位，面板不强制校验可达性
	email := cmd.String("email")
	if email == "" {
		email = username + "@example.com"
	}

	user, err := s.userRepo.Create(ctx, username, password, email)
	if err != nil {
		return errors.New(s.t.Get("Failed to create user: %v", err))
	}

	fmt.Println(s.t.Get("User %s created successfully", user.Username))
	return nil
}

func (s *CliService) UserDelete(ctx context.Context, cmd *cli.Command) error {
	username := cmd.Args().First()
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	user := new(biz.User)
	if err := s.db.Where("username", username).First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("User not exists"))
		}
		return errors.New(s.t.Get("Failed to get user: %v", err))
	}

	if err := s.userRepo.Delete(ctx, user.ID); err != nil {
		return errors.New(s.t.Get("Failed to delete user: %v", err))
	}

	fmt.Println(s.t.Get("User %s deleted successfully", username))
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
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	password, err := s.readSecret(cmd.Args().Get(1), passwordEnv, s.t.Get("Please enter the new password: "))
	if err != nil {
		return err
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

	conf.HTTP.TLS = "acme"

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

	conf.HTTP.TLS = "off"

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("HTTPS disabled"))
	return s.Restart(ctx, cmd)
}

func (s *CliService) HTTPSGenerate(ctx context.Context, cmd *cli.Command) error {
	names := tools.CollectLocalNames()

	var crt, key []byte
	var err error

	switch s.conf.HTTP.TLS {
	case "self-signed":
		// 自签模式
		crt, key, err = cert.GenerateSelfSigned(names)
		if err != nil {
			return err
		}
	case "off", "custom":
		// off/custom 不需要自动生成，回退到自签
		crt, key, err = cert.GenerateSelfSigned(names)
		if err != nil {
			return err
		}
	default:
		// ACME 模式
		var user biz.User
		if err = s.db.First(&user).Error; err != nil {
			return errors.New(s.t.Get("Failed to get a panel user: %v", err))
		}
		account, err := s.certAccountRepo.GetDefault(user.ID)
		if err != nil {
			return errors.New(s.t.Get("Failed to get ACME account: %v", err))
		}
		crt, key, err = s.certRepo.ObtainPanel(account, s.conf.HTTP.BindDomain)
		if err == nil {
			fmt.Println(s.t.Get("Successfully obtained panel certificate via ACME"))
		} else {
			fmt.Println(s.t.Get("Failed to obtain panel certificate via ACME, using self-signed certificate"))
		}
	}

	// ACME 失败回退到自签
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

func (s *CliService) BindDomainOn(ctx context.Context, cmd *cli.Command) error {
	domains := cmd.Args().Slice()
	if len(domains) == 0 {
		return errors.New(s.t.Get("Please specify at least one domain"))
	}

	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.BindDomain = domains

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind domain enabled: %s", strings.Join(domains, ", ")))
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

func (s *CliService) BindIPOn(ctx context.Context, cmd *cli.Command) error {
	ips := cmd.Args().Slice()
	if len(ips) == 0 {
		return errors.New(s.t.Get("Please specify at least one IP"))
	}

	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.BindIP = ips

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind IP enabled: %s", strings.Join(ips, ", ")))
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

func (s *CliService) BindUAOn(ctx context.Context, cmd *cli.Command) error {
	uas := cmd.Args().Slice()
	if len(uas) == 0 {
		return errors.New(s.t.Get("Please specify at least one User-Agent"))
	}

	conf, err := config.Load()
	if err != nil {
		return err
	}

	conf.HTTP.BindUA = uas

	if err = config.Save(conf); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Bind UA enabled: %s", strings.Join(uas, ", ")))
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
	// 兼容旧的位置参数写法
	port := cmp.Or(cmd.Uint("port"), cast.ToUint(cmd.Args().First()))
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

func (s *CliService) FirewallStatus(ctx context.Context, cmd *cli.Command) error {
	fw := firewall.NewFirewall()
	running, err := fw.Status()
	if err != nil {
		return err
	}

	statusStr := s.t.Get("stopped")
	if running {
		statusStr = s.t.Get("running")
	}
	fmt.Println(s.t.Get("Firewall status: %s", statusStr))

	if ping, pingErr := fw.PingStatus(); pingErr == nil {
		fmt.Println(s.t.Get("Ping allowed: %t", ping))
	}

	return nil
}

func (s *CliService) FirewallOn(ctx context.Context, cmd *cli.Command) error {
	if err := firewall.NewFirewall().Enable(); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Firewall enabled"))
	return nil
}

func (s *CliService) FirewallOff(ctx context.Context, cmd *cli.Command) error {
	if err := firewall.NewFirewall().Disable(); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Firewall disabled"))
	return nil
}

func (s *CliService) FirewallList(ctx context.Context, cmd *cli.Command) error {
	rules, err := firewall.NewFirewall().ListRule()
	if err != nil {
		return err
	}

	return s.printList(cmd, rules, func() {
		for _, rule := range rules {
			fmt.Println(s.t.Get("Port: %d-%d, Protocol: %s, Strategy: %s, Direction: %s, Address: %s", rule.PortStart, rule.PortEnd, string(rule.Protocol), string(rule.Strategy), string(rule.Direction), rule.Address))
		}
	})
}

// FirewallPort 放行或移除端口，端口支持 8888 与 8000-9000 两种写法
func (s *CliService) FirewallPort(ctx context.Context, cmd *cli.Command) error {
	value := cmd.Args().First()
	start, end, found := strings.Cut(value, "-")
	if !found {
		end = start
	}

	portStart, portEnd := cast.ToUint(start), cast.ToUint(end)
	startValid := portStart >= 1 && portStart <= 65535
	endValid := portEnd >= 1 && portEnd <= 65535
	if !startValid || !endValid || portStart > portEnd {
		return errors.New(s.t.Get("Port range error"))
	}

	protocol := firewall.Protocol(cmd.String("protocol"))
	if !slices.Contains([]firewall.Protocol{firewall.ProtocolTCP, firewall.ProtocolUDP, firewall.ProtocolTCPUDP}, protocol) {
		return errors.New(s.t.Get("Unsupported protocol: %s", cmd.String("protocol")))
	}

	fw := firewall.NewFirewall()
	if running, _ := fw.Status(); !running {
		return errors.New(s.t.Get("Firewall is not running"))
	}

	operation := firewall.OperationAdd
	if cmd.Bool("remove") {
		operation = firewall.OperationRemove
	}

	if err := fw.Port(firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: portStart,
		PortEnd:   portEnd,
		Protocol:  protocol,
		Strategy:  firewall.StrategyAccept,
		Direction: firewall.DirectionIn,
	}, operation); err != nil {
		return err
	}

	msg := s.t.Get("Port %s allowed successfully", value)
	if cmd.Bool("remove") {
		msg = s.t.Get("Port %s removed successfully", value)
	}

	fmt.Println(msg)
	return nil
}

func (s *CliService) WebsiteList(ctx context.Context, cmd *cli.Command) error {
	websites, _, err := s.websiteRepo.List("all", 1, math.MaxUint32)
	if err != nil {
		return err
	}

	return s.printList(cmd, websites, func() {
		for _, website := range websites {
			status := s.t.Get("stopped")
			if website.Status {
				status = s.t.Get("running")
			}
			fmt.Println(s.t.Get("ID: %d, Name: %s, Type: %s, Status: %s, Path: %s", website.ID, website.Name, string(website.Type), status, website.Path))
		}
	})
}

func (s *CliService) WebsiteCreate(ctx context.Context, cmd *cli.Command) error {
	req := &request.WebsiteCreate{
		Type:       cmd.String("type"),
		Name:       cmd.String("name"),
		Domains:    cmd.StringSlice("domains"),
		Listens:    cmd.StringSlice("listens"),
		Path:       cmd.String("path"),
		Remark:     cmd.String("remark"),
		PHP:        cmd.Uint("php"),
		Proxy:      cmd.String("proxy"),
		DB:         cmd.String("db") != "",
		DBType:     cmd.String("db"),
		DBName:     cmd.String("db-name"),
		DBUser:     cmd.String("db-user"),
		DBPassword: cmd.String("db-password"),
	}

	// 未指定目录时使用默认目录，与面板端行为保持一致
	if req.Path == "" {
		root, err := s.settingRepo.Get(biz.SettingKeyWebsitePath)
		if err != nil {
			return err
		}
		req.Path = filepath.Join(root, req.Name, "public")
	}
	// 数据库参数缺省时按网站名生成，密码随机
	if req.DB {
		req.DBName = cmp.Or(req.DBName, req.Name)
		req.DBUser = cmp.Or(req.DBUser, req.Name)
		req.DBPassword = cmp.Or(req.DBPassword, str.Random(16))
	}

	if err := s.validate(ctx, req); err != nil {
		return err
	}

	website, err := s.websiteRepo.Create(ctx, req)
	if err != nil {
		return err
	}

	fmt.Println(s.t.Get("Website %s created successfully", website.Name))
	fmt.Println(s.t.Get("Path: %s", req.Path))
	if req.DB {
		fmt.Println(s.t.Get("Database: %s, username: %s, password: %s", req.DBName, req.DBUser, req.DBPassword))
	}
	return nil
}

// WebsiteCert 从文件写入网站证书，供外部签发工具对接
func (s *CliService) WebsiteCert(ctx context.Context, cmd *cli.Command) error {
	certPEM, err := stdos.ReadFile(cmd.String("cert"))
	if err != nil {
		return errors.New(s.t.Get("Failed to read certificate file: %v", err))
	}
	keyPEM, err := stdos.ReadFile(cmd.String("key"))
	if err != nil {
		return errors.New(s.t.Get("Failed to read private key file: %v", err))
	}

	req := &request.WebsiteUpdateCert{
		Name: cmd.String("name"),
		Cert: string(certPEM),
		Key:  string(keyPEM),
	}
	if err = s.validate(ctx, req); err != nil {
		return err
	}

	if err = s.websiteRepo.UpdateCert(req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Certificate for website %s updated successfully", req.Name))
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

// WebsiteWrite 仅写入网站数据到面板数据库，不创建网站目录和配置文件
func (s *CliService) WebsiteWrite(ctx context.Context, cmd *cli.Command) error {
	name := cmd.String("name")
	typ := biz.WebsiteType(cmd.String("type"))
	if !slices.Contains([]biz.WebsiteType{biz.WebsiteTypeProxy, biz.WebsiteTypeStatic, biz.WebsiteTypePHP}, typ) {
		return errors.New(s.t.Get("Unsupported website type: %s", cmd.String("type")))
	}

	path := cmd.String("path")
	if path == "" {
		root, err := s.settingRepo.Get(biz.SettingKeyWebsitePath)
		if err != nil {
			return err
		}
		path = filepath.Join(root, name, "public")
	}
	if !filepath.IsAbs(path) {
		return errors.New(s.t.Get("Website path must be an absolute path"))
	}

	// 已存在则覆盖，用于修复错误的网站数据
	website := new(biz.Website)
	if err := s.db.Where("name", name).First(website).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(s.t.Get("Failed to get website: %v", err))
		}
	}
	website.Name = name
	website.Type = typ
	website.Path = path
	website.Status = cmd.Bool("status")
	website.SSL = cmd.Bool("ssl")
	website.Remark = cmd.String("remark")
	if err := s.db.Save(website).Error; err != nil {
		return errors.New(s.t.Get("Failed to save website: %v", err))
	}

	fmt.Println(s.t.Get("Website %s written successfully", name))
	return nil
}

func (s *CliService) DatabaseListServer(ctx context.Context, cmd *cli.Command) error {
	servers, _, err := s.databaseServerRepo.List(ctx, 1, math.MaxUint32, "")
	if err != nil {
		return err
	}

	return s.printList(cmd, servers, func() {
		for _, server := range servers {
			fmt.Println(s.t.Get("ID: %d, Name: %s, Type: %s, Address: %s:%d, Username: %s", server.ID, server.Name, string(server.Type), server.Host, server.Port, server.Username))
		}
	})
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

	if err := s.validate(ctx, req); err != nil {
		return err
	}

	if err := s.databaseServerRepo.Create(ctx, req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Database server %s added successfully", cmd.String("name")))
	return nil
}

func (s *CliService) DatabaseDeleteServer(ctx context.Context, cmd *cli.Command) error {
	server, err := s.databaseServerRepo.GetByName(ctx, cmd.String("name"))
	if err != nil {
		return err
	}

	if err = s.databaseServerRepo.Delete(server.ID); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Database server %s deleted successfully", server.Name))
	return nil
}

func (s *CliService) CertList(ctx context.Context, cmd *cli.Command) error {
	certs, _, err := s.certRepo.List(1, math.MaxUint32)
	if err != nil {
		return err
	}

	return s.printList(cmd, certs, func() {
		for _, item := range certs {
			fmt.Println(s.t.Get("ID: %d, Domains: %s, Auto Renewal: %t, Expires At: %s", item.ID, strings.Join(item.Domains, ","), item.AutoRenewal, item.NotAfter.Format(time.DateTime)))
		}
	})
}

// CertRenew 续签证书，--all 时续签全部开启自动续签的证书
func (s *CliService) CertRenew(ctx context.Context, cmd *cli.Command) error {
	ids := []uint{}
	switch {
	case cmd.Bool("all"):
		certs, _, err := s.certRepo.List(1, math.MaxUint32)
		if err != nil {
			return err
		}
		ids = lo.FilterMap(certs, func(item *types.CertList, _ int) (uint, bool) {
			return item.ID, item.AutoRenewal
		})
	case cmd.Uint("id") != 0:
		ids = []uint{cmd.Uint("id")}
	}
	if len(ids) == 0 {
		return errors.New(s.t.Get("Please specify the certificate ID or use --all"))
	}

	var failed int
	for _, id := range ids {
		fmt.Println(s.t.Get("|-Renewing certificate %d...", id))
		if _, err := s.certRepo.RenewWithProgressCallback(ctx, id, func(msg string) {
			fmt.Println("  " + msg)
		}); err != nil {
			failed++
			fmt.Println(s.t.Get("|-Certificate %d renewal failed: %v", id, err))
			continue
		}
		fmt.Println(s.t.Get("|-Certificate %d renewed successfully", id))
	}

	if failed > 0 {
		return errors.New(s.t.Get("%d certificate(s) failed to renew", failed))
	}

	return nil
}

func (s *CliService) BackupList(ctx context.Context, cmd *cli.Command) error {
	files, err := s.backupRepo.List(biz.BackupType(cmd.String("type")))
	if err != nil {
		return err
	}

	return s.printList(cmd, files, func() {
		for _, file := range files {
			fmt.Println(s.t.Get("Name: %s, Size: %s, Time: %s", file.Name, file.Size, file.Time.Format(time.DateTime)))
		}
	})
}

func (s *CliService) BackupWebsite(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Create(ctx, biz.BackupTypeWebsite, cmd.String("name"), cmd.Uint("storage"))
}

func (s *CliService) BackupDatabase(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Create(ctx, biz.BackupType(cmd.String("type")), cmd.String("name"), cmd.Uint("storage"))
}

func (s *CliService) BackupPath(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Create(ctx, biz.BackupTypePath, cmd.String("path"), cmd.Uint("storage"))
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
		if err := s.backupRepo.ClearStorageExpired(cmd.Uint("storage"), cmd.String("type"), cmd.String("file"), cmd.Uint("keep")); err != nil {
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

func (s *CliService) RestoreWebsite(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Restore(ctx, biz.BackupTypeWebsite, cmd.String("file"), cmd.String("name"))
}

func (s *CliService) RestoreDatabase(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Restore(ctx, biz.BackupType(cmd.String("type")), cmd.String("file"), cmd.String("name"))
}

// RestorePanel 从面板备份恢复，恢复完成后面板会自行重启
func (s *CliService) RestorePanel(ctx context.Context, cmd *cli.Command) error {
	return s.backupRepo.Restore(ctx, biz.BackupTypePanel, cmd.String("file"), "")
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

	// 本地切割目录与各自的文件前缀，访问日志和错误日志分别计份
	var path string
	var prefixes []string
	switch typ {
	case "website":
		website, err := s.websiteRepo.GetByName(name)
		if err != nil {
			return err
		}
		path = filepath.Join(app.Root, "sites", website.Name, "log")
		prefixes = []string{"access_", "error_"}
	case "container":
		path = filepath.Join(app.Root, "server/cutoff/container", name)
		prefixes = []string{""}
	default:
		return errors.New(s.t.Get("Unsupported rotation type: %s", typ))
	}

	for _, prefix := range prefixes {
		if err := s.backupRepo.ClearExpired(path, prefix, keep); err != nil {
			return err
		}
		// 清理远程存储过期日志，切割日志上传在 cutoff/<类型>/<目标> 下
		if storageID != 0 {
			if err := s.backupRepo.ClearStorageExpired(storageID, filepath.Join("cutoff", typ, name), prefix, keep); err != nil {
				return err
			}
		}
	}

	fmt.Println(s.hr)
	fmt.Println(s.t.Get("☆ Cleaning successful [%s]", time.Now().Format(time.DateTime)))
	fmt.Println(s.hr)
	return nil
}

func (s *CliService) AppList(ctx context.Context, cmd *cli.Command) error {
	apps, err := s.appRepo.Installed()
	if err != nil {
		return errors.New(s.t.Get("Failed to get app list: %v", err))
	}

	return s.printList(cmd, apps, func() {
		for _, item := range apps {
			fmt.Println(s.t.Get("Slug: %s, Channel: %s, Version: %s", item.Slug, item.Channel, item.Version))
		}
	})
}

func (s *CliService) AppInstall(ctx context.Context, cmd *cli.Command) error {
	slug := cmd.Args().First()
	if slug == "" {
		return errors.New(s.t.Get("Parameters cannot be empty"))
	}
	// 未指定通道时跟随面板通道
	channel := cmd.Args().Get(1)
	if channel == "" {
		channel, _ = s.settingRepo.Get(biz.SettingKeyChannel)
		channel = cmp.Or(channel, "stable")
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
	if acme {
		conf.HTTP.TLS = "acme"
	}

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

func (s *CliService) CronList(ctx context.Context, cmd *cli.Command) error {
	crons, _, err := s.cronRepo.List(1, math.MaxUint32)
	if err != nil {
		return err
	}

	return s.printList(cmd, crons, func() {
		for _, cron := range crons {
			status := s.t.Get("stopped")
			if cron.Status {
				status = s.t.Get("running")
			}
			fmt.Println(s.t.Get("ID: %d, Name: %s, Type: %s, Schedule: %s, Status: %s", cron.ID, cron.Name, cron.Type, cron.Time, status))
		}
	})
}

// CronRun 立即执行一次计划任务，直接跑任务脚本，不经 wrapper 上报失败
func (s *CliService) CronRun(ctx context.Context, cmd *cli.Command) error {
	cron, err := s.cronRepo.Get(cmd.Uint("id"))
	if err != nil {
		return err
	}
	if !io.Exists(cron.Shell) {
		return errors.New(s.t.Get("Cron task script %s not exists", cron.Shell))
	}

	fmt.Println(s.t.Get("|-Running cron task: %s", cron.Name))
	out, err := shell.Execf("bash %s", cron.Shell)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return errors.New(s.t.Get("Cron task %s failed: %v", cron.Name, err))
	}

	fmt.Println(s.t.Get("Cron task %s executed successfully", cron.Name))
	return nil
}

func (s *CliService) CronStatus(ctx context.Context, cmd *cli.Command) error {
	status := !cmd.Bool("off")
	if err := s.cronRepo.Status(cmd.Uint("id"), status); err != nil {
		return err
	}

	msg := s.t.Get("Cron task %d disabled", cmd.Uint("id"))
	if status {
		msg = s.t.Get("Cron task %d enabled", cmd.Uint("id"))
	}

	fmt.Println(msg)
	return nil
}

// CronFailed 上报计划任务执行失败，由任务 wrapper 脚本调用
func (s *CliService) CronFailed(ctx context.Context, cmd *cli.Command) error {
	cron, err := s.cronRepo.Get(cmd.Uint("id"))
	if err != nil {
		return err
	}

	// 附带日志尾部，便于直接定位问题
	tail, _ := shell.Execf("tail -n 20 %s", cron.Log)

	return s.notifyRepo.SendEventSync(ctx, biz.NotifyEventCronFailed, s.t.Get("[AcePanel] Cron Task Failed"),
		biz.NotifyBody(s.t.Get("cron task exited abnormally"), [][2]string{
			{s.t.Get("Task"), cron.Name},
			{s.t.Get("Schedule"), cron.Time},
			{s.t.Get("Exit Code"), cast.ToString(cmd.Int("code"))},
			{s.t.Get("Log"), cron.Log},
			{s.t.Get("Output"), tail},
			{s.t.Get("Time"), time.Now().Format(time.DateTime)},
		}))
}

// validate 校验请求结构体，CLI 不走 HTTP 绑定，需要单独调用
func (s *CliService) validate(ctx context.Context, req any) error {
	vd, err := s.validator.Struct(req)
	if err != nil {
		return err
	}
	if err = vd.Validate(ctx); err != nil {
		return err
	}
	if vd.Fails() {
		return errors.New(vd.Errors().One())
	}

	return nil
}

// printList 输出列表，--json 时输出原始数据，否则交给 plain 打印
func (s *CliService) printList(cmd *cli.Command, data any, plain func()) error {
	if !cmd.Bool("json") {
		plain()
		return nil
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))

	return nil
}

// readSecret 按参数 > 环境变量 > 交互输入的优先级取密码，交互输入不回显
func (s *CliService) readSecret(value, env, prompt string) (string, error) {
	if value != "" {
		return value, nil
	}
	if fromEnv := stdos.Getenv(env); fromEnv != "" {
		return fromEnv, nil
	}

	// 非终端环境无法交互输入，直接提示改用参数或环境变量
	fd := int(stdos.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errors.New(s.t.Get("Password not provided, pass it as an argument or set the %s environment variable", env))
	}

	fmt.Print(prompt)
	secret, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", errors.New(s.t.Get("Failed to read input: %v", err))
	}

	return strings.TrimSpace(string(secret)), nil
}
