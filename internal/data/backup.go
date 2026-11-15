package data

import (
	"context"
	"errors"
	"fmt"
	stdio "io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/storage"
	"github.com/acepanel/panel/v3/pkg/tools"
	"github.com/acepanel/panel/v3/pkg/types"
)

type backupRepo struct {
	hr       string
	t        *gotext.Locale
	conf     *config.Config
	db       *gorm.DB
	log      *slog.Logger
	setting  biz.SettingRepo
	website  biz.WebsiteRepo
	account  biz.BackupAccountRepo
	updating atomic.Bool // 面板升级进行中标志，防止并发触发
}

func NewBackupRepo(
	conf *config.Config,
	db *gorm.DB,
	t *gotext.Locale,
	log *slog.Logger,
	settingRepo biz.SettingRepo,
	websiteRepo biz.WebsiteRepo,
	backupAccountRepo biz.BackupAccountRepo,
) biz.BackupRepo {
	return &backupRepo{
		hr:      "+----------------------------------------------------",
		t:       t,
		conf:    conf,
		db:      db,
		log:     log,
		setting: settingRepo,
		website: websiteRepo,
		account: backupAccountRepo,
	}
}

// List 备份列表
func (r *backupRepo) List(typ biz.BackupType) ([]*types.BackupFile, error) {
	path := r.GetDefaultPath(typ)
	files, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make([]*types.BackupFile, 0), nil
		}
		return nil, err
	}

	list := make([]*types.BackupFile, 0)
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			continue
		}
		list = append(list, &types.BackupFile{
			Name: file.Name(),
			Path: filepath.Join(path, file.Name()),
			Size: tools.FormatBytes(float64(info.Size())),
			Time: info.ModTime(),
		})
	}

	slices.SortFunc(list, func(a, b *types.BackupFile) int {
		return b.Time.Compare(a.Time)
	})

	return list, nil
}

// Create 创建备份
// typ 备份类型
// target 目标名称
// storage 备份存储ID
func (r *backupRepo) Create(ctx context.Context, typ biz.BackupType, target string, storage uint) error {
	// 取备份存储，0 为本地备份
	var backupStorage *biz.BackupStorage
	if storage != 0 {
		var err error
		backupStorage, err = r.account.GetByID(storage)
		if err != nil {
			return err
		}
	} else {
		backupStorage = &biz.BackupStorage{
			Name: r.t.Get("Local Storage"),
			Type: biz.BackupStorageTypeLocal,
			Info: types.BackupStorageInfo{
				Path: filepath.Dir(r.GetDefaultPath(typ)), // 需要取根目录
			},
		}
	}

	client, err := newStorage(r.t, backupStorage)
	if err != nil {
		return err
	}

	start := time.Now()
	namePrefix := target
	if typ == biz.BackupTypePath {
		namePrefix = filepath.Base(target)
	}
	name := fmt.Sprintf("%s_%s", namePrefix, start.Format("20060102150405"))
	if app.IsCli {
		fmt.Println(r.hr)
		fmt.Println(r.t.Get("★ Start backup [%s]", start.Format(time.DateTime)))
		fmt.Println(r.hr)
		fmt.Println(r.t.Get("|-Backup type: %s", string(typ)))
		fmt.Println(r.t.Get("|-Backup storage: %s", backupStorage.Name))
		fmt.Println(r.t.Get("|-Backup target: %s", target))
	}

	switch typ {
	case biz.BackupTypeWebsite:
		err = r.createWebsite(ctx, name, client, target)
	case biz.BackupTypeMySQL:
		err = r.createMySQL(ctx, name, client, target)
	case biz.BackupTypePostgres:
		err = r.createPostgres(ctx, name, client, target)
	case biz.BackupTypeClickHouse:
		err = r.createClickHouse(ctx, name, client, target)
	case biz.BackupTypeRedis:
		err = r.createRedisLike(ctx, name, client, "redis")
	case biz.BackupTypeValkey:
		err = r.createRedisLike(ctx, name, client, "valkey")
	case biz.BackupTypePath:
		err = r.createPath(ctx, name, client, target)
	default:
		return errors.New(r.t.Get("unknown backup type"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.hr)
	}
	if err != nil {
		if app.IsCli {
			fmt.Println(r.t.Get("☆ Backup failed: %v [%s]", err, time.Now().Format(time.DateTime)))
		}
		r.log.Warn(r.t.Get("backup failed"),
			slog.String("type", biz.OperationTypeBackup),
			slog.Uint64("operator_id", biz.OperatorID(ctx)),
			slog.String("backup_type", string(typ)),
			slog.String("target", target),
			slog.Any("err", err),
		)
	} else {
		if app.IsCli {
			fmt.Println(r.t.Get("☆ Backup completed [%s]", time.Now().Format(time.DateTime)))
		}
		r.log.Info(r.t.Get("backup created"),
			slog.String("type", biz.OperationTypeBackup),
			slog.Uint64("operator_id", biz.OperatorID(ctx)),
			slog.String("backup_type", string(typ)),
			slog.String("target", target),
		)
	}

	if app.IsCli {
		fmt.Println(r.hr)
	}

	return err
}

// createWebsite 创建网站备份
func (r *backupRepo) createWebsite(ctx context.Context, name string, storage storage.Storage, target string) error {
	website, err := r.website.GetByName(target)
	if err != nil {
		return err
	}

	// 创建用于压缩的临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	// 压缩网站
	name += r.backupExt()
	if err = io.Compress(ctx, website.Path, nil, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = r.upload(ctx, storage, filepath.Join(string(biz.BackupTypeWebsite), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// createPath 创建目录备份
func (r *backupRepo) createPath(ctx context.Context, name string, storage storage.Storage, target string) error {
	if !io.Exists(target) {
		return errors.New(r.t.Get("path does not exist: %s", target))
	}
	if !io.IsDir(target) {
		return errors.New(r.t.Get("path is not a directory: %s", target))
	}

	// 创建用于压缩的临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
	}

	// 压缩目录
	name += r.backupExt()
	if err = io.Compress(ctx, target, nil, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = r.upload(ctx, storage, filepath.Join(string(biz.BackupTypePath), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// Delete 删除备份
func (r *backupRepo) Delete(ctx context.Context, typ biz.BackupType, name string) error {
	path := r.GetDefaultPath(typ)

	file := filepath.Join(path, name)
	if err := io.Remove(file); err != nil {
		return err
	}

	return nil
}

// Restore 恢复备份
// typ 备份类型
// backup 备份压缩包，可以是绝对路径或者相对路径
// target 目标名称
func (r *backupRepo) Restore(ctx context.Context, typ biz.BackupType, backup, target string) error {
	if !io.Exists(backup) {
		backup = filepath.Join(r.GetDefaultPath(typ), backup)
	}
	if !io.Exists(backup) {
		return errors.New(r.t.Get("backup file not exists"))
	}

	start := time.Now()
	if app.IsCli {
		fmt.Println(r.hr)
		fmt.Println(r.t.Get("★ Start restore [%s]", start.Format(time.DateTime)))
		fmt.Println(r.hr)
		fmt.Println(r.t.Get("|-Restore type: %s", string(typ)))
		fmt.Println(r.t.Get("|-Restore target: %s", target))
		fmt.Println(r.t.Get("|-Backup file: %s", filepath.Base(backup)))
	}

	var err error
	switch typ {
	case biz.BackupTypeWebsite:
		err = r.restoreWebsite(ctx, backup, target)
	case biz.BackupTypeMySQL:
		err = r.restoreMySQL(ctx, backup, target)
	case biz.BackupTypePostgres:
		err = r.restorePostgres(ctx, backup, target)
	case biz.BackupTypeClickHouse:
		err = r.restoreClickHouse(ctx, backup, target)
	case biz.BackupTypeRedis:
		err = r.restoreRedisLike(ctx, backup, "redis")
	case biz.BackupTypeValkey:
		err = r.restoreRedisLike(ctx, backup, "valkey")
	case biz.BackupTypePanel:
		err = r.restorePanel(ctx, backup)
	default:
		if app.IsCli {
			fmt.Println(r.hr)
		}
		return errors.New(r.t.Get("unknown backup type"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Restore time: %s", time.Since(start).String()))
		fmt.Println(r.hr)
		if err != nil {
			fmt.Println(r.t.Get("☆ Restore failed: %v [%s]", err, time.Now().Format(time.DateTime)))
		} else {
			fmt.Println(r.t.Get("☆ Restore completed [%s]", time.Now().Format(time.DateTime)))
		}
		fmt.Println(r.hr)
	}

	return err
}

// restoreWebsite 恢复网站备份
func (r *backupRepo) restoreWebsite(ctx context.Context, backup, target string) error {
	website, err := r.website.GetByName(target)
	if err != nil {
		return err
	}

	// 先解压到同级暂存目录，确认产物有效后再替换，避免解压失败把原站点清空
	// 放同级目录保证与网站目录同分区，rename 才是原子的
	stage, err := os.MkdirTemp(filepath.Dir(website.Path), ".ace-restore-*")
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(stage)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Website path: %s", website.Path))
		fmt.Println(r.t.Get("|-Uncompressing backup..."))
	}
	if err = io.UnCompress(ctx, backup, stage); err != nil {
		return err
	}

	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New(r.t.Get("uncompressed backup is empty, restore aborted"))
	}

	content, err := r.selectWebsiteBackup(ctx, stage)
	if err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Replacing website files..."))
	}
	// 站点目录已删、暂存产物还没就位时被取消，会连 defer 一起把还原内容也清掉，这段不可取消
	replaceCtx := context.WithoutCancel(ctx)
	if err = io.Remove(website.Path); err != nil {
		return err
	}
	if err = os.Rename(content, website.Path); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Fixing file permissions..."))
	}
	if err = io.ChmodR(replaceCtx, website.Path, 0755); err != nil {
		return err
	}
	_ = io.ChownR(replaceCtx, website.Path, "www", "www")

	return nil
}

// ClearExpired 清理过期备份
// path 备份目录绝对路径
// prefix 目标文件前缀
// save 保存份数
func (r *backupRepo) ClearExpired(path, prefix string, save uint) error {
	// path 是绝对路径，以 / 为根的本地存储器就能和备份账号走同一套清理
	client, err := storage.NewLocal("/")
	if err != nil {
		return err
	}

	return r.clearExpired(context.Background(), client, path, prefix, save)
}

// ClearStorageExpired 清理备份账号过期备份
// dir 存储器内的目标目录，备份为 <类型>，切割日志为 cutoff/<类型>/<目标>
// prefix 目标文件前缀
// save 保存份数
func (r *backupRepo) ClearStorageExpired(ctx context.Context, account uint, dir, prefix string, save uint) error {
	client, err := r.remoteStorage(account)
	if err != nil {
		return err
	}

	return r.clearExpired(ctx, client, dir, prefix, save)
}

func (r *backupRepo) clearExpired(ctx context.Context, client storage.Storage, dir, prefix string, save uint) error {
	files, err := client.List(ctx, dir)
	if err != nil {
		return err
	}

	files = lo.Filter(files, func(file storage.File, _ int) bool {
		return strings.HasPrefix(file.Name, prefix) && r.isBackupArchive(file.Name)
	})
	// 排序所有备份文件，从新到旧
	slices.SortFunc(files, func(a, b storage.File) int {
		return b.ModTime.Compare(a.ModTime)
	})
	if uint(len(files)) <= save {
		return nil
	}

	// 切片保留 save 份，删除剩余
	// 已判定过期的文件删到一半就停会留下无人认领的残留，这段不可取消
	deleteCtx := context.WithoutCancel(ctx)
	for _, file := range files[save:] {
		filePath := filepath.Join(dir, file.Name)
		if app.IsCli {
			fmt.Println(r.t.Get("|-Cleaning expired file: %s", filePath))
		}
		if err = client.Delete(deleteCtx, filePath); err != nil {
			return errors.New(r.t.Get("Cleanup failed: %v", err))
		}
	}

	return nil
}

// CutoffLog 切割日志
// path 保存目录绝对路径
// target 待切割日志文件绝对路径
func (r *backupRepo) CutoffLog(ctx context.Context, path, target string) (string, error) {
	if !io.Exists(target) {
		return "", errors.New(r.t.Get("log file %s not exists", target))
	}

	name := strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
	to := filepath.Join(path, fmt.Sprintf("%s_%s%s", name, time.Now().Format("20060102150405"), r.backupExt()))
	if err := io.Compress(ctx, filepath.Dir(target), []string{filepath.Base(target)}, to); err != nil {
		return "", err
	}

	// 原文件不能直接删除，直接删的话仍会占用空间直到重启相关的应用
	if _, err := shell.Execf(ctx, "cat /dev/null > '%s'", target); err != nil {
		return "", err
	}

	return to, nil
}

// CutoffUpload 将指定的切割日志文件上传到远程存储
func (r *backupRepo) CutoffUpload(
	ctx context.Context,
	account uint,
	typ biz.BackupType,
	name string,
	files []string,
) error {
	client, err := r.remoteStorage(account)
	if err != nil {
		return err
	}

	for _, localPath := range files {
		file, err := os.Open(localPath)
		if err != nil {
			return err
		}
		remotePath := filepath.Join("cutoff", string(typ), name, filepath.Base(localPath))
		if putErr := r.upload(ctx, client, remotePath, file); putErr != nil {
			_ = file.Close()
			return putErr
		}
		_ = file.Close()
	}

	return nil
}

// GetDefaultPath 获取默认备份路径
func (r *backupRepo) GetDefaultPath(typ biz.BackupType) string {
	backupPath, err := r.setting.Get(biz.SettingKeyBackupPath)
	if err != nil {
		backupPath = filepath.Join(app.Root, "backup")
	}
	path := filepath.Join(backupPath, string(typ))
	_ = os.MkdirAll(path, 0755)
	return path
}

// tmpDir 创建备份临时目录，优先使用 TMPDIR（任务队列为取消清理设置的独享目录），
// 未设置时使用面板目录，避免大文件撑爆 /tmp（常为内存盘）
func (r *backupRepo) tmpDir() (string, error) {
	base := os.Getenv("TMPDIR")
	if base == "" {
		base = filepath.Join(app.Root, "tmp")
		if err := os.MkdirAll(base, 0755); err != nil {
			return "", err
		}
	}
	return os.MkdirTemp(base, "ace-backup-*")
}

// backupExt 根据全局设置返回备份文件扩展名
func (r *backupRepo) backupExt() string {
	format, _ := r.setting.Get(biz.SettingKeyBackupFormat, "tar.xz")
	return "." + format
}

// upload 上传备份文件到存储器
// 中断留下的截断文件会被只认前缀和后缀的过期清理当成有效备份，因此先传临时名、成功后再改名
// S3 不支持改名也不需要：分片上传在失败时 abort 清理，对象在 complete 之前不存在
func (r *backupRepo) upload(ctx context.Context, client storage.Storage, path string, content stdio.Reader) error {
	renamer, ok := client.(storage.Renamer)
	if !ok {
		return client.Put(ctx, path, content)
	}

	tmp := path + ".partial"
	err := client.Put(ctx, tmp, content)
	if err == nil {
		err = renamer.Rename(ctx, tmp, path)
	}
	if err != nil {
		// 清理不跟随取消，否则远端会留下无人认领的半截文件
		_ = client.Delete(context.WithoutCancel(ctx), tmp)
	}

	return err
}

func (r *backupRepo) remoteStorage(id uint) (storage.Storage, error) {
	account, err := r.account.GetByID(id)
	if err != nil {
		return nil, err
	}

	return newStorage(r.t, account)
}

// isBackupArchive 判断文件名是否是已知的备份压缩包后缀
func (r *backupRepo) isBackupArchive(name string) bool {
	for _, ext := range []string{".tar.xz", ".tar.gz", ".tar.zst", ".zip", ".7z"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
