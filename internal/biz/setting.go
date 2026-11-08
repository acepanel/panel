package biz

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/cert"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/os"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type SettingKey string

const (
	SettingKeyName                      SettingKey = "name"
	SettingKeyVersion                   SettingKey = "version"
	SettingKeyChannel                   SettingKey = "channel"
	SettingKeyMonitor                   SettingKey = "monitor"
	SettingKeyMonitorDays               SettingKey = "monitor_days"
	SettingKeyMonitorInterval           SettingKey = "monitor_interval"
	SettingKeyBackupPath                SettingKey = "backup_path"
	SettingKeyBackupFormat              SettingKey = "backup_format" // tar.xz / tar.gz / tar.zst / zip / 7z
	SettingKeyWebsitePath               SettingKey = "website_path"
	SettingKeyProjectPath               SettingKey = "project_path"
	SettingKeyContainerSock             SettingKey = "container_sock"
	SettingKeyWebsiteTLSVersions        SettingKey = "website_tls_versions"
	SettingKeyWebsiteListenIPv6         SettingKey = "website_listen_ipv6"
	SettingKeyMySQLRootPassword         SettingKey = "mysql_root_password"
	SettingKeyPostgresPassword          SettingKey = "postgres_password"
	SettingKeyMongoDBAdminPassword      SettingKey = "mongodb_admin_password"
	SettingKeyClickHouseDefaultPassword SettingKey = "clickhouse_default_password"
	SettingKeyStalwartAdminUser         SettingKey = "stalwart_admin_user"
	SettingKeyStalwartAdminPassword     SettingKey = "stalwart_admin_password"
	SettingKeyStalwartCertID            SettingKey = "stalwart_cert_id"     // 绑定到邮件服务的面板证书 ID
	SettingKeyStalwartCertObject        SettingKey = "stalwart_cert_object" // Stalwart 侧对应的证书对象 ID
	SettingKeyStalwartProxyWebsite      SettingKey = "stalwart_proxy_website" // 一键创建的反代网站 ID
	SettingKeyOfflineMode               SettingKey = "offline_mode"
	SettingKeyAutoUpdate                SettingKey = "auto_update"
	SettingKeyWebserver                 SettingKey = "webserver"
	SettingKeyPublicIPs                 SettingKey = "public_ips"
	SettingHiddenMenu                   SettingKey = "hidden_menu"
	SettingKeyCustomLogo                SettingKey = "custom_logo"
	SettingKeyMemo                      SettingKey = "memo"
	SettingKeyScanAware                 SettingKey = "scan_aware"
	SettingKeyScanAwareDays             SettingKey = "scan_aware_days"
	SettingKeyScanAwareInterfaces       SettingKey = "scan_aware_interfaces"
	SettingKeyScanAwareAutoBlock        SettingKey = "scan_aware_auto_block"
	SettingKeyScanAwareBlockThreshold   SettingKey = "scan_aware_block_threshold"
	SettingKeyScanAwareBlockWindow      SettingKey = "scan_aware_block_window"
	SettingKeyScanAwareBlockDuration    SettingKey = "scan_aware_block_duration" // 小时，0=永久
	SettingKeyScanAwareWhitelist        SettingKey = "scan_aware_whitelist"      // JSON 数组
	SettingKeyWebsiteStatDays           SettingKey = "website_stat_days"
	SettingKeyWebsiteStatErrBufMax      SettingKey = "website_stat_err_buf_max"
	SettingKeyWebsiteStatUVMaxKeys      SettingKey = "website_stat_uv_max_keys"
	SettingKeyWebsiteStatIPMaxKeys      SettingKey = "website_stat_ip_max_keys"
	SettingKeyWebsiteStatDetailMaxKeys  SettingKey = "website_stat_detail_max_keys"
	SettingKeyWebsiteStatBodyEnabled    SettingKey = "website_stat_body_enabled"
	SettingKeyIPDBType                  SettingKey = "ipdb_type" // "" / "custom" / "subscribe"
	SettingKeyIPDBURL                   SettingKey = "ipdb_url"  // 订阅链接
	SettingKeyIPDBPath                  SettingKey = "ipdb_path"
	SettingKeyInfoRan                   SettingKey = "info_ran" // info 命令是否已运行过
	SettingKeyTamperEnabled             SettingKey = "tamper_enabled"
	SettingKeyTamperMode                SettingKey = "tamper_mode"           // chattr / ebpf
	SettingKeyTamperBlockNew            SettingKey = "tamper_block_new"      // 新建受保护类型文件时删除拦截
	SettingKeyTamperLogDays             SettingKey = "tamper_log_days"       // 拦截日志保留天数
	SettingKeyNotifyEvents              SettingKey = "notify_event_types"    // 订阅的系统事件类型，JSON 数组
	SettingKeyNotifyEventChannels       SettingKey = "notify_event_channels" // 接收系统事件的渠道 ID，JSON 数组
	SettingKeyAlertLogDays              SettingKey = "alert_log_days"        // 告警记录保留天数
)

type Setting struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Key       SettingKey `gorm:"not null;default:'';unique" json:"key"`
	Value     string     `gorm:"not null;default:''" json:"value"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type SettingRepo interface {
	Get(key SettingKey, defaultValue ...string) (string, error)
	GetBool(key SettingKey, defaultValue ...bool) (bool, error)
	GetInt(key SettingKey, defaultValue ...int) (int, error)
	GetSlice(key SettingKey, defaultValue ...[]string) ([]string, error)
	Set(key SettingKey, value string) error
	SetSlice(key SettingKey, value []string) error
	Delete(key SettingKey) error
	GetPanel() (*request.SettingPanel, error)
}

type SettingUsecase struct {
	repo SettingRepo
	task TaskRepo
	t    *gotext.Locale
	log  *slog.Logger
}

func NewSettingUsecase(t *gotext.Locale, log *slog.Logger, settingRepo SettingRepo, taskRepo TaskRepo) *SettingUsecase {
	return &SettingUsecase{
		repo: settingRepo,
		task: taskRepo,
		t:    t,
		log:  log,
	}
}

func (uc *SettingUsecase) Get(key SettingKey, defaultValue ...string) (string, error) {
	return uc.repo.Get(key, defaultValue...)
}

func (uc *SettingUsecase) GetBool(key SettingKey, defaultValue ...bool) (bool, error) {
	return uc.repo.GetBool(key, defaultValue...)
}

func (uc *SettingUsecase) GetInt(key SettingKey, defaultValue ...int) (int, error) {
	return uc.repo.GetInt(key, defaultValue...)
}

func (uc *SettingUsecase) GetSlice(key SettingKey, defaultValue ...[]string) ([]string, error) {
	return uc.repo.GetSlice(key, defaultValue...)
}

func (uc *SettingUsecase) Set(key SettingKey, value string) error {
	return uc.repo.Set(key, value)
}

func (uc *SettingUsecase) SetSlice(key SettingKey, value []string) error {
	return uc.repo.SetSlice(key, value)
}

func (uc *SettingUsecase) Delete(key SettingKey) error {
	return uc.repo.Delete(key)
}

func (uc *SettingUsecase) GetPanel() (*request.SettingPanel, error) {
	return uc.repo.GetPanel()
}

func (uc *SettingUsecase) UpdatePanel(ctx context.Context, req *request.SettingPanel) (bool, error) {
	if err := uc.repo.Set(SettingKeyName, req.Name); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyChannel, req.Channel); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyOfflineMode, cast.ToString(req.OfflineMode)); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyAutoUpdate, cast.ToString(req.AutoUpdate)); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyWebsitePath, req.WebsitePath); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyBackupPath, req.BackupPath); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyBackupFormat, req.BackupFormat); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyProjectPath, req.ProjectPath); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyContainerSock, req.ContainerSock); err != nil {
		return false, err
	}
	if err := uc.repo.SetSlice(SettingHiddenMenu, req.HiddenMenu); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyCustomLogo, req.CustomLogo); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyIPDBType, req.IPDBType); err != nil {
		return false, err
	}
	ipdbURL := req.IPDBURL
	if req.IPDBType == "subscribe" && ipdbURL == "" {
		// https://github.com/metowolf/qqwry.ipdb
		ipdbURL = "https://fastly.jsdelivr.net/npm/qqwry.ipdb/qqwry.ipdb"
	}
	if err := uc.repo.Set(SettingKeyIPDBURL, ipdbURL); err != nil {
		return false, err
	}
	if err := uc.repo.Set(SettingKeyIPDBPath, req.IPDBPath); err != nil {
		return false, err
	}
	if err := uc.repo.SetSlice(SettingKeyPublicIPs, req.PublicIP); err != nil {
		return false, err
	}

	// 订阅模式后台下载 IPDB
	if req.IPDBType == "subscribe" && ipdbURL != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Minute)
			defer cancel()
			destPath := filepath.Join(app.Root, "panel/storage/geo.ipdb")
			if err := tools.DownloadFile(ctx, ipdbURL, destPath); err != nil {
				uc.log.Warn(uc.t.Get("failed to download ipdb"), slog.String("url", ipdbURL), slog.Any("err", err))
			} else {
				uc.log.Info(uc.t.Get("ipdb downloaded"), slog.String("url", ipdbURL))
			}
		}()
	}

	// 下面是需要需要重启的设置
	// 面板HTTPS
	restartFlag := false

	// 自签模式
	if req.TLS == "self-signed" {
		needGen := req.Cert == "" || req.Key == ""
		if !needGen {
			if _, err := cert.ParseCert([]byte(req.Cert)); err != nil {
				needGen = true
			}
		}
		if needGen {
			crt, key, err := cert.GenerateSelfSigned(tools.CollectLocalNames()) //nolint:contextcheck
			if err != nil {
				return false, errors.New(uc.t.Get("failed to generate self-signed certificate: %v", err))
			}
			req.Cert = string(crt)
			req.Key = string(key)
		}
	}

	oldCert, _ := io.Read(filepath.Join(app.Root, "panel/storage/cert.pem"))
	oldKey, _ := io.Read(filepath.Join(app.Root, "panel/storage/cert.key"))
	if oldCert != req.Cert || oldKey != req.Key {
		if uc.task.HasRunningTask() {
			return false, errors.New(uc.t.Get("background task is running, modifying some settings is prohibited, please try again later"))
		}
		restartFlag = true
	}
	// custom 模式需要验证证书格式
	if req.TLS == "custom" {
		if _, err := cert.ParseCert([]byte(req.Cert)); err != nil {
			return false, errors.New(uc.t.Get("failed to parse certificate: %v", err))
		}
		if _, err := cert.ParseKey([]byte(req.Key)); err != nil {
			return false, errors.New(uc.t.Get("failed to parse private key: %v", err))
		}
	}
	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), req.Cert, 0600); err != nil {
		return false, err
	}
	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), req.Key, 0600); err != nil {
		return false, err
	}

	// 面板主配置
	conf, err := config.Load()
	if err != nil {
		return false, err
	}

	if req.Port != conf.HTTP.Port {
		if os.TCPPortInUse(req.Port) { //nolint:contextcheck
			return false, errors.New(uc.t.Get("port is already in use"))
		}
		// 放行端口与后面的保存配置、重启是一个整体，中途取消会让面板监听在被防火墙拦截的端口上，管理员直接失联
		ctx = context.WithoutCancel(ctx)
		fw := firewall.NewFirewall()
		running, statusErr := fw.Status(ctx)
		if statusErr != nil {
			return false, statusErr
		}
		if running {
			if err = fw.Port(ctx, firewall.FireInfo{
				Type:      firewall.TypeNormal,
				PortStart: req.Port,
				PortEnd:   req.Port,
				Protocol:  firewall.ProtocolTCPUDP,
				Strategy:  firewall.StrategyAccept,
				Direction: firewall.DirectionIn,
			}, firewall.OperationAdd); err != nil {
				return false, err
			}
		}
	}

	conf.App.Locale = req.Locale
	conf.HTTP.Port = req.Port
	conf.HTTP.Entrance = "/" + strings.Trim(req.Entrance, "/")
	conf.HTTP.EntranceError = req.EntranceError
	conf.HTTP.LoginCaptcha = req.LoginCaptcha
	conf.HTTP.TLS = req.TLS
	conf.HTTP.IPHeader = req.IPHeader
	conf.HTTP.BindDomain = req.BindDomain
	conf.HTTP.BindIP = req.BindIP
	conf.HTTP.BindUA = req.BindUA
	conf.Session.Lifetime = req.Lifetime

	// 检查配置是否有变更
	if same, _ := config.Check(conf); !same {
		if uc.task.HasRunningTask() {
			return false, errors.New(uc.t.Get("background task is running, modifying some settings is prohibited, please try again later"))
		}
		restartFlag = true
	}
	if err = config.Save(conf); err != nil {
		return false, err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("panel settings updated"), slog.String("type", OperationTypeSetting), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("name", req.Name), slog.Uint64("port", uint64(req.Port)), slog.String("locale", req.Locale), slog.String("tls", req.TLS), slog.Uint64("lifetime", uint64(req.Lifetime)), slog.Any("bind_domain", req.BindDomain), slog.Bool("restart", restartFlag))

	return restartFlag, nil
}

func (uc *SettingUsecase) UpdateCert(req *request.SettingCert) error {
	if uc.task.HasRunningTask() {
		return errors.New(uc.t.Get("background task is running, modifying some settings is prohibited, please try again later"))
	}
	if _, err := cert.ParseCert([]byte(req.Cert)); err != nil {
		return errors.New(uc.t.Get("failed to parse certificate: %v", err))
	}
	if _, err := cert.ParseKey([]byte(req.Key)); err != nil {
		return errors.New(uc.t.Get("failed to parse private key: %v", err))
	}

	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), req.Cert, 0600); err != nil {
		return err
	}
	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), req.Key, 0600); err != nil {
		return err
	}

	return nil
}
