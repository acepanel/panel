package biz

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/collect"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/types"
)

type BackupType string

const (
	BackupTypePath       BackupType = "path"
	BackupTypeWebsite    BackupType = "website"
	BackupTypeMySQL      BackupType = "mysql"
	BackupTypePostgres   BackupType = "postgresql"
	BackupTypeClickHouse BackupType = "clickhouse"
	BackupTypeRedis      BackupType = "redis"
	BackupTypeValkey     BackupType = "valkey"
	BackupTypePanel      BackupType = "panel"
)

// {目标}_{20060102150405}{扩展名}
var backupNamePattern = regexp.MustCompile(`^(.+)_(\d{14})(\..*)?$`)

// PanelRelease 更新渠道内最新的面板版本，本机架构没有安装包时 URL 与 Checksum 为空
type PanelRelease struct {
	Version  string
	URL      string
	Checksum string
}

// Newer 报告该版本是否比运行中的面板新
func (r *PanelRelease) Newer() (bool, error) {
	current, err := version.NewVersion(app.Version)
	if err != nil {
		return false, err
	}
	latest, err := version.NewVersion(r.Version)
	if err != nil {
		return false, err
	}

	return latest.GreaterThan(current), nil
}

type BackupRepo interface {
	List(typ BackupType) ([]*types.BackupFile, error)
	Create(ctx context.Context, typ BackupType, target string, account uint) error
	CreatePanel(ctx context.Context) error
	Delete(ctx context.Context, typ BackupType, name string) error
	Restore(ctx context.Context, typ BackupType, backup, target string) error
	ClearExpired(path, prefix string, save uint) error
	ClearStorageExpired(ctx context.Context, account uint, dir, prefix string, save uint) error
	CutoffLog(ctx context.Context, path, target string) (string, error)
	CutoffUpload(ctx context.Context, account uint, typ BackupType, name string, files []string) error
	GetDefaultPath(typ BackupType) string
	FixPanel(ctx context.Context) error
	UpdatePanel(ctx context.Context, version, url, checksum string, progress func(string)) error
}

type BackupUsecase struct {
	repo    BackupRepo
	cache   CacheRepo
	setting SettingRepo
	conf    *config.Config
	log     *slog.Logger
	notify  *NotifyUsecase
	t       *gotext.Locale
}

func NewBackupUsecase(
	notifyUsecase *NotifyUsecase,
	conf *config.Config,
	t *gotext.Locale,
	log *slog.Logger,
	backupRepo BackupRepo,
	cacheRepo CacheRepo,
	settingRepo SettingRepo,
) *BackupUsecase {
	return &BackupUsecase{
		repo:    backupRepo,
		cache:   cacheRepo,
		setting: settingRepo,
		conf:    conf,
		log:     log,
		notify:  notifyUsecase,
		t:       t,
	}
}

func (uc *BackupUsecase) List(typ BackupType) ([]*types.BackupFile, error) {
	return uc.repo.List(typ)
}

// ListGroup 时间点取文件名中的时间，上传或拷贝来的旧备份修改时间不可信
func (uc *BackupUsecase) ListGroup(typ BackupType) ([]*types.BackupGroup, error) {
	files, err := uc.repo.List(typ)
	if err != nil {
		return nil, err
	}

	var groups []*types.BackupGroup
	targets := make(map[string]*types.BackupGroup)
	for _, file := range files {
		target, at, ok := uc.parseBackupName(file.Name)
		if !ok {
			groups = append(groups, &types.BackupGroup{Name: file.Name, Items: []*types.BackupFile{file}})
			continue
		}
		file.Time = at
		if group, exists := targets[target]; exists {
			group.Items = append(group.Items, file)
			continue
		}
		targets[target] = &types.BackupGroup{Name: target, Items: []*types.BackupFile{file}}
		groups = append(groups, targets[target])
	}

	// 同一秒的备份按文件名定序，保证翻页时顺序稳定
	newest := func(a, b *types.BackupFile) int {
		return cmp.Or(b.Time.Compare(a.Time), cmp.Compare(a.Name, b.Name))
	}
	for _, group := range groups {
		slices.SortFunc(group.Items, newest)
	}
	slices.SortFunc(groups, func(a, b *types.BackupGroup) int {
		return newest(a.Items[0], b.Items[0])
	})

	return groups, nil
}

func (uc *BackupUsecase) Create(ctx context.Context, typ BackupType, target string, account uint) error {
	err := uc.repo.Create(ctx, typ, target, account)
	if err == nil {
		return nil
	}

	// 定时备份由 CLI 执行，命令返回即退出，异步通知来不及发出，必须同步发送
	if sendErr := uc.notify.SendEventSync(ctx, NotifyEventBackup, uc.t.Get("[AcePanel] Backup Failed"), NotifyBody(uc.t.Get("backup task failed"), [][2]string{
		{uc.t.Get("Type"), string(typ)},
		{uc.t.Get("Target"), target},
		{uc.t.Get("Error"), err.Error()},
		{uc.t.Get("Time"), time.Now().Format(time.DateTime)},
	})); sendErr != nil {
		uc.log.Warn(uc.t.Get("failed to send backup failure notification"), slog.Any("err", sendErr))
	}

	return err
}

func (uc *BackupUsecase) CreatePanel(ctx context.Context) error {
	return uc.repo.CreatePanel(ctx)
}

func (uc *BackupUsecase) Delete(ctx context.Context, typ BackupType, name string) error {
	if err := uc.repo.Delete(ctx, typ, name); err != nil {
		return err
	}

	// 记录日志
	uc.log.Warn(uc.t.Get("backup deleted"), slog.String("type", OperationTypeBackup), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("backup_type", string(typ)), slog.String("name", name))

	return nil
}

func (uc *BackupUsecase) Restore(ctx context.Context, typ BackupType, backup, target string) error {
	if err := uc.repo.Restore(ctx, typ, backup, target); err != nil {
		return err
	}

	// 记录日志
	uc.log.Info(uc.t.Get("backup restored"),
		slog.String("type", OperationTypeBackup),
		slog.Uint64("operator_id", OperatorID(ctx)),
		slog.String("backup_type", string(typ)),
		slog.String("target", target),
		slog.String("file", filepath.Base(backup)),
	)

	return nil
}

func (uc *BackupUsecase) ClearExpired(path, prefix string, save uint) error {
	return uc.repo.ClearExpired(path, prefix, save)
}

func (uc *BackupUsecase) ClearStorageExpired(ctx context.Context, account uint, dir, prefix string, save uint) error {
	return uc.repo.ClearStorageExpired(ctx, account, dir, prefix, save)
}

func (uc *BackupUsecase) CutoffLog(ctx context.Context, path, target string) (string, error) {
	return uc.repo.CutoffLog(ctx, path, target)
}

func (uc *BackupUsecase) CutoffUpload(
	ctx context.Context,
	account uint,
	typ BackupType,
	name string,
	files []string,
) error {
	return uc.repo.CutoffUpload(ctx, account, typ, name, files)
}

func (uc *BackupUsecase) GetDefaultPath(typ BackupType) string {
	return uc.repo.GetDefaultPath(typ)
}

func (uc *BackupUsecase) FixPanel(ctx context.Context) error {
	return uc.repo.FixPanel(ctx)
}

// LatestPanel 取更新渠道内最新的面板版本及其安装包地址
func (uc *BackupUsecase) LatestPanel() (*PanelRelease, error) {
	channel, _ := uc.setting.Get(SettingKeyChannel)
	latest, err := uc.cache.FetchLatestVersion(channel)
	if err != nil {
		return nil, err
	}

	release := &PanelRelease{Version: latest.Version}
	if download := collect.First(latest.Downloads); download != nil {
		release.URL = fmt.Sprintf("https://%s%s", uc.conf.App.DownloadEndpoint, download.URL)
		release.Checksum = fmt.Sprintf("https://%s%s", uc.conf.App.DownloadEndpoint, download.Checksum)
	}

	return release, nil
}

// PanelChangelog 取运行中的版本之后更新渠道内发布的全部版本
func (uc *BackupUsecase) PanelChangelog() (*api.Versions, error) {
	channel, _ := uc.setting.Get(SettingKeyChannel)
	return uc.cache.FetchIntermediateVersions(channel)
}

// CurrentPanelChangelog 取运行中版本自身的更新日志，上游没有这个版本（如开发版）时为空
func (uc *BackupUsecase) CurrentPanelChangelog() (*api.Versions, error) {
	current, err := version.NewVersion(app.Version)
	if err != nil {
		return nil, err
	}

	// 上游按类型分开列出版本，预发布版只在 beta 里
	versions, err := uc.cache.FetchVersions(lo.Ternary(current.Prerelease() == "", "stable", "beta"))
	if err != nil {
		return nil, err
	}

	return new(lo.Filter(*versions, func(item api.Version, _ int) bool {
		return item.Version == app.Version
	})), nil
}

func (uc *BackupUsecase) UpdatePanel(ctx context.Context, release *PanelRelease, progress func(string)) error {
	fromVersion := app.Version
	if err := uc.repo.UpdatePanel(ctx, release.Version, release.URL, release.Checksum, progress); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("panel updated"), slog.String("type", OperationTypePanel), slog.Uint64("operator_id", OperatorID(ctx)), slog.String("from_version", fromVersion), slog.String("version", release.Version))

	return nil
}

func (uc *BackupUsecase) parseBackupName(name string) (string, time.Time, bool) {
	matches := backupNamePattern.FindStringSubmatch(name)
	if matches == nil {
		return "", time.Time{}, false
	}
	at, err := time.ParseInLocation("20060102150405", matches[2], time.Local)
	if err != nil {
		return "", time.Time{}, false
	}
	return matches[1], at, true
}
