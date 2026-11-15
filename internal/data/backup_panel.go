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

	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
)

// CreatePanel 创建面板备份
// 面板备份始终保存在本地
func (r *backupRepo) CreatePanel(ctx context.Context) error {
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

	// 先打到临时名再改名，中途被打断（如 cron 停止）留下的截断包不会有合法备份文件名，
	// 否则 FixPanel 会按 mtime 选中它去覆盖运行中的面板；改名成功后临时文件已不存在，Remove 无害
	tmp := backup + ".partial"
	defer func() { _ = os.Remove(tmp) }()

	// 两个 -C 把 panel 内的核心文件与 panel 外的 cli 二进制收进同一个包
	if _, err := shell.Execf(
		ctx, "tar -cJf '%s' -C '%s' %s -C /usr/local/sbin acepanel",
		tmp, app.Root, strings.Join(files, " "),
	); err != nil {
		return err
	}
	if err := io.Chmod(tmp, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, backup); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Backup time: %s", time.Since(start).String()))
		fmt.Println(r.t.Get("|-Backup file: %s", filepath.Base(backup)))
	}

	return nil
}

func (r *backupRepo) FixPanel(ctx context.Context) error {
	if app.IsCli {
		fmt.Println(r.t.Get("|-Start fixing the panel..."))
	}

	// 检查关键文件是否正常
	panelBroken := !io.Exists(filepath.Join(app.Root, "panel", "ace")) ||
		!io.Exists(filepath.Join(app.Root, "panel", "storage", "config.yml")) ||
		!io.Exists(filepath.Join(app.Root, "panel", "storage", "panel.db"))
	// 检查主数据库完整性
	if !r.quickCheck(r.db) {
		panelBroken = true
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		panelBroken = true
	}

	// 检查辅助数据库是否异常
	var brokenAuxDBs []string
	for _, name := range []string{"stat", "scan", "tamper", "monitor"} {
		auxDB, err := openDB(name)
		if err != nil {
			brokenAuxDBs = append(brokenAuxDBs, name)
			continue
		}
		ok := r.quickCheck(auxDB)
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

	// 有异常，先同步停掉面板再动文件，避免运行中的面板往新目录写东西
	// 停掉面板后的修复被取消会让面板起不来，后续全程不可取消
	ctx = context.WithoutCancel(ctx)
	if err := systemctl.Stop(ctx, "acepanel"); err != nil {
		return errors.New(r.t.Get("Failed to stop panel: %v", err))
	}

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

	// 仅辅助数据库异常，重新启动即可恢复
	if !panelBroken {
		if err := systemctl.Start(ctx, "acepanel"); err != nil {
			return errors.New(r.t.Get("Failed to start panel: %v", err))
		}
		if app.IsCli {
			fmt.Println(r.t.Get("|-Fix completed"))
		}
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

	return r.restorePanel(ctx, latestPath)
}

// quickCheck 用 PRAGMA quick_check 检测 SQLite 数据库是否损坏
func (r *backupRepo) quickCheck(db *gorm.DB) bool {
	var result string
	if err := db.Raw("PRAGMA quick_check").Row().Scan(&result); err != nil {
		return false
	}
	return result == "ok"
}

// restorePanel 用指定的面板备份覆盖当前面板，完成后重启面板
// 面板内恢复备份与 CLI 修复共用，重启必须走 RestartPanel 的延迟排程，面板内调用才不会自杀在半路
func (r *backupRepo) restorePanel(ctx context.Context, backup string) error {
	// 覆盖的是运行中的面板二进制与 systemd 单元，中途取消会留下起不来的面板
	ctx = context.WithoutCancel(ctx)
	// 解压备份文件
	if app.IsCli {
		fmt.Println(r.t.Get("|-Unzip backup file..."))
	}
	if err := io.Remove("/tmp/panel-fix"); err != nil {
		return errors.New(r.t.Get("Cleaning temporary directory failed: %v", err))
	}
	if err := io.UnCompress(ctx, backup, "/tmp/panel-fix"); err != nil {
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
			_ = io.Mv(ctx, customize, keep)
		}
		if err := io.Remove(filepath.Join(app.Root, "panel")); err != nil {
			return errors.New(r.t.Get("Remove panel file failed: %v", err))
		}
		if err := io.Mv(ctx, filepath.Join("/tmp/panel-fix", "panel"), filepath.Join(app.Root, "panel")); err != nil {
			return errors.New(r.t.Get("Move panel file failed: %v", err))
		}
		if io.Exists(keep) {
			_ = io.Remove(customize)
			_ = io.Mv(ctx, keep, customize)
		}
	}
	if io.Exists(filepath.Join("/tmp/panel-fix", "acepanel")) {
		if err := io.Mv(ctx, filepath.Join("/tmp/panel-fix", "acepanel"), "/usr/local/sbin/acepanel"); err != nil {
			return errors.New(r.t.Get("Move acepanel file failed: %v", err))
		}
	}

	// 下载服务文件
	if !io.Exists("/etc/systemd/system/acepanel.service") {
		if _, err := shell.Execf(ctx, `wget -O /etc/systemd/system/acepanel.service https://%s/acepanel.service && sed -i "s|/opt/ace|%s|g" /etc/systemd/system/acepanel.service`, r.conf.App.DownloadEndpoint, app.Root); err != nil {
			return err
		}
	}

	// 处理权限
	if app.IsCli {
		fmt.Println(r.t.Get("|-Set key file permissions..."))
	}
	if err := io.ChmodR(ctx, filepath.Join(app.Root, "panel"), 0700); err != nil {
		return err
	}
	if err := io.Chmod(filepath.Join(app.Root, "panel", "storage", "config.yml"), 0600); err != nil {
		return err
	}
	if err := io.Chmod(filepath.Join(app.Root, "panel", "storage", "panel.db"), 0600); err != nil {
		return err
	}
	if err := io.Chmod("/etc/systemd/system/acepanel.service", 0644); err != nil {
		return err
	}
	if err := io.Chmod("/usr/local/sbin/acepanel", 0700); err != nil {
		return err
	}

	if err := io.Remove("/tmp/panel-fix"); err != nil {
		return err
	}

	if app.IsCli {
		fmt.Println(r.t.Get("|-Fix completed"))
	}

	tools.RestartPanel(ctx)
	return nil
}

// UpdatePanel 升级面板
func (r *backupRepo) UpdatePanel(ctx context.Context, version, url, checksum string, progress func(string)) error {
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
		app.Status.Store(app.StatusNormal)
		return err
	}

	app.Status.Store(app.StatusUpgrade)

	progress(r.t.Get("Preparing to update to %s...", version))
	if err := io.Remove(workDir); err != nil {
		return rollback(errors.New(r.t.Get("Failed to clean up temporary directory: %v", err)))
	}
	if err := r.db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		return rollback(errors.New(r.t.Get("Failed to optimize database: %v", err)))
	}

	// 备份留档
	progress(r.t.Get("Backing up panel data..."))
	if err := r.CreatePanel(ctx); err != nil {
		r.log.Warn(r.t.Get("failed to backup panel before update"), slog.Any("err", err))
	}

	// 下载新版本
	if err := os.MkdirAll(workDir, 0700); err != nil {
		return rollback(errors.New(r.t.Get("Failed to create temporary directory: %v", err)))
	}
	progress(r.t.Get("Downloading new version..."))
	if _, err := shell.Execf(ctx, "aria2c -c --file-allocation=falloc --allow-overwrite=true --auto-file-renaming=false --retry-wait=5 --max-tries=5 -x 16 -s 16 -k 1M -d %s -o %s %s", workDir, name, url); err != nil {
		return rollback(errors.New(r.t.Get("Download failed: %v", err)))
	}
	if _, err := shell.Execf(ctx, "aria2c -c --file-allocation=falloc --allow-overwrite=true --auto-file-renaming=false --retry-wait=5 --max-tries=5 -x 1 -s 1 -k 1M -d %s -o %s %s", workDir, name+".sha256", checksum); err != nil {
		return rollback(errors.New(r.t.Get("Download failed: %v", err)))
	}
	if !io.Exists(filepath.Join(workDir, name)) || !io.Exists(filepath.Join(workDir, name+".sha256")) {
		return rollback(errors.New(r.t.Get("Download file check failed")))
	}

	// 校验 sha256
	progress(r.t.Get("Verifying download file..."))
	check, err := shell.ExecfWithDir(ctx, workDir, "sha256sum -c %s --ignore-missing", name+".sha256")
	if err == nil && check != name+": OK" {
		err = fmt.Errorf("unexpected sha256sum output: %s", check)
	}
	if err != nil {
		return rollback(errors.New(r.t.Get("Verify download file failed: %v", err)))
	}

	// 解压
	progress(r.t.Get("Extracting new version..."))
	if err := io.UnCompress(ctx, filepath.Join(workDir, name), newDir); err != nil {
		return rollback(errors.New(r.t.Get("Unzip new version failed: %v", err)))
	}
	if !io.Exists(filepath.Join(newDir, "ace")) {
		return rollback(errors.New(r.t.Get("Unzip new version failed, missing file")))
	}

	// 应用
	// 下载和备份都可以随时放弃，但从这里开始替换的是运行中的面板二进制与 systemd 单元，
	// 中途取消会留下起不来的面板
	applyCtx := context.WithoutCancel(ctx)
	progress(r.t.Get("Applying update..."))
	if err := r.applyUpdate(applyCtx, newDir); err != nil {
		return rollback(errors.New(r.t.Get("Applying update failed: %v", err)))
	}

	// 收尾
	progress(r.t.Get("Finishing up..."))
	if err := r.finishUpdate(applyCtx, version); err != nil {
		return rollback(errors.New(r.t.Get("Finishing update failed: %v", err)))
	}

	_ = io.Remove(workDir)
	progress(r.t.Get("Update completed"))

	// 由调用方重启面板，数据库连接在进程退出时统一关闭
	return nil
}

// applyUpdate 用 newDir 下的新版本文件替换 panel/ 中的程序文件
func (r *backupRepo) applyUpdate(ctx context.Context, newDir string) error {
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
			if err = io.Cp(ctx, src, tmp); err != nil {
				return err
			}
			if err = io.Mv(ctx, tmp, "/usr/local/sbin/acepanel"); err != nil {
				return err
			}
			continue
		}
		// 其余程序文件
		if err = io.Mv(ctx, src, filepath.Join(panelDir, name)); err != nil {
			return err
		}
	}
	return nil
}

// finishUpdate 升级收尾
func (r *backupRepo) finishUpdate(ctx context.Context, version string) error {
	panelDir := filepath.Join(app.Root, "panel")
	serviceFile := "/etc/systemd/system/acepanel.service"

	// 更新 service 文件
	tmpService := serviceFile + ".new"
	if _, err := shell.Execf(ctx, `wget -O %s https://%s/acepanel.service`, tmpService, r.conf.App.DownloadEndpoint); err == nil {
		_, _ = shell.Execf(ctx, `sed -i "s|/opt/ace|%s|g" %s`, app.Root, tmpService)
		if out, _ := shell.Execf(ctx, "grep -c ExecStart %s", tmpService); strings.TrimSpace(out) != "0" {
			_ = io.Mv(ctx, tmpService, serviceFile) // 同在 /etc/systemd/system → 同分区 rename
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
	_, _ = shell.Execf(ctx, "systemctl daemon-reload")

	// 执行后置脚本
	_, _ = shell.Execf(ctx, "curl -sSLm 10 --fail --retry 3 https://%s/auto_update.sh | bash", r.conf.App.DownloadEndpoint)

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
	_, _ = shell.Execf(ctx, "restorecon %s /usr/local/sbin/acepanel %s", filepath.Join(panelDir, "ace"), serviceFile)

	return nil
}
