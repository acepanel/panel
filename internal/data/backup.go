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
	"time"

	"github.com/acepanel/panel/pkg/storage"
	"github.com/acepanel/panel/pkg/types"
	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/db"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/tools"
)

type backupRepo struct {
	hr      string
	t       *gotext.Locale
	conf    *config.Config
	db      *gorm.DB
	log     *slog.Logger
	setting biz.SettingRepo
	website biz.WebsiteRepo
}

func NewBackupRepo(t *gotext.Locale, conf *config.Config, db *gorm.DB, log *slog.Logger, setting biz.SettingRepo, website biz.WebsiteRepo) biz.BackupRepo {
	return &backupRepo{
		hr:      "+----------------------------------------------------",
		t:       t,
		conf:    conf,
		db:      db,
		log:     log,
		setting: setting,
		website: website,
	}
}

// List 备份列表
func (r *backupRepo) List(typ biz.BackupType) ([]*types.BackupFile, error) {
	path := r.GetDefaultPath(typ)
	files, err := os.ReadDir(path)
	if err != nil {
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
// account 备份账号ID
func (r *backupRepo) Create(ctx context.Context, typ biz.BackupType, target string, account uint) error {
	backupAccount := new(biz.BackupAccount)
	if err := r.db.First(backupAccount, account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(r.t.Get("backup account not found"))
		}
		return err
	}

	client, err := r.getStorage(*backupAccount)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%s_%s", target, time.Now().Format("20060102150405"))
	if app.IsCli {
		fmt.Println(r.hr)
		fmt.Println(r.t.Get("★ Start backup [%s]", time.Now().Format(time.DateTime)))
		fmt.Println(r.hr)
		fmt.Println(r.t.Get("|-Backup type: %s", string(typ)))
		fmt.Println(r.t.Get("|-Backup account: %s", backupAccount.Name))
		fmt.Println(r.t.Get("|-Backup target: %s", target))
	}

	switch typ {
	case biz.BackupTypeWebsite:
		err = r.createWebsite(name, client, target)
	case biz.BackupTypeMySQL:
		err = r.createMySQL(name, client, target)
	case biz.BackupTypePostgres:
		err = r.createPostgres(name, client, target)
	default:
		return errors.New(r.t.Get("unknown backup type"))
	}

	if app.IsCli {
		fmt.Println(r.hr)
	}
	if err != nil {
		r.log.Warn("backup failed",
			slog.String("type", biz.OperationTypeBackup),
			slog.Uint64("operator_id", getOperatorID(ctx)),
			slog.String("backup_type", string(typ)),
			slog.String("target", target),
		)
		if app.IsCli {
			fmt.Println(r.t.Get("☆ Backup failed: %v [%s]", err, time.Now().Format(time.DateTime)))
		}
	} else {
		r.log.Info("backup created",
			slog.String("type", biz.OperationTypeBackup),
			slog.Uint64("operator_id", getOperatorID(ctx)),
			slog.String("backup_type", string(typ)),
			slog.String("target", target),
		)
		if app.IsCli {
			fmt.Println(r.t.Get("☆ Backup completed [%s]\n", time.Now().Format(time.DateTime)))
		}
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

	backup := filepath.Join(r.GetDefaultPath(biz.BackupTypePanel), "panel", fmt.Sprintf("panel_%s.zip", time.Now().Format("20060102150405")))

	temp, err := os.MkdirTemp("", "acepanel-backup-*")
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(temp)

	if err = io.Cp(filepath.Join(app.Root, "panel"), temp); err != nil {
		return err
	}
	if err = io.Cp("/usr/local/sbin/acepanel", temp); err != nil {
		return err
	}

	_ = io.Chmod(temp, 0600)
	if err = io.Compress(temp, nil, backup); err != nil {
		return err
	}
	if err = io.Chmod(backup, 0600); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.t.Get("|-Backed up to file: %s", filepath.Base(backup)))
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

	// 记录日志
	r.log.Info("backup deleted", slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("backup_type", string(typ)), slog.String("name", name))

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

	var err error
	switch typ {
	case biz.BackupTypeWebsite:
		err = r.restoreWebsite(backup, target)
	case biz.BackupTypeMySQL:
		err = r.restoreMySQL(backup, target)
	case biz.BackupTypePostgres:
		err = r.restorePostgres(backup, target)
	default:
		return errors.New(r.t.Get("unknown backup type"))
	}

	if err != nil {
		return err
	}

	// 记录日志
	r.log.Info("backup restored",
		slog.String("type", biz.OperationTypeBackup),
		slog.Uint64("operator_id", getOperatorID(ctx)),
		slog.String("backup_type", string(typ)),
		slog.String("target", target),
	)

	return nil
}

// GetDefaultPath 获取默认备份路径
func (r *backupRepo) GetDefaultPath(typ biz.BackupType) string {
	backupPath, err := r.setting.Get(biz.SettingKeyBackupPath)
	if err != nil {
		return filepath.Join(app.Root, "backup", string(typ))
	}
	return filepath.Join(backupPath, string(typ))
}

// CutoffLog 切割日志
// path 保存目录绝对路径
// target 待切割日志文件绝对路径
func (r *backupRepo) CutoffLog(path, target string) error {
	if !io.Exists(target) {
		return errors.New(r.t.Get("log file %s not exists", target))
	}

	to := filepath.Join(path, fmt.Sprintf("%s_%s.zip", time.Now().Format("20060102150405"), filepath.Base(target)))
	if err := io.Compress(filepath.Dir(target), []string{filepath.Base(target)}, to); err != nil {
		return err
	}

	// 原文件不能直接删除，直接删的话仍会占用空间直到重启相关的应用
	if _, err := shell.Execf("cat /dev/null > '%s'", target); err != nil {
		return err
	}

	return nil
}

// ClearExpired 清理过期备份
// path 备份目录绝对路径
// prefix 目标文件前缀
// save 保存份数
func (r *backupRepo) ClearExpired(path, prefix string, save int) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	var filtered []os.FileInfo
	for _, file := range files {
		if strings.HasPrefix(file.Name(), prefix) && strings.HasSuffix(file.Name(), ".zip") {
			info, err := os.Stat(filepath.Join(path, file.Name()))
			if err != nil {
				continue
			}
			filtered = append(filtered, info)
		}
	}

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
	if len(filtered) <= save {
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

// ClearAccountExpired 清理备份账号过期备份
func (r *backupRepo) ClearAccountExpired(account uint, typ biz.BackupType, prefix string, save int) error {
	backupAccount := new(biz.BackupAccount)
	if err := r.db.First(backupAccount, account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New(r.t.Get("backup account not found"))
		}
		return err
	}

	client, err := r.getStorage(*backupAccount)
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
		if strings.HasPrefix(file, prefix) && strings.HasSuffix(file, ".zip") {
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
	if len(filtered) <= save {
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
func (r *backupRepo) getStorage(account biz.BackupAccount) (storage.Storage, error) {
	switch account.Type {
	case biz.BackupAccountTypeLocal:
		return storage.NewLocal(account.Info.Path)
	case biz.BackupAccountTypeS3:
		return storage.NewS3(storage.S3Config{
			Region:          account.Info.Region,
			Bucket:          account.Info.Bucket,
			AccessKeyID:     account.Info.AccessKey,
			SecretAccessKey: account.Info.SecretKey,
			Endpoint:        account.Info.Endpoint,
			BasePath:        account.Info.Path,
			AddressingStyle: storage.S3AddressingStyle(account.Info.Style),
		})
	case biz.BackupAccountTypeSFTP:
		return storage.NewSFTP(storage.SFTPConfig{
			Host:       account.Info.Host,
			Port:       account.Info.Port,
			Username:   account.Info.Username,
			Password:   account.Info.Password,
			PrivateKey: account.Info.PrivateKey,
			BasePath:   account.Info.Path,
		})
	case biz.BackupAccountTypeWebDav:
		return storage.NewWebDav(storage.WebDavConfig{
			URL:      account.Info.URL,
			Username: account.Info.Username,
			Password: account.Info.Password,
			BasePath: account.Info.Path,
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
	tmpDir, err := os.MkdirTemp("", "acepanel-backup-*")
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
		fmt.Println(r.t.Get("|-Exporting website..."))
	}

	// 压缩网站
	name = name + ".zip"
	if err = io.Compress(website.Path, nil, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 上传备份文件到存储器
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Moving backup..."))
	}

	if err = storage.Put(filepath.Join("website", name), file); err != nil {
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
	tmpDir, err := os.MkdirTemp("", "acepanel-backup-*")
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
		fmt.Println(r.t.Get("|-Exporting database..."))
	}

	// 导出数据库
	name = name + ".sql"
	_ = os.Setenv("MYSQL_PWD", rootPassword)
	if _, err = shell.Execf(`mysqldump -u root '%s' > '%s'`, target, filepath.Join(tmpDir, name)); err != nil {
		return err
	}
	_ = os.Unsetenv("MYSQL_PWD")

	// 压缩备份文件
	if err = io.Compress(tmpDir, []string{name}, filepath.Join(tmpDir, name+".zip")); err != nil {
		return err
	}

	// 上传备份文件到存储器
	name = name + ".zip"
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Moving backup..."))
	}

	if err = storage.Put(filepath.Join("mysql", name), file); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file: %s", name))
	}

	return nil
}

// createPostgres 创建 PostgreSQL 备份
func (r *backupRepo) createPostgres(name string, storage storage.Storage, target string) error {
	postgres, err := db.NewPostgres("postgres", "", "127.0.0.1", 5432)
	if err != nil {
		return err
	}
	defer postgres.Close()
	if exist, _ := postgres.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	// 创建用于压缩的临时目录
	tmpDir, err := os.MkdirTemp("", "acepanel-backup-*")
	if err != nil {
		return err
	}
	defer func(path string) { _ = os.RemoveAll(path) }(tmpDir)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Temporary directory: %s", tmpDir))
		fmt.Println(r.t.Get("|-Exporting database..."))
	}

	// 导出数据库
	name = name + ".sql"
	if _, err = shell.Execf(`su - postgres -c "pg_dump '%s'" > '%s'`, target, filepath.Join(tmpDir, name)); err != nil {
		return err
	}

	// 压缩备份文件
	if err = io.Compress(tmpDir, []string{name}, filepath.Join(tmpDir, name+".zip")); err != nil {
		return err
	}

	// 上传备份文件到存储器
	name = name + ".zip"
	file, err := os.Open(filepath.Join(tmpDir, name))
	if err != nil {
		return err
	}
	defer func(file *os.File) { _ = file.Close() }(file)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Moving backup..."))
	}

	if err = storage.Put(filepath.Join("postgres", name), file); err != nil {
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

	if err = io.Remove(website.Path); err != nil {
		return err
	}
	if err = io.UnCompress(backup, website.Path); err != nil {
		return err
	}
	if err = io.Chmod(website.Path, 0755); err != nil {
		return err
	}
	if err = io.Chown(website.Path, "www", "www"); err != nil {
		return err
	}

	return nil
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
	if err = os.Setenv("MYSQL_PWD", rootPassword); err != nil {
		return err
	}

	clean := false
	if !strings.HasSuffix(backup, ".sql") {
		backup, err = r.autoUnCompressSQL(backup)
		if err != nil {
			return err
		}
		clean = true
	}

	if _, err = shell.Execf(`mysql -u root '%s' < '%s'`, target, backup); err != nil {
		return err
	}
	if err = os.Unsetenv("MYSQL_PWD"); err != nil {
		return err
	}
	if clean {
		_ = io.Remove(filepath.Dir(backup))
	}

	return nil
}

// restorePostgres 恢复 PostgreSQL 备份
func (r *backupRepo) restorePostgres(backup, target string) error {
	postgres, err := db.NewPostgres("postgres", "", "127.0.0.1", 5432)
	if err != nil {
		return err
	}
	defer postgres.Close()
	if exist, _ := postgres.DatabaseExists(target); !exist {
		return errors.New(r.t.Get("database does not exist: %s", target))
	}

	clean := false
	if !strings.HasSuffix(backup, ".sql") {
		backup, err = r.autoUnCompressSQL(backup)
		if err != nil {
			return err
		}
		clean = true
	}

	if _, err = shell.Execf(`su - postgres -c "psql '%s'" < '%s'`, target, backup); err != nil {
		return err
	}
	if clean {
		_ = io.Remove(filepath.Dir(backup))
	}

	return nil
}

// autoUnCompressSQL 自动处理压缩文件
func (r *backupRepo) autoUnCompressSQL(backup string) (string, error) {
	temp, err := os.MkdirTemp("", "acepanel-sql-*")
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
	flag := !io.Exists(filepath.Join(app.Root, "panel", "ace")) ||
		!io.Exists(filepath.Join(app.Root, "panel", "storage", "config.yml")) ||
		!io.Exists(filepath.Join(app.Root, "panel", "storage", "panel.db")) ||
		io.Exists("/tmp/panel-storage.zip")
	// 检查数据库连接
	if err := r.db.Exec("VACUUM").Error; err != nil {
		flag = true
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		flag = true
	}
	if !flag {
		return errors.New(r.t.Get("Files are normal and do not need to be repaired, please run acepanel update to update the panel"))
	}

	// 再次确认是否需要修复
	if io.Exists("/tmp/panel-storage.zip") {
		// 文件齐全情况下只移除临时文件
		if io.Exists(filepath.Join(app.Root, "panel", "ace")) &&
			io.Exists(filepath.Join(app.Root, "panel", "storage", "config.yml")) &&
			io.Exists(filepath.Join(app.Root, "panel", "storage", "panel.db")) {
			if err := io.Remove("/tmp/panel-storage.zip"); err != nil {
				return errors.New(r.t.Get("failed to clean temporary files: %v", err))
			}
			if app.IsCli {
				fmt.Println(r.t.Get("|-Cleaned up temporary files, please run acepanel update to update the panel"))
			}
			return nil
		}
	}

	// 从备份目录中找最新的备份文件
	files, err := os.ReadDir(r.GetDefaultPath(biz.BackupTypePanel))
	if err != nil {
		return err
	}
	var list []os.FileInfo
	for _, file := range files {
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
		if err = io.Remove(filepath.Join(app.Root, "panel")); err != nil {
			return errors.New(r.t.Get("Remove panel file failed: %v", err))
		}
		if err = io.Mv(filepath.Join("/tmp/panel-fix", "panel"), filepath.Join(app.Root)); err != nil {
			return errors.New(r.t.Get("Move panel file failed: %v", err))
		}
	}
	if io.Exists(filepath.Join("/tmp/panel-fix", "acepanel")) {
		if err = io.Mv(filepath.Join("/tmp/panel-fix", "acepanel"), "/usr/local/sbin/acepanel"); err != nil {
			return errors.New(r.t.Get("Move acepanel file failed: %v", err))
		}
	}

	// tmp 目录下如果有 storage 备份，则解压回去
	if app.IsCli {
		fmt.Println(r.t.Get("|-Restore panel data..."))
	}
	if io.Exists("/tmp/panel-storage.zip") {
		if err = io.UnCompress("/tmp/panel-storage.zip", filepath.Join(app.Root, "panel")); err != nil {
			return errors.New(r.t.Get("Unzip panel data failed: %v", err))
		}
		if err = io.Remove("/tmp/panel-storage.zip"); err != nil {
			return errors.New(r.t.Get("Cleaning temporary file failed: %v", err))
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

func (r *backupRepo) UpdatePanel(version, url, checksum string) error {
	// 预先优化数据库
	if err := r.db.Exec("VACUUM").Error; err != nil {
		return err
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		return err
	}

	name := filepath.Base(url)
	if app.IsCli {
		fmt.Println(r.t.Get("|-Target version: %s", version))
		fmt.Println(r.t.Get("|-Download link: %s", url))
		fmt.Println(r.t.Get("|-File name: %s", name))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Downloading..."))
	}
	if _, err := shell.Execf("wget -T 120 -t 3 -O /tmp/%s %s", name, url); err != nil {
		return errors.New(r.t.Get("Download failed: %v", err))
	}
	if _, err := shell.Execf("wget -T 20 -t 3 -O /tmp/%s %s", name+".sha256", checksum); err != nil {
		return errors.New(r.t.Get("Download failed: %v", err))
	}
	if !io.Exists(filepath.Join("/tmp", name)) || !io.Exists(filepath.Join("/tmp", name+".sha256")) {
		return errors.New(r.t.Get("Download file check failed"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Verify download file..."))
	}
	if check, err := shell.Execf("cd /tmp && sha256sum -c %s --ignore-missing", name+".sha256"); check != name+": OK" || err != nil {
		return errors.New(r.t.Get("Verify download file failed: %v", err))
	}
	if err := io.Remove(filepath.Join("/tmp", name+".sha256")); err != nil {
		return errors.New(r.t.Get("|-Clean up verification file failed: %v", err))
	}

	if io.Exists("/tmp/panel-storage.zip") {
		return errors.New(r.t.Get("Temporary file detected in /tmp, this may be caused by the last update failure, please run acepanel fix to repair and try again"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup panel data..."))
	}
	// 备份面板
	if err := r.CreatePanel(); err != nil {
		return errors.New(r.t.Get("|-Backup panel data failed: %v", err))
	}
	if err := io.Compress(filepath.Join(app.Root, "panel/storage"), nil, "/tmp/panel-storage.zip"); err != nil {
		return errors.New(r.t.Get("|-Backup panel data failed: %v", err))
	}
	if !io.Exists("/tmp/panel-storage.zip") {
		return errors.New(r.t.Get("|-Backup panel data failed, missing file"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Cleaning old version..."))
	}
	if _, err := shell.Execf("rm -rf %s/panel/*", app.Root); err != nil {
		return errors.New(r.t.Get("|-Cleaning old version failed: %v", err))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Unzip new version..."))
	}
	if err := io.UnCompress(filepath.Join("/tmp", name), filepath.Join(app.Root, "panel")); err != nil {
		return errors.New(r.t.Get("|-Unzip new version failed: %v", err))
	}
	if !io.Exists(filepath.Join(app.Root, "panel", "ace")) {
		return errors.New(r.t.Get("|-Unzip new version failed, missing file"))
	}
	if err := io.Remove(filepath.Join("/tmp", name)); err != nil {
		return errors.New(r.t.Get("|-Clean up temporary file failed: %v", err))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Restore panel data..."))
	}
	if err := io.UnCompress("/tmp/panel-storage.zip", filepath.Join(app.Root, "panel", "storage")); err != nil {
		return errors.New(r.t.Get("|-Restore panel data failed: %v", err))
	}
	if !io.Exists(filepath.Join(app.Root, "panel/storage/panel.db")) {
		return errors.New(r.t.Get("|-Restore panel data failed, missing file"))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Run post-update script..."))
	}
	if _, err := shell.Execf("curl -sSLm 10 https://%s/auto_update.sh | bash", r.conf.App.DownloadEndpoint); err != nil {
		return errors.New(r.t.Get("|-Run post-update script failed: %v", err))
	}
	if _, err := shell.Execf(
		`wget -O /etc/systemd/system/acepanel.service https://%s/acepanel.service && sed -i "s|/www|%s|g" /etc/systemd/system/acepanel.service`,
		r.conf.App.DownloadEndpoint, app.Root,
	); err != nil {
		return errors.New(r.t.Get("|-Download panel service file failed: %v", err))
	}
	if _, err := shell.Execf("acepanel setting write version %s", version); err != nil {
		return errors.New(r.t.Get("|-Write new panel version failed: %v", err))
	}
	if err := io.Mv(filepath.Join(app.Root, "panel/cli"), "/usr/local/sbin/acepanel"); err != nil {
		return errors.New(r.t.Get("|-Move acepanel tool failed: %v", err))
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Set key file permissions..."))
	}
	_ = io.Chmod("/usr/local/sbin/acepanel", 0700)
	_ = io.Chmod("/etc/systemd/system/acepanel.service", 0644)
	_ = io.Chmod(filepath.Join(app.Root, "panel"), 0700)

	if app.IsCli {
		fmt.Println(r.t.Get("|-Update completed"))
	}

	_, _ = shell.Execf("systemctl daemon-reload")
	_ = io.Remove("/tmp/panel-storage.zip")
	_ = io.Remove(filepath.Join(app.Root, "panel/config.example.yml"))
	if sqlDB, err := r.db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	tools.RestartPanel()

	return nil
}
