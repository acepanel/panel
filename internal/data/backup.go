package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/do/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/db"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/storage"
	"github.com/acepanel/panel/v3/pkg/systemctl"
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
	updating atomic.Bool // 面板升级进行中标志，防止并发触发
}

func NewBackupRepo(i do.Injector) (biz.BackupRepo, error) {
	return &backupRepo{
		hr:      "+----------------------------------------------------",
		t:       do.MustInvoke[*gotext.Locale](i),
		conf:    do.MustInvoke[*config.Config](i),
		db:      do.MustInvoke[*gorm.DB](i),
		log:     do.MustInvoke[*slog.Logger](i),
		setting: do.MustInvoke[biz.SettingRepo](i),
		website: do.MustInvoke[biz.WebsiteRepo](i),
	}, nil
}

// GetStorage 获取备份存储
func (r *backupRepo) GetStorage(id uint) (*biz.BackupStorage, error) {
	backupStorage := new(biz.BackupStorage)
	if err := r.db.First(backupStorage, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(r.t.Get("backup storage not found"))
		}
		return nil, err
	}
	return backupStorage, nil
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
		backupStorage, err = r.GetStorage(storage)
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

	client, err := r.getStorage(*backupStorage)
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
		err = r.createWebsite(name, client, target)
	case biz.BackupTypeMySQL:
		err = r.createMySQL(name, client, target)
	case biz.BackupTypePostgres:
		err = r.createPostgres(name, client, target)
	case biz.BackupTypeClickHouse:
		err = r.createClickHouse(name, client, target)
	case biz.BackupTypeRedis:
		err = r.createRedisLike(name, client, "redis")
	case biz.BackupTypeValkey:
		err = r.createRedisLike(name, client, "valkey")
	case biz.BackupTypePath:
		err = r.createPath(name, client, target)
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
		r.log.Warn("backup failed",
			slog.String("type", biz.OperationTypeBackup),
			slog.Uint64("operator_id", getOperatorID(ctx)),
			slog.String("backup_type", string(typ)),
			slog.String("target", target),
		)
	} else {
		if app.IsCli {
			fmt.Println(r.t.Get("☆ Backup completed [%s]", time.Now().Format(time.DateTime)))
		}
		r.log.Info("backup created",
			slog.String("type", biz.OperationTypeBackup),
			slog.Uint64("operator_id", getOperatorID(ctx)),
			slog.String("backup_type", string(typ)),
			slog.String("target", target),
		)
	}

	if app.IsCli {
		fmt.Println(r.hr)
	}

	return err
}

// CreatePanel 创建面板备份
// 面板备份始终保存在本地
func (r *backupRepo) CreatePanel() error {
	start := time.Now()

	backup := filepath.Join(r.GetDefaultPath(biz.BackupTypePanel), fmt.Sprintf("panel_%s.tar.xz", time.Now().Format("20060102150405")))

	// 备份前 checkpoint 主库
	_ = r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error

	// 只备份恢复面板运行所必需的核心文件；checkpoint 后 -wal 已并入 panel.db、
	// -shm 为纯临时共享内存，均无需备份
	var files []string
	for _, f := range []string{
		"panel/ace",
		"panel/storage/panel.db",
		"panel/storage/config.yml",
		"panel/storage/cert.pem",
		"panel/storage/cert.key",
		"panel/storage/customize",
	} {
		if io.Exists(filepath.Join(app.Root, f)) {
			files = append(files, f)
		}
	}

	// 两个 -C 把 panel 内的核心文件与 panel 外的 cli 二进制收进同一个包
	if _, err := shell.Execf(
		"tar -cJf '%s' -C '%s' %s -C /usr/local/sbin acepanel",
		backup, app.Root, strings.Join(files, " "),
	); err != nil {
		return err
	}
	if err := io.Chmod(backup, 0600); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.t.Get("|-Backup file: %s", filepath.Base(backup)))
	}

	return nil
}

// Delete 删除备份
func (r *backupRepo) Delete(typ biz.BackupType, name string) error {
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
func (r *backupRepo) Restore(typ biz.BackupType, backup, target string) error {
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
		err = r.restoreWebsite(backup, target)
	case biz.BackupTypeMySQL:
		err = r.restoreMySQL(backup, target)
	case biz.BackupTypePostgres:
		err = r.restorePostgres(backup, target)
	case biz.BackupTypeClickHouse:
		err = r.restoreClickHouse(backup, target)
	case biz.BackupTypeRedis:
		err = r.restoreRedisLike(backup, "redis")
	case biz.BackupTypeValkey:
		err = r.restoreRedisLike(backup, "valkey")
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

// CutoffLog 切割日志
// path 保存目录绝对路径
// target 待切割日志文件绝对路径
func (r *backupRepo) CutoffLog(path, target string) (string, error) {
	if !io.Exists(target) {
		return "", errors.New(r.t.Get("log file %s not exists", target))
	}

	name := strings.TrimSuffix(filepath.Base(target), filepath.Ext(target))
	to := filepath.Join(path, fmt.Sprintf("%s_%s%s", name, time.Now().Format("20060102150405"), r.backupExt()))
	if err := io.Compress(filepath.Dir(target), []string{filepath.Base(target)}, to); err != nil {
		return "", err
	}

	// 原文件不能直接删除，直接删的话仍会占用空间直到重启相关的应用
	if _, err := shell.Execf("cat /dev/null > '%s'", target); err != nil {
		return "", err
	}

	return to, nil
}

// CutoffUpload 将指定的切割日志文件上传到远程存储
func (r *backupRepo) CutoffUpload(account uint, typ biz.BackupType, name string, files []string) error {
	backupStorage, err := r.GetStorage(account)
	if err != nil {
		return err
	}

	client, err := r.getStorage(*backupStorage)
	if err != nil {
		return err
	}

	for _, localPath := range files {
		file, err := os.Open(localPath)
		if err != nil {
			return err
		}
		remotePath := filepath.Join("cutoff", string(typ), name, filepath.Base(localPath))
		if putErr := client.Put(remotePath, file); putErr != nil {
			_ = file.Close()
			return putErr
		}
		_ = file.Close()
	}

	return nil
}

// ClearExpired 清理过期备份
// path 备份目录绝对路径
// prefix 目标文件前缀
// save 保存份数
func (r *backupRepo) ClearExpired(path, prefix string, save uint) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	filtered := lo.FilterMap(files, func(file os.DirEntry, _ int) (os.FileInfo, bool) {
		if !strings.HasPrefix(file.Name(), prefix) || !r.isBackupArchive(file.Name()) {
			return nil, false
		}
		info, err := os.Stat(filepath.Join(path, file.Name()))
		if err != nil {
			return nil, false
		}
		return info, true
	})

	// 排序所有备份文件，从新到旧
	slices.SortFunc(filtered, func(a, b os.FileInfo) int {
		if a.ModTime().After(b.ModTime()) {
			return -1
		}
		if a.ModTime().Before(b.ModTime()) {
			return 1
		}
		return 0
	})
	if uint(len(filtered)) <= save {
		return nil
	}

	// 切片保留 save 份，删除剩余
	toDelete := filtered[save:]
	for _, file := range toDelete {
		filePath := filepath.Join(path, file.Name())
		if app.IsCli {
			fmt.Println(r.t.Get("|-Cleaning expired file: %s", filePath))
		}
		if err = os.Remove(filePath); err != nil {
			return errors.New(r.t.Get("Cleanup failed: %v", err))
		}
	}

	return nil
}

// ClearStorageExpired 清理备份账号过期备份
func (r *backupRepo) ClearStorageExpired(storage uint, typ biz.BackupType, prefix string, save uint) error {
	backupStorage, err := r.GetStorage(storage)
	if err != nil {
		return err
	}

	client, err := r.getStorage(*backupStorage)
	if err != nil {
		return err
	}

	files, err := client.List(string(typ))
	if err != nil {
		return err
	}

	type fileInfo struct {
		name    string
		modTime time.Time
	}
	var filtered []fileInfo
	for _, file := range files {
		if strings.HasPrefix(file, prefix) && r.isBackupArchive(file) {
			lastModified, modErr := client.LastModified(filepath.Join(string(typ), file))
			if modErr != nil {
				continue
			}
			filtered = append(filtered, fileInfo{name: file, modTime: lastModified})
		}
	}

	// 排序所有备份文件，从新到旧
	slices.SortFunc(filtered, func(a, b fileInfo) int {
		if a.modTime.After(b.modTime) {
			return -1
		}
		if a.modTime.Before(b.modTime) {
			return 1
		}
		return 0
	})
	if uint(len(filtered)) <= save {
		return nil
	}

	// 切片保留 save 份，删除剩余
	toDelete := filtered[save:]
	for _, file := range toDelete {
		filePath := filepath.Join(string(typ), file.name)
		if app.IsCli {
			fmt.Println(r.t.Get("|-Cleaning expired file: %s", filePath))
		}
		if err = client.Delete(filePath); err != nil {
			return errors.New(r.t.Get("Cleanup failed: %v", err))
		}
	}

	return nil
}

// getStorage 获取存储器
func (r *backupRepo) getStorage(backupStorage biz.BackupStorage) (storage.Storage, error) {
	switch backupStorage.Type {
	case biz.BackupStorageTypeLocal:
		return storage.NewLocal(backupStorage.Info.Path)
	case biz.BackupStorageTypeS3:
		return storage.NewS3(storage.S3Config{
			Region:          backupStorage.Info.Region,
			Bucket:          backupStorage.Info.Bucket,
			AccessKey:       backupStorage.Info.AccessKey,
			SecretKey:       backupStorage.Info.SecretKey,
			Endpoint:        backupStorage.Info.Endpoint,
			Scheme:          backupStorage.Info.Scheme,
			BasePath:        backupStorage.Info.Path,
			AddressingStyle: storage.S3AddressingStyle(backupStorage.Info.Style),
		})
	case biz.BackupStorageTypeSFTP:
		return storage.NewSFTP(storage.SFTPConfig{
			Host:       backupStorage.Info.Host,
			Port:       backupStorage.Info.Port,
			Username:   backupStorage.Info.Username,
			Password:   backupStorage.Info.Password,
			PrivateKey: backupStorage.Info.PrivateKey,
			BasePath:   backupStorage.Info.Path,
		})
	case biz.BackupStorageTypeWebDAV:
		return storage.NewWebDav(storage.WebDavConfig{
			URL:      backupStorage.Info.URL,
			Username: backupStorage.Info.Username,
			Password: backupStorage.Info.Password,
			BasePath: backupStorage.Info.Path,
		})
	default:
		return nil, errors.New(r.t.Get("unknown storage type"))
	}
}

// createWebsite 创建网站备份
func (r *backupRepo) createWebsite(name string, storage storage.Storage, target string) error {
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
	name = name + r.backupExt()
	if err = io.Compress(website.Path, nil, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = storage.Put(filepath.Join(string(biz.BackupTypeWebsite), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// createMySQL 创建 MySQL 备份
func (r *backupRepo) createMySQL(name string, storage storage.Storage, target string) error {
	rootPassword, err := r.setting.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return err
	}
	mysql, err := db.NewMySQL("root", rootPassword, "/tmp/mysql.sock", "unix")
	if err != nil {
		return err
	}
	defer mysql.Close()
	if exist, _ := mysql.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
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

	// 导出数据库
	name = name + ".sql"
	_ = os.Setenv("MYSQL_PWD", rootPassword)
	if _, err = shell.Execf(`mysqldump -u root --single-transaction --quick '%s' > '%s'`, target, filepath.Join(tmpDir, name)); err != nil {
		return err
	}
	_ = os.Unsetenv("MYSQL_PWD")

	// 压缩备份文件
	if err = io.Compress(tmpDir, []string{name}, filepath.Join(tmpDir, name+r.backupExt())); err != nil {
		return err
	}

	// 上传备份文件到存储器
	name = name + r.backupExt()
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = storage.Put(filepath.Join(string(biz.BackupTypeMySQL), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// createPostgres 创建 PostgreSQL 备份
func (r *backupRepo) createPostgres(name string, storage storage.Storage, target string) error {
	postgresPassword, err := r.setting.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		return err
	}
	postgres, err := db.NewPostgres("postgres", postgresPassword, "127.0.0.1", 5432)
	if err != nil {
		return err
	}
	defer postgres.Close()
	if exist, _ := postgres.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
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

	// 导出数据库
	name = name + ".sql"
	_ = os.Setenv("PGPASSWORD", postgresPassword)
	if _, err = shell.Execf(`pg_dump -h 127.0.0.1 -U postgres '%s' > '%s'`, target, filepath.Join(tmpDir, name)); err != nil {
		return err
	}
	_ = os.Unsetenv("PGPASSWORD")

	// 压缩备份文件
	if err = io.Compress(tmpDir, []string{name}, filepath.Join(tmpDir, name+r.backupExt())); err != nil {
		return err
	}

	// 上传备份文件到存储器
	name = name + r.backupExt()
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = storage.Put(filepath.Join(string(biz.BackupTypePostgres), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// createClickHouse 创建 ClickHouse 备份
func (r *backupRepo) createClickHouse(name string, storage storage.Storage, target string) error {
	password, err := r.setting.Get(biz.SettingKeyClickHouseDefaultPassword)
	if err != nil {
		return err
	}
	// clickhouse-client 走 native 9000 端口，本地 default 用户
	conn := fmt.Sprintf("--host 127.0.0.1 --port 9000 --user default --password '%s'", password)

	// 校验数据库是否存在
	exist, err := shell.Execf("clickhouse-client %s --query \"SELECT count() FROM system.databases WHERE name = '%s'\"", conn, target)
	if err != nil {
		return err
	}
	if strings.TrimSpace(exist) == "0" {
		return errors.New(r.t.Get("database does not exist: %s", target))
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

	// 数据表（含数据）在前，视图（仅结构）在后
	dataTables, err := r.clickHouseTables(conn, target, false)
	if err != nil {
		return err
	}
	views, err := r.clickHouseTables(conn, target, true)
	if err != nil {
		return err
	}

	// 导出结构到 schema.sql，去掉库名限定（恢复时由 --database 指定目标库）
	objects := make([]string, 0, len(dataTables)+len(views))
	objects = append(objects, dataTables...)
	objects = append(objects, views...)
	var schema strings.Builder
	for _, tbl := range objects {
		create, err := shell.Execf("clickhouse-client %s --query \"SELECT create_table_query FROM system.tables WHERE database = '%s' AND name = '%s' FORMAT TabSeparatedRaw\"", conn, target, tbl)
		if err != nil {
			return err
		}
		stmt := strings.TrimSpace(create)
		stmt = strings.ReplaceAll(stmt, fmt.Sprintf("`%s`.", target), "")
		stmt = strings.ReplaceAll(stmt, fmt.Sprintf("%s.", target), "")
		schema.WriteString(stmt)
		schema.WriteString(";\n")
	}
	if err = io.Write(filepath.Join(tmpDir, "schema.sql"), schema.String(), 0644); err != nil {
		return err
	}

	// 导出数据（仅数据表，Native 格式）
	files := []string{"schema.sql"}
	for _, tbl := range dataTables {
		dataFile := tbl + ".native"
		if _, err = shell.Execf("clickhouse-client %s --query 'SELECT * FROM `%s`.`%s` FORMAT Native' > '%s'", conn, target, tbl, filepath.Join(tmpDir, dataFile)); err != nil {
			return err
		}
		files = append(files, dataFile)
	}

	// 压缩备份文件
	name = name + r.backupExt()
	if err = io.Compress(tmpDir, files, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = storage.Put(filepath.Join(string(biz.BackupTypeClickHouse), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// clickHouseTables 列出库中对象名，onlyView 为 true 时仅返回视图，否则返回非视图数据表
func (r *backupRepo) clickHouseTables(conn, database string, onlyView bool) ([]string, error) {
	op := "NOT LIKE"
	if onlyView {
		op = "LIKE"
	}
	out, err := shell.Execf("clickhouse-client %s --query \"SELECT name FROM system.tables WHERE database = '%s' AND NOT is_temporary AND engine %s '%%View%%' ORDER BY name FORMAT TabSeparated\"", conn, database, op)
	if err != nil {
		return nil, err
	}

	var tables []string
	for line := range strings.SplitSeq(out, "\n") {
		if l := strings.TrimSpace(line); l != "" {
			tables = append(tables, l)
		}
	}
	return tables, nil
}

// createPath 创建目录备份
func (r *backupRepo) createPath(name string, storage storage.Storage, target string) error {
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
	name = name + r.backupExt()
	if err = io.Compress(target, nil, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = storage.Put(filepath.Join(string(biz.BackupTypePath), name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// restoreWebsite 恢复网站备份
func (r *backupRepo) restoreWebsite(backup, target string) error {
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
	cmd, err := io.UnCompressShell(backup, stage)
	if err != nil {
		return err
	}
	if _, err = shell.Execf("%s", cmd); err != nil {
		return err
	}

	// 解压命令容忍单条目失败（如 .user.ini 带 chattr +i），这里用产物非空兜底，
	// 避免归档损坏或磁盘写满时清空网站却报告成功
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New(r.t.Get("uncompressed backup is empty, restore aborted"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Replacing website files..."))
	}
	if err = io.Remove(website.Path); err != nil {
		return err
	}
	if err = os.Rename(stage, website.Path); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Fixing file permissions..."))
	}
	if err = io.Chmod(website.Path, 0755); err != nil {
		return err
	}
	if err = io.Chown(website.Path, "www", "www"); err != nil {
		return err
	}

	return nil
}

// importFile 把备份文件喂给数据库客户端 stdin，并按秒级周期打印大致进度
// pipe 有背压，已写入字节数 ≈ 客户端已消费字节数，足以作为进度参考
func (r *backupRepo) importFile(path, name string, args []string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) { _ = f.Close() }(f)

	var total int64
	if stat, err := f.Stat(); err == nil {
		total = stat.Size()
	}

	var progress func(written, total int64, rate float64)
	if app.IsCli {
		fmt.Println(r.t.Get("|-File size: %s", tools.FormatBytes(float64(total))))
		progress = func(written, total int64, rate float64) {
			if total > 0 {
				fmt.Println(r.t.Get("|-Importing: %s / %s (%.1f%%, %s/s)",
					tools.FormatBytes(float64(written)), tools.FormatBytes(float64(total)),
					float64(written)*100/float64(total), tools.FormatBytes(rate)))
			} else {
				fmt.Println(r.t.Get("|-Importing: %s (%s/s)",
					tools.FormatBytes(float64(written)), tools.FormatBytes(rate)))
			}
		}
	}

	_, err = shell.ExecWithStdinProgress(context.Background(), name, args, f, total, 5*time.Second, progress)
	return err
}

// restoreMySQL 恢复 MySQL 备份
func (r *backupRepo) restoreMySQL(backup, target string) error {
	rootPassword, err := r.setting.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return err
	}
	mysql, err := db.NewMySQL("root", rootPassword, "/tmp/mysql.sock", "unix")
	if err != nil {
		return err
	}
	defer mysql.Close()
	if exist, _ := mysql.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	clean := false
	if !strings.HasSuffix(backup, ".sql") {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Uncompressing backup..."))
		}
		backup, err = r.autoUnCompressSQL(backup)
		if err != nil {
			return err
		}
		clean = true
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Importing SQL into database..."))
	}
	_ = os.Setenv("MYSQL_PWD", rootPassword)
	err = r.importFile(backup, "mysql", []string{"-u", "root", target})
	_ = os.Unsetenv("MYSQL_PWD")
	if err != nil {
		return err
	}
	if clean {
		_ = io.Remove(filepath.Dir(backup))
	}

	return nil
}

// restorePostgres 恢复 PostgreSQL 备份
func (r *backupRepo) restorePostgres(backup, target string) error {
	postgresPassword, err := r.setting.Get(biz.SettingKeyPostgresPassword)
	if err != nil {
		return err
	}
	postgres, err := db.NewPostgres("postgres", postgresPassword, "127.0.0.1", 5432)
	if err != nil {
		return err
	}
	defer postgres.Close()
	if exist, _ := postgres.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	clean := false
	if !strings.HasSuffix(backup, ".sql") {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Uncompressing backup..."))
		}
		backup, err = r.autoUnCompressSQL(backup)
		if err != nil {
			return err
		}
		clean = true
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Importing SQL into database..."))
	}
	_ = os.Setenv("PGPASSWORD", postgresPassword)
	err = r.importFile(backup, "psql", []string{"-h", "127.0.0.1", "-U", "postgres", target})
	_ = os.Unsetenv("PGPASSWORD")
	if err != nil {
		return err
	}
	if clean {
		_ = io.Remove(filepath.Dir(backup))
	}

	return nil
}

// restoreClickHouse 恢复 ClickHouse 备份
func (r *backupRepo) restoreClickHouse(backup, target string) error {
	password, err := r.setting.Get(biz.SettingKeyClickHouseDefaultPassword)
	if err != nil {
		return err
	}
	conn := fmt.Sprintf("--host 127.0.0.1 --port 9000 --user default --password '%s'", password)
	connArgs := []string{"--host", "127.0.0.1", "--port", "9000", "--user", "default", "--password", password}

	// 校验目标数据库是否存在
	exist, err := shell.Execf("clickhouse-client %s --query \"SELECT count() FROM system.databases WHERE name = '%s'\"", conn, target)
	if err != nil {
		return err
	}
	if strings.TrimSpace(exist) == "0" {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	// 纯 SQL 文件直接执行
	if strings.HasSuffix(backup, ".sql") {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Importing SQL into database..."))
		}
		return r.importFile(backup, "clickhouse-client", append(slices.Clone(connArgs), "--database", target, "--multiquery"))
	}

	// 解压到临时目录
	tmpDir, err := r.tmpDir()
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
		fmt.Println(r.t.Get("|-Uncompressing backup..."))
	}
	if err = io.UnCompress(backup, tmpDir); err != nil {
		return err
	}

	// 先恢复结构（视图已排在末尾，源表先建好，规避物化视图依赖顺序问题）
	schemaPath := filepath.Join(tmpDir, "schema.sql")
	if io.Exists(schemaPath) {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Restoring schema..."))
		}
		if err = r.importFile(schemaPath, "clickhouse-client", append(slices.Clone(connArgs), "--database", target, "--multiquery")); err != nil {
			return err
		}
	}

	// 再导入数据
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".native") {
			continue
		}
		tbl := strings.TrimSuffix(entry.Name(), ".native")
		if app.IsCli {
			fmt.Println(r.t.Get("|-Importing table: %s", tbl))
		}
		query := fmt.Sprintf("INSERT INTO `%s` FORMAT Native", tbl)
		if err = r.importFile(filepath.Join(tmpDir, entry.Name()), "clickhouse-client", append(slices.Clone(connArgs), "--database", target, "--query", query)); err != nil {
			return err
		}
	}

	return nil
}

// redisLikeConf redis/valkey 的连接与持久化参数
type redisLikeConf struct {
	kind       string // redis / valkey
	cli        string // redis-cli / valkey-cli
	authEnv    string // REDISCLI_AUTH / VALKEYCLI_AUTH
	dataDir    string // {app.Root}/server/{kind}
	confPath   string
	port       string
	password   string
	appendonly bool
}

// loadRedisLikeConf 从 {kind}.conf 读取连接与持久化参数
func (r *backupRepo) loadRedisLikeConf(kind string) (*redisLikeConf, error) {
	dataDir := fmt.Sprintf("%s/server/%s", app.Root, kind)
	confPath := fmt.Sprintf("%s/%s.conf", dataDir, kind)
	content, err := io.Read(confPath)
	if err != nil {
		return nil, err
	}

	port := redisLikeValue(content, "port")
	if port == "" {
		port = "6379"
	}
	authEnv := "REDISCLI_AUTH"
	if kind == "valkey" {
		authEnv = "VALKEYCLI_AUTH"
	}

	return &redisLikeConf{
		kind:       kind,
		cli:        kind + "-cli",
		authEnv:    authEnv,
		dataDir:    dataDir,
		confPath:   confPath,
		port:       port,
		password:   redisLikeValue(content, "requirepass"),
		appendonly: redisLikeValue(content, "appendonly") == "yes",
	}, nil
}

// redisLikeValue 从 redis/valkey 配置内容中读取指定键的有效值（忽略注释行）
func redisLikeValue(content, key string) string {
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && fields[0] == key {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

// createRedisLike 创建 Redis/Valkey 整实例备份（{kind}-cli --rdb 导出快照）
// redis 与 valkey 共用本实现，kind 为 "redis" 或 "valkey"
func (r *backupRepo) createRedisLike(name string, storage storage.Storage, kind string) error {
	conf, err := r.loadRedisLikeConf(kind)
	if err != nil {
		return err
	}
	// 用环境变量传密码，避免密码出现在命令行/进程列表
	if conf.password != "" {
		_ = os.Setenv(conf.authEnv, conf.password)
		defer func() { _ = os.Unsetenv(conf.authEnv) }()
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

	// 通过复制协议拉取整实例 RDB 快照到本地文件
	rdb := filepath.Join(tmpDir, "dump.rdb")
	if _, err = shell.Execf("%s -h 127.0.0.1 -p %s --rdb '%s'", conf.cli, conf.port, rdb); err != nil {
		return err
	}
	if !io.Exists(rdb) {
		return errors.New(r.t.Get("failed to export RDB snapshot"))
	}

	// 压缩备份文件
	name = name + r.backupExt()
	if err = io.Compress(tmpDir, []string{"dump.rdb"}, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if err = storage.Put(filepath.Join(kind, name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// restoreRedisLike 恢复 Redis/Valkey 整实例备份
// 停服务 → 替换 dump.rdb → 启动；妥善处理 AOF 优先级陷阱（appendonly=yes 时 AOF 会盖过 RDB）
func (r *backupRepo) restoreRedisLike(backup, kind string) error {
	conf, err := r.loadRedisLikeConf(kind)
	if err != nil {
		return err
	}

	// 准备 dump.rdb：裸 .rdb 直接用，否则解压取包内的 dump.rdb
	rdb := backup
	if !strings.HasSuffix(backup, ".rdb") {
		tmpDir, err := r.tmpDir()
		if err != nil {
			return err
		}
		defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)
		if app.IsCli {
			fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
			fmt.Println(r.t.Get("|-Uncompressing backup..."))
		}
		if err = io.UnCompress(backup, tmpDir); err != nil {
			return err
		}
		rdb = filepath.Join(tmpDir, "dump.rdb")
		if !io.Exists(rdb) {
			return errors.New(r.t.Get("dump.rdb not found in backup file"))
		}
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Stopping %s service...", conf.kind))
	}
	if err = systemctl.Stop(conf.kind); err != nil {
		return err
	}

	// 清理旧 AOF（多部件目录与旧式单文件），避免 AOF 优先于 RDB 被加载
	_ = io.Remove(filepath.Join(conf.dataDir, "appendonlydir"))
	_ = io.Remove(filepath.Join(conf.dataDir, "appendonly.aof"))

	if app.IsCli {
		fmt.Println(r.t.Get("|-Replacing dump.rdb..."))
	}
	// 覆盖 dump.rdb
	target := filepath.Join(conf.dataDir, "dump.rdb")
	if err = io.Cp(rdb, target); err != nil {
		_ = systemctl.Start(conf.kind) // 尽力恢复服务
		return err
	}
	_ = io.Chown(target, kind, kind)
	_ = io.Chmod(target, 0640)

	// 若原本开启 AOF，必须先以 appendonly no 启动加载 RDB，否则会建空 AOF 以空库覆盖
	if conf.appendonly {
		if err = r.disableAppendonly(conf.confPath); err != nil {
			_ = systemctl.Start(conf.kind)
			return err
		}
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Starting %s service (loading RDB)...", conf.kind))
	}
	// 启动服务（Type=notify，返回即已加载 RDB）
	if err = systemctl.Start(conf.kind); err != nil {
		return err
	}

	// 原本开启 AOF 的，在线转回并持久化配置
	if conf.appendonly {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Re-enabling AOF..."))
		}
		if conf.password != "" {
			_ = os.Setenv(conf.authEnv, conf.password)
			defer func() { _ = os.Unsetenv(conf.authEnv) }()
		}
		_, _ = shell.Execf("%s -h 127.0.0.1 -p %s config set appendonly yes", conf.cli, conf.port)
		_, _ = shell.Execf("%s -h 127.0.0.1 -p %s config rewrite", conf.cli, conf.port)
	}

	return nil
}

// disableAppendonly 将 redis/valkey 配置中的 appendonly 临时改为 no
func (r *backupRepo) disableAppendonly(confPath string) error {
	content, err := io.Read(confPath)
	if err != nil {
		return err
	}

	lines := strings.Split(content, "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if fields := strings.Fields(trimmed); len(fields) >= 1 && fields[0] == "appendonly" {
			lines[i] = "appendonly no"
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "appendonly no")
	}

	return io.Write(confPath, strings.Join(lines, "\n"), 0644)
}

// autoUnCompressSQL 自动处理压缩文件
func (r *backupRepo) autoUnCompressSQL(backup string) (string, error) {
	temp, err := r.tmpDir()
	if err != nil {
		return "", err
	}

	if err = io.UnCompress(backup, temp); err != nil {
		return "", err
	}

	backup = "" // 置空，防止干扰后续判断
	if files, err := os.ReadDir(temp); err == nil {
		if len(files) != 1 {
			return "", errors.New(r.t.Get("The number of files contained in the compressed file is not 1, actual %d", len(files)))
		}
		if strings.HasSuffix(files[0].Name(), ".sql") {
			backup = filepath.Join(temp, files[0].Name())
		}
	}

	if backup == "" {
		return "", errors.New(r.t.Get("could not find .sql backup file"))
	}

	return backup, nil
}

func (r *backupRepo) FixPanel() error {
	if app.IsCli {
		fmt.Println(r.t.Get("|-Start fixing the panel..."))
	}

	// 检查关键文件是否正常
	panelBroken := !io.Exists(filepath.Join(app.Root, "panel", "ace")) ||
		!io.Exists(filepath.Join(app.Root, "panel", "storage", "config.yml")) ||
		!io.Exists(filepath.Join(app.Root, "panel", "storage", "panel.db"))
	// 检查主数据库完整性
	if !quickCheck(r.db) {
		panelBroken = true
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		panelBroken = true
	}

	// 检查辅助数据库是否异常
	var brokenAuxDBs []string
	for _, name := range []string{"stat", "scan", "tamper"} {
		auxDB, err := openDB(name)
		if err != nil {
			brokenAuxDBs = append(brokenAuxDBs, name)
			continue
		}
		ok := quickCheck(auxDB)
		if sqlDB, dbErr := auxDB.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		if !ok {
			brokenAuxDBs = append(brokenAuxDBs, name)
		}
	}

	// 一切正常，无需修复
	if !panelBroken && len(brokenAuxDBs) == 0 {
		return errors.New(r.t.Get("Files are normal and do not need to be repaired, please run acepanel update to update the panel"))
	}

	// 有异常，先停止面板
	tools.StopPanel()

	// 删除损坏的辅助数据库（会自动重建）
	for _, name := range brokenAuxDBs {
		dbPath := filepath.Join(app.Root, "panel", "storage", name+".db")
		if removeErr := io.Remove(dbPath); removeErr != nil {
			return errors.New(r.t.Get("Failed to remove %s.db: %v", name, removeErr))
		}
		if app.IsCli {
			fmt.Println(r.t.Get("|-Found %s.db is abnormal, removed it", name))
		}
	}

	// 仅辅助数据库异常，重启即可恢复
	if !panelBroken {
		if app.IsCli {
			fmt.Println(r.t.Get("|-Fix completed"))
		}
		tools.RestartPanel()
		return nil
	}

	// 从备份目录中找最新的备份文件
	files, err := os.ReadDir(r.GetDefaultPath(biz.BackupTypePanel))
	if err != nil {
		return err
	}
	var list []os.FileInfo
	for _, file := range files {
		if !r.isBackupArchive(file.Name()) {
			continue
		}
		info, infoErr := file.Info()
		if infoErr != nil {
			continue
		}
		list = append(list, info)
	}
	slices.SortFunc(list, func(a os.FileInfo, b os.FileInfo) int {
		return int(b.ModTime().Unix() - a.ModTime().Unix())
	})
	if len(list) == 0 {
		return errors.New(r.t.Get("No backup file found, unable to automatically repair"))
	}
	latest := list[0]
	latestPath := filepath.Join(r.GetDefaultPath(biz.BackupTypePanel), latest.Name())
	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file used: %s", latest.Name()))
	}

	// 解压备份文件
	if app.IsCli {
		fmt.Println(r.t.Get("|-Unzip backup file..."))
	}
	if err = io.Remove("/tmp/panel-fix"); err != nil {
		return errors.New(r.t.Get("Cleaning temporary directory failed: %v", err))
	}
	if err = io.UnCompress(latestPath, "/tmp/panel-fix"); err != nil {
		return errors.New(r.t.Get("Unzip backup file failed: %v", err))
	}

	// 移动文件到对应位置
	if app.IsCli {
		fmt.Println(r.t.Get("|-Move backup file..."))
	}
	if io.Exists(filepath.Join("/tmp/panel-fix", "panel")) && io.IsDir(filepath.Join("/tmp/panel-fix", "panel")) {
		// 整体替换 panel 目录前先保住自定义编译参数
		customize := filepath.Join(app.Root, "panel", "storage", "customize")
		keep := filepath.Join(app.Root, ".customize-keep")
		_ = io.Remove(keep)
		if io.Exists(customize) {
			_ = io.Mv(customize, keep)
		}
		if err = io.Remove(filepath.Join(app.Root, "panel")); err != nil {
			return errors.New(r.t.Get("Remove panel file failed: %v", err))
		}
		if err = io.Mv(filepath.Join("/tmp/panel-fix", "panel"), filepath.Join(app.Root)); err != nil {
			return errors.New(r.t.Get("Move panel file failed: %v", err))
		}
		if io.Exists(keep) {
			_ = io.Remove(customize)
			_ = io.Mv(keep, customize)
		}
	}
	if io.Exists(filepath.Join("/tmp/panel-fix", "acepanel")) {
		if err = io.Mv(filepath.Join("/tmp/panel-fix", "acepanel"), "/usr/local/sbin/acepanel"); err != nil {
			return errors.New(r.t.Get("Move acepanel file failed: %v", err))
		}
	}

	// 下载服务文件
	if !io.Exists("/etc/systemd/system/acepanel.service") {
		if _, err = shell.Execf(`wget -O /etc/systemd/system/acepanel.service https://%s/acepanel.service && sed -i "s|/opt/ace|%s|g" /etc/systemd/system/acepanel.service`, r.conf.App.DownloadEndpoint, app.Root); err != nil {
			return err
		}
	}

	// 处理权限
	if app.IsCli {
		fmt.Println(r.t.Get("|-Set key file permissions..."))
	}
	if err = io.Chmod(filepath.Join(app.Root, "panel", "storage", "config.yml"), 0600); err != nil {
		return err
	}
	if err = io.Chmod(filepath.Join(app.Root, "panel", "storage", "panel.db"), 0600); err != nil {
		return err
	}
	if err = io.Chmod("/etc/systemd/system/acepanel.service", 0644); err != nil {
		return err
	}
	if err = io.Chmod("/usr/local/sbin/acepanel", 0700); err != nil {
		return err
	}
	if err = io.Chmod(filepath.Join(app.Root, "panel"), 0700); err != nil {
		return err
	}

	if err = io.Remove("/tmp/panel-fix"); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Fix completed"))
	}

	tools.RestartPanel()
	return nil
}

// UpdatePanel 升级面板
func (r *backupRepo) UpdatePanel(version, url, checksum string, progress func(string)) error {
	if progress == nil {
		progress = func(string) {}
	}

	// 进程级升级锁
	if !r.updating.CompareAndSwap(false, true) {
		return errors.New(r.t.Get("panel is already updating, please try again later"))
	}
	defer r.updating.Store(false)

	panelDir := filepath.Join(app.Root, "panel")
	workDir := filepath.Join(panelDir, ".update-work") // staging 目录固定在 panel 内，绝不用 /tmp（可能跨分区）
	newDir := filepath.Join(workDir, "new")
	name := filepath.Base(url)

	// 失败回滚
	rollback := func(err error) error {
		_ = io.Remove(workDir)
		app.Status = app.StatusNormal
		return err
	}

	app.Status = app.StatusUpgrade

	progress(r.t.Get("Preparing to update to %s...", version))
	if err := io.Remove(workDir); err != nil {
		return rollback(errors.New(r.t.Get("Failed to clean up temporary directory: %v", err)))
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		return rollback(errors.New(r.t.Get("Failed to optimize database: %v", err)))
	}

	// 备份留档
	progress(r.t.Get("Backing up panel data..."))
	if err := r.CreatePanel(); err != nil {
		r.log.Warn("failed to backup panel before update", slog.Any("err", err))
	}

	// 下载新版本
	if err := os.MkdirAll(workDir, 0700); err != nil {
		return rollback(errors.New(r.t.Get("Failed to create temporary directory: %v", err)))
	}
	progress(r.t.Get("Downloading new version..."))
	if _, err := shell.Execf("aria2c -c --file-allocation=falloc --allow-overwrite=true --auto-file-renaming=false --retry-wait=5 --max-tries=5 -x 16 -s 16 -k 1M -d %s -o %s %s", workDir, name, url); err != nil {
		return rollback(errors.New(r.t.Get("Download failed: %v", err)))
	}
	if _, err := shell.Execf("aria2c -c --file-allocation=falloc --allow-overwrite=true --auto-file-renaming=false --retry-wait=5 --max-tries=5 -x 1 -s 1 -k 1M -d %s -o %s %s", workDir, name+".sha256", checksum); err != nil {
		return rollback(errors.New(r.t.Get("Download failed: %v", err)))
	}
	if !io.Exists(filepath.Join(workDir, name)) || !io.Exists(filepath.Join(workDir, name+".sha256")) {
		return rollback(errors.New(r.t.Get("Download file check failed")))
	}

	// 校验 sha256
	progress(r.t.Get("Verifying download file..."))
	if check, err := shell.ExecfWithDir(workDir, "sha256sum -c %s --ignore-missing", name+".sha256"); check != name+": OK" || err != nil {
		return rollback(errors.New(r.t.Get("Verify download file failed: %v", err)))
	}

	// 解压
	progress(r.t.Get("Extracting new version..."))
	if err := io.UnCompress(filepath.Join(workDir, name), newDir); err != nil {
		return rollback(errors.New(r.t.Get("Unzip new version failed: %v", err)))
	}
	if !io.Exists(filepath.Join(newDir, "ace")) {
		return rollback(errors.New(r.t.Get("Unzip new version failed, missing file")))
	}

	// 应用
	progress(r.t.Get("Applying update..."))
	if err := r.applyUpdate(newDir); err != nil {
		return rollback(errors.New(r.t.Get("Applying update failed: %v", err)))
	}

	// 收尾
	progress(r.t.Get("Finishing up..."))
	if err := r.finishUpdate(version); err != nil {
		return rollback(errors.New(r.t.Get("Finishing update failed: %v", err)))
	}

	_ = io.Remove(workDir)
	r.log.Info("panel updated", slog.String("version", version))
	progress(r.t.Get("Update completed"))

	// 由调用方重启面板
	if sqlDB, err := r.db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	return nil
}

// applyUpdate 用 newDir 下的新版本文件替换 panel/ 中的程序文件
func (r *backupRepo) applyUpdate(newDir string) error {
	entries, err := os.ReadDir(newDir)
	if err != nil {
		return err
	}
	panelDir := filepath.Join(app.Root, "panel")
	for _, e := range entries {
		name := e.Name()
		if name == "storage" {
			continue
		}
		src := filepath.Join(newDir, name)
		if name == "cli" {
			// 先 cp 保证原子替换
			tmp := "/usr/local/sbin/.acepanel.new"
			if err = io.Cp(src, tmp); err != nil {
				return err
			}
			if err = io.Mv(tmp, "/usr/local/sbin/acepanel"); err != nil {
				return err
			}
			continue
		}
		// 其余程序文件
		if err = io.Mv(src, filepath.Join(panelDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// finishUpdate 升级收尾
func (r *backupRepo) finishUpdate(version string) error {
	panelDir := filepath.Join(app.Root, "panel")
	serviceFile := "/etc/systemd/system/acepanel.service"

	// 更新 service 文件
	tmpService := serviceFile + ".new"
	if _, err := shell.Execf(`wget -O %s https://%s/acepanel.service`, tmpService, r.conf.App.DownloadEndpoint); err == nil {
		_, _ = shell.Execf(`sed -i "s|/opt/ace|%s|g" %s`, app.Root, tmpService)
		if out, _ := shell.Execf("grep -c ExecStart %s", tmpService); strings.TrimSpace(out) != "0" {
			_ = io.Mv(tmpService, serviceFile) // 同在 /etc/systemd/system → 同分区 rename
		}
	}
	_ = io.Remove(tmpService)
	if !io.Exists(serviceFile) {
		return errors.New(r.t.Get("panel service file is missing"))
	}
	// 校验 unit 指向的主程序确实存在，避免 daemon-reload 后起不来
	if !io.Exists(filepath.Join(panelDir, "ace")) {
		return errors.New(r.t.Get("panel binary is missing after update"))
	}
	_, _ = shell.Execf("systemctl daemon-reload")

	// 执行后置脚本
	_, _ = shell.Execf("curl -sSLm 10 --fail --retry 3 https://%s/auto_update.sh | bash", r.conf.App.DownloadEndpoint)

	// 后置脚本之后再写版本号
	if err := r.setting.Set(biz.SettingKeyVersion, version); err != nil {
		return err
	}

	// 设置权限
	_ = io.Chmod(filepath.Join(panelDir, "ace"), 0700)
	_ = io.Chmod("/usr/local/sbin/acepanel", 0700)
	_ = io.Chmod(serviceFile, 0644)
	_ = io.Remove(filepath.Join(panelDir, "config.example.yml"))

	// 修正可能从 staging 继承的错误 SELinux 上下文
	_, _ = shell.Execf("restorecon %s /usr/local/sbin/acepanel %s", filepath.Join(panelDir, "ace"), serviceFile)

	return nil
}

// backupExt 根据全局设置返回备份文件扩展名
func (r *backupRepo) backupExt() string {
	format, _ := r.setting.Get(biz.SettingKeyBackupFormat, "tar.xz")
	return "." + format
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
