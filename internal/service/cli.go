package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	stdos "os"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/validator"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type CliService struct {
	hr                 string
	t                  *gotext.Locale
	log                *slog.Logger
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
	validator          *validator.Validator
}

func NewCliService(
	appUsecase *biz.AppUsecase,
	backupUsecase *biz.BackupUsecase,
	cacheUsecase *biz.CacheUsecase,
	certAccountUsecase *biz.CertAccountUsecase,
	certUsecase *biz.CertUsecase,
	cronUsecase *biz.CronUsecase,
	databaseServerUsecase *biz.DatabaseServerUsecase,
	notifyUsecase *biz.NotifyUsecase,
	settingUsecase *biz.SettingUsecase,
	userPasskeyUsecase *biz.UserPasskeyUsecase,
	userUsecase *biz.UserUsecase,
	websiteUsecase *biz.WebsiteUsecase,
	conf *config.Config,
	db *gorm.DB,
	t *gotext.Locale,
	log *slog.Logger,
	v *validator.Validator,
) *CliService {
	return &CliService{
		hr:                 `+----------------------------------------------------`,
		t:                  t,
		log:                log,
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
	}
}

// validate 校验请求结构体，CLI 不走 HTTP 绑定，需要单独调用
func (s *CliService) validate(ctx context.Context, req any) error {
	vd, err := s.validator.Struct(req)
	if err != nil {
		return err
	}
	if err = vd.Validate(ctx); err != nil {
		if errs, ok := validator.AsErrors(err); ok {
			return errors.New(errs.One())
		}
		return err
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

// printAccessAddresses 输出面板的本机与公网访问地址
func (s *CliService) printAccessAddresses() {
	protocol := "http"
	if s.conf.HTTP.IsHTTPS() {
		protocol = "https"
	}
	port, entrance := s.conf.HTTP.Port, s.conf.HTTP.Entrance

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
}

// updateConfig 读取配置文件，修改后写回并同步到 s.conf
func (s *CliService) updateConfig(update func(conf *config.Config)) error {
	conf, err := config.Load()
	if err != nil {
		return err
	}
	update(conf)
	if err = config.Save(conf); err != nil {
		return err
	}
	*s.conf = *conf
	return nil
}
