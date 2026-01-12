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

	"github.com/leonelquinteros/gotext"
	"github.com/shirou/gopsutil/disk"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/db"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/tools"
	"github.com/acepanel/panel/pkg/types"
)

type backupRepo struct {
	t       *gotext.Locale
	conf    *config.Config
	db      *gorm.DB
	log     *slog.Logger
	setting biz.SettingRepo
	website biz.WebsiteRepo
}

func NewBackupRepo(t *gotext.Locale, conf *config.Config, db *gorm.DB, log *slog.Logger, setting biz.SettingRepo, website biz.WebsiteRepo) biz.BackupRepo {
	return &backupRepo{
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
	path, err := r.GetPath(typ)
	if err != nil {
		return nil, err
	}

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
// path 可选备份保存路径
func (r *backupRepo) Create(ctx context.Context, typ biz.BackupType, target string, path ...string) error {
	defPath, err := r.GetPath(typ)
	if err != nil {
		return err
	}
	if len(path) > 0 && path[0] != "" {
		defPath = path[0]
	}

	var createErr error
	switch typ {
	case biz.BackupTypeWebsite:
		createErr = r.createWebsite(defPath, target)
	case biz.BackupTypeMySQL:
		createErr = r.createMySQL(defPath, target)
	case biz.BackupTypePostgres:
		createErr = r.createPostgres(defPath, target)
	case biz.BackupTypePanel:
		createErr = r.createPanel(defPath)
	default:
		return errors.New(r.t.Get("unknown backup type"))
	}

	if createErr != nil {
		return createErr
	}

	// 记录日志
	r.log.Info("backup created", slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("backup_type", string(typ)), slog.String("target", target))

	return nil
}

// Delete 删除备份
func (r *backupRepo) Delete(ctx context.Context, typ biz.BackupType, name string) error {
	path, err := r.GetPath(typ)
	if err != nil {
		return err
	}

	file := filepath.Join(path, name)
	if err = io.Remove(file); err != nil {
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
		path, err := r.GetPath(typ)
		if err != nil {
			return err
		}
		backup = filepath.Join(path, backup)
	}

	var restoreErr error
	switch typ {
	case biz.BackupTypeWebsite:
		restoreErr = r.restoreWebsite(backup, target)
	case biz.BackupTypeMySQL:
		restoreErr = r.restoreMySQL(backup, target)
	case biz.BackupTypePostgres:
		restoreErr = r.restorePostgres(backup, target)
	default:
		return errors.New(r.t.Get("unknown backup type"))
	}

	if restoreErr != nil {
		return restoreErr
	}

	// 记录日志
	r.log.Info("backup restored", slog.String("type", biz.OperationTypeBackup), slog.Uint64("operator_id", getOperatorID(ctx)), slog.String("backup_type", string(typ)), slog.String("target", target))

	return nil
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

// GetPath 获取备份路径
func (r *backupRepo) GetPath(typ biz.BackupType) (string, error) {
	backupPath, err := r.setting.Get(biz.SettingKeyBackupPath)
	if err != nil {
		return "", err
	}
	if !slices.Contains([]biz.BackupType{biz.BackupTypePath, biz.BackupTypeWebsite, biz.BackupTypeMySQL, biz.BackupTypePostgres, biz.BackupTypeRedis, biz.BackupTypePanel}, typ) {
		return "", errors.New(r.t.Get("unknown backup type"))
	}

	backupPath = filepath.Join(backupPath, string(typ))
	if !io.Exists(backupPath) {
		if err = os.MkdirAll(backupPath, 0644); err != nil {
			return "", err
		}
	}

	return backupPath, nil
}

// createWebsite 创建网站备份
func (r *backupRepo) createWebsite(to string, name string) error {
	website, err := r.website.GetByName(name)
	if err != nil {
		return err
	}

	if err = r.preCheckPath(to, website.Path); err != nil {
		return err
	}

	start := time.Now()
	backup := filepath.Join(to, fmt.Sprintf("%s_%s.zip", website.Name, time.Now().Format("20060102150405")))
	if err = io.Compress(website.Path, nil, backup); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.t.Get("|-Backed up to file: %s", filepath.Base(backup)))
	}
	return nil
}

// createMySQL 创建 MySQL 备份
func (r *backupRepo) createMySQL(to string, name string) error {
	rootPassword, err := r.setting.Get(biz.SettingKeyMySQLRootPassword)
	if err != nil {
		return err
	}
	mysql, err := db.NewMySQL("root", rootPassword, "/tmp/mysql.sock", "unix")
	if err != nil {
		return err
	}
	defer mysql.Close()
	if exist, _ := mysql.DatabaseExists(name); !exist {
		return errors.New(r.t.Get("database does not exist: %s", name))
	}
	size, err := mysql.DatabaseSize(name)
	if err != nil {
		return err
	}
	if err = r.preCheckDB(to, size); err != nil {
		return err
	}

	if err = os.Setenv("MYSQL_PWD", rootPassword); err != nil {
		return err
	}
	start := time.Now()
	backup := filepath.Join(to, fmt.Sprintf("%s_%s.sql", name, time.Now().Format("20060102150405")))
	if _, err = shell.Execf(`mysqldump -u root '%s' > '%s'`, name, backup); err != nil {
		return err
	}
	if err = os.Unsetenv("MYSQL_PWD"); err != nil {
		return err
	}

	if err = io.Compress(filepath.Dir(backup), []string{filepath.Base(backup)}, backup+".zip"); err != nil {
		return err
	}
	if err = io.Remove(backup); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.t.Get("|-Backed up to file: %s", filepath.Base(backup+".zip")))
	}
	return nil
}

// createPostgres 创建 PostgreSQL 备份
func (r *backupRepo) createPostgres(to string, name string) error {
	postgres, err := db.NewPostgres("postgres", "", "127.0.0.1", 5432)
	if err != nil {
		return err
	}
	defer postgres.Close()
	if exist, _ := postgres.DatabaseExists(name); !exist {
		return errors.New(r.t.Get("database does not exist: %s", name))
	}
	size, err := postgres.DatabaseSize(name)
	if err != nil {
		return err
	}
	if err = r.preCheckDB(to, size); err != nil {
		return err
	}

	start := time.Now()
	backup := filepath.Join(to, fmt.Sprintf("%s_%s.sql", name, time.Now().Format("20060102150405")))
	if _, err = shell.Execf(`su - postgres -c "pg_dump '%s'" > '%s'`, name, backup); err != nil {
		return err
	}

	if err = io.Compress(filepath.Dir(backup), []string{filepath.Base(backup)}, backup+".zip"); err != nil {
		return err
	}
	if err = io.Remove(backup); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.t.Get("|-Backed up to file: %s", filepath.Base(backup+".zip")))
	}
	return nil
}

// createPanel 创建面板备份
func (r *backupRepo) createPanel(to string) error {
	backup := filepath.Join(to, fmt.Sprintf("panel_%s.zip", time.Now().Format("20060102150405")))

	if err := r.preCheckPath(to, filepath.Join(app.Root, "panel")); err != nil {
		return err
	}

	start := time.Now()

	temp, err := os.MkdirTemp("", "panel-backup")
	if err != nil {
		return err
	}

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

	return io.Remove(temp)
}

// restoreWebsite 恢复网站备份
func (r *backupRepo) restoreWebsite(backup, target string) error {
	if !io.Exists(backup) {
		return errors.New(r.t.Get("backup file %s not exists", backup))
	}

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
	if !io.Exists(backup) {
		return errors.New(r.t.Get("backup file %s not exists", backup))
	}

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
	if !io.Exists(backup) {
		return errors.New(r.t.Get("backup file %s not exists", backup))
	}

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

// preCheckPath 预检空间和 inode 是否足够
// to 备份保存目录
// path 待备份目录
func (r *backupRepo) preCheckPath(to, path string) error {
	size, err := io.SizeX(path)
	if err != nil {
		return err
	}
	files, err := io.CountX(path)
	if err != nil {
		return err
	}

	usage, err := disk.Usage(to)
	if err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Target size: %s", tools.FormatBytes(float64(size))))
		fmt.Println(r.t.Get("|-Target file count: %d", files))
		fmt.Println(r.t.Get("|-Backup directory available space: %s", tools.FormatBytes(float64(usage.Free))))
		fmt.Println(r.t.Get("|-Backup directory available Inode: %d", usage.InodesFree))
	}

	if uint64(size) > usage.Free {
		return errors.New(r.t.Get("Insufficient backup directory space"))
	}
	// 对于 fuse 等文件系统，可能没有 inode 的概念
	/*if uint64(files) > usage.InodesFree {
		return errors.New(r.t.Get("Insufficient backup directory inode"))
	}*/

	return nil
}

// preCheckDB 预检空间和 inode 是否足够
// to 备份保存目录
// size 数据库大小
func (r *backupRepo) preCheckDB(to string, size int64) error {
	usage, err := disk.Usage(to)
	if err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Target size: %s", tools.FormatBytes(float64(size))))
		fmt.Println(r.t.Get("|-Backup directory available space: %s", tools.FormatBytes(float64(usage.Free))))
		fmt.Println(r.t.Get("|-Backup directory available Inode: %d", usage.InodesFree))
	}

	if uint64(size) > usage.Free {
		return errors.New(r.t.Get("Insufficient backup directory space"))
	}

	return nil
}

// autoUnCompressSQL 自动处理压缩文件
func (r *backupRepo) autoUnCompressSQL(backup string) (string, error) {
	temp, err := os.MkdirTemp("", "sql-uncompress")
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
	list, err := r.List(biz.BackupTypePanel)
	if err != nil {
		return err
	}
	slices.SortFunc(list, func(a *types.BackupFile, b *types.BackupFile) int {
		return int(b.Time.Unix() - a.Time.Unix())
	})
	if len(list) == 0 {
		return errors.New(r.t.Get("No backup file found, unable to automatically repair"))
	}
	latest := list[0]
	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup file used: %s", latest.Name))
	}

	// 解压备份文件
	if app.IsCli {
		fmt.Println(r.t.Get("|-Unzip backup file..."))
	}
	if err = io.Remove("/tmp/panel-fix"); err != nil {
		return errors.New(r.t.Get("Cleaning temporary directory failed: %v", err))
	}
	if err = io.UnCompress(latest.Path, "/tmp/panel-fix"); err != nil {
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
	if err := r.Create(context.Background(), biz.BackupTypePanel, ""); err != nil {
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
