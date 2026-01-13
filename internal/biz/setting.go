package biz

import (
	"context"
	"time"

	"github.com/acepanel/panel/internal/http/request"
)

type SettingKey string

const (
	SettingKeyName                SettingKey = "name"
	SettingKeyVersion             SettingKey = "version"
	SettingKeyChannel             SettingKey = "channel"
	SettingKeyMonitor             SettingKey = "monitor"
	SettingKeyMonitorDays         SettingKey = "monitor_days"
	SettingKeyBackupPath          SettingKey = "backup_path"
	SettingKeyWebsitePath         SettingKey = "website_path"
	SettingKeyProjectPath         SettingKey = "project_path"
	SettingKeyWebsiteTLSVersions  SettingKey = "website_tls_versions"
	SettingKeyWebsiteCipherSuites SettingKey = "website_tls_cipher_suites"
	SettingKeyMySQLRootPassword   SettingKey = "mysql_root_password"
	SettingKeyOfflineMode         SettingKey = "offline_mode"
	SettingKeyAutoUpdate          SettingKey = "auto_update"
	SettingKeyWebserver           SettingKey = "webserver"
	SettingKeyPublicIPs           SettingKey = "public_ips"
	SettingHiddenMenu             SettingKey = "hidden_menu"
	SettingKeyCustomLogo          SettingKey = "custom_logo"
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
	UpdatePanel(ctx context.Context, req *request.SettingPanel) (bool, error)
	UpdateCert(req *request.SettingCert) error
}
