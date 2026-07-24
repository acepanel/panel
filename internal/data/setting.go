package data

import (
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/samber/do/v2"
	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/io"
)

type settingRepo struct {
	db   *gorm.DB
	conf *config.Config
}

func NewSettingRepo(i do.Injector) (biz.SettingRepo, error) {
	return &settingRepo{
		db:   do.MustInvoke[*gorm.DB](i),
		conf: do.MustInvoke[*config.Config](i),
	}, nil
}

func (r *settingRepo) Get(key biz.SettingKey, defaultValue ...string) (string, error) {
	value, err := r.getRaw(key)
	if err != nil {
		return "", err
	}

	if value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return value, nil
}

func (r *settingRepo) GetBool(key biz.SettingKey, defaultValue ...bool) (bool, error) {
	value, err := r.getRaw(key)
	if err != nil {
		return false, err
	}

	if value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return cast.ToBool(value), nil
}

func (r *settingRepo) GetInt(key biz.SettingKey, defaultValue ...int) (int, error) {
	value, err := r.getRaw(key)
	if err != nil {
		return 0, err
	}

	if value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return cast.ToInt(value), nil
}

func (r *settingRepo) GetSlice(key biz.SettingKey, defaultValue ...[]string) ([]string, error) {
	value, err := r.getRaw(key)
	if err != nil {
		return nil, err
	}

	// 设置值为空时提前返回
	slice := make([]string, 0)
	if value == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0], nil
		}
		return slice, nil
	}

	if err = json.Unmarshal([]byte(value), &slice); err != nil {
		return nil, err
	}
	if len(slice) == 0 && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return slice, nil
}

func (r *settingRepo) Set(key biz.SettingKey, value string) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&biz.Setting{Key: key, Value: value}).Error
}

func (r *settingRepo) SetSlice(key biz.SettingKey, value []string) error {
	v := "[]"
	if len(value) > 0 {
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		v = string(b)
	}

	return r.Set(key, v)
}

func (r *settingRepo) Delete(key biz.SettingKey) error {
	return r.db.Where("key = ?", key).Delete(new(biz.Setting)).Error
}

func (r *settingRepo) GetPanel() (*request.SettingPanel, error) {
	name, err := r.Get(biz.SettingKeyName)
	if err != nil {
		return nil, err
	}
	channel, err := r.Get(biz.SettingKeyChannel)
	if err != nil {
		return nil, err
	}
	offlineMode, err := r.GetBool(biz.SettingKeyOfflineMode)
	if err != nil {
		return nil, err
	}
	autoUpdate, err := r.GetBool(biz.SettingKeyAutoUpdate)
	if err != nil {
		return nil, err
	}
	websitePath, err := r.Get(biz.SettingKeyWebsitePath)
	if err != nil {
		return nil, err
	}
	backupPath, err := r.Get(biz.SettingKeyBackupPath)
	if err != nil {
		return nil, err
	}
	backupFormat, err := r.Get(biz.SettingKeyBackupFormat, "tar.xz")
	if err != nil {
		return nil, err
	}
	projectPath, err := r.Get(biz.SettingKeyProjectPath)
	if err != nil {
		return nil, err
	}
	containerSock, err := r.Get(biz.SettingKeyContainerSock)
	if err != nil {
		return nil, err
	}
	hiddenMenu, err := r.GetSlice(biz.SettingHiddenMenu)
	if err != nil {
		return nil, err
	}
	customLogo, err := r.Get(biz.SettingKeyCustomLogo)
	if err != nil {
		return nil, err
	}
	ipdbType, err := r.Get(biz.SettingKeyIPDBType)
	if err != nil {
		return nil, err
	}
	ipdbURL, err := r.Get(biz.SettingKeyIPDBURL)
	if err != nil {
		return nil, err
	}
	ipdbPath, err := r.Get(biz.SettingKeyIPDBPath)
	if err != nil {
		return nil, err
	}
	ip, err := r.Get(biz.SettingKeyPublicIPs)
	if err != nil {
		return nil, err
	}
	publicIP := make([]string, 0)
	if err = json.Unmarshal([]byte(ip), &publicIP); err != nil {
		return nil, err
	}

	crt, _ := io.Read(filepath.Join(app.Root, "panel/storage/cert.pem"))
	key, _ := io.Read(filepath.Join(app.Root, "panel/storage/cert.key"))

	return &request.SettingPanel{
		Name:          name,
		Channel:       channel,
		Locale:        r.conf.App.Locale,
		Entrance:      r.conf.HTTP.Entrance,
		EntranceError: r.conf.HTTP.EntranceError,
		LoginCaptcha:  r.conf.HTTP.LoginCaptcha,
		OfflineMode:   offlineMode,
		AutoUpdate:    autoUpdate,
		Lifetime:      r.conf.Session.Lifetime,
		IPHeader:      r.conf.HTTP.IPHeader,
		BindDomain:    r.conf.HTTP.BindDomain,
		BindIP:        r.conf.HTTP.BindIP,
		BindUA:        r.conf.HTTP.BindUA,
		WebsitePath:   websitePath,
		BackupPath:    backupPath,
		BackupFormat:  backupFormat,
		ProjectPath:   projectPath,
		ContainerSock: containerSock,
		HiddenMenu:    hiddenMenu,
		CustomLogo:    customLogo,
		IPDBType:      ipdbType,
		IPDBURL:       ipdbURL,
		IPDBPath:      ipdbPath,
		Port:          r.conf.HTTP.Port,
		TLS:           r.conf.HTTP.TLS,
		PublicIP:      publicIP,
		Cert:          crt,
		Key:           key,
	}, nil
}

// getRaw 从数据库获取设置项的原始字符串值
func (r *settingRepo) getRaw(key biz.SettingKey) (string, error) {
	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}
	return setting.Value, nil
}
