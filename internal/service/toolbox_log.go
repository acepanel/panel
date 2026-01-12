package service

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/io"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/tools"
)

type ToolboxLogService struct {
	t                  *gotext.Locale
	db                 *gorm.DB
	containerImageRepo biz.ContainerImageRepo
	settingRepo        biz.SettingRepo
}

func NewToolboxLogService(t *gotext.Locale, db *gorm.DB, containerImageRepo biz.ContainerImageRepo, settingRepo biz.SettingRepo) *ToolboxLogService {
	return &ToolboxLogService{
		t:                  t,
		db:                 db,
		containerImageRepo: containerImageRepo,
		settingRepo:        settingRepo,
	}
}

// LogItem 日志项信息
type LogItem struct {
	Name string `json:"name"` // 日志名称
	Path string `json:"path"` // 日志路径
	Size string `json:"size"` // 日志大小
}

// Scan 扫描日志
func (s *ToolboxLogService) Scan(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxLogClean](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	var items []LogItem

	switch req.Type {
	case "panel":
		items = s.scanPanelLogs()
	case "website":
		items = s.scanWebsiteLogs()
	case "mysql":
		items = s.scanMySQLLogs()
	case "docker":
		items = s.scanDockerLogs()
	case "system":
		items = s.scanSystemLogs()
	default:
		Error(w, http.StatusUnprocessableEntity, s.t.Get("unknown log type"))
		return
	}

	Success(w, items)
}

// Clean 清理日志
func (s *ToolboxLogService) Clean(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxLogClean](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	var cleaned int64
	var cleanErr error

	switch req.Type {
	case "panel":
		cleaned, cleanErr = s.cleanPanelLogs()
	case "website":
		cleaned, cleanErr = s.cleanWebsiteLogs()
	case "mysql":
		cleaned, cleanErr = s.cleanMySQLLogs()
	case "docker":
		cleaned, cleanErr = s.cleanDockerLogs()
	case "system":
		cleaned, cleanErr = s.cleanSystemLogs()
	default:
		Error(w, http.StatusUnprocessableEntity, s.t.Get("unknown log type"))
		return
	}

	if cleanErr != nil {
		Error(w, http.StatusInternalServerError, "%v", cleanErr)
		return
	}

	Success(w, chix.M{
		"cleaned": tools.FormatBytes(float64(cleaned)),
	})
}

// scanPanelLogs 扫描面板日志
func (s *ToolboxLogService) scanPanelLogs() []LogItem {
	var items []LogItem
	logPath := filepath.Join(app.Root, "panel/storage/logs")

	if !io.Exists(logPath) {
		return items
	}

	entries, err := os.ReadDir(logPath)
	if err != nil {
		return items
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, LogItem{
			Name: entry.Name(),
			Path: filepath.Join(logPath, entry.Name()),
			Size: tools.FormatBytes(float64(info.Size())),
		})
	}

	return items
}

// scanWebsiteLogs 扫描网站日志
func (s *ToolboxLogService) scanWebsiteLogs() []LogItem {
	var items []LogItem
	sitesPath := filepath.Join(app.Root, "sites")

	if !io.Exists(sitesPath) {
		return items
	}

	// 获取所有网站
	websites := make([]*biz.Website, 0)
	if err := s.db.Find(&websites).Error; err != nil {
		return items
	}

	for _, website := range websites {
		logPath := filepath.Join(sitesPath, website.Name, "log")
		if !io.Exists(logPath) {
			continue
		}

		entries, err := os.ReadDir(logPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			items = append(items, LogItem{
				Name: fmt.Sprintf("%s - %s", website.Name, entry.Name()),
				Path: filepath.Join(logPath, entry.Name()),
				Size: tools.FormatBytes(float64(info.Size())),
			})
		}
	}

	return items
}

// scanMySQLLogs 扫描 MySQL 日志
func (s *ToolboxLogService) scanMySQLLogs() []LogItem {
	var items []LogItem
	mysqlPath := filepath.Join(app.Root, "server/mysql")

	if !io.Exists(mysqlPath) {
		return items
	}

	// 慢查询日志
	slowLogPath := filepath.Join(mysqlPath, "mysql-slow.log")
	if io.Exists(slowLogPath) {
		if info, err := os.Stat(slowLogPath); err == nil {
			items = append(items, LogItem{
				Name: "mysql-slow.log",
				Path: slowLogPath,
				Size: tools.FormatBytes(float64(info.Size())),
			})
		}
	}

	// 二进制日志
	entries, err := os.ReadDir(mysqlPath)
	if err != nil {
		return items
	}

	binLogRegex := regexp.MustCompile(`^mysql-bin\.\d+$`)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if binLogRegex.MatchString(entry.Name()) {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			items = append(items, LogItem{
				Name: entry.Name(),
				Path: filepath.Join(mysqlPath, entry.Name()),
				Size: tools.FormatBytes(float64(info.Size())),
			})
		}
	}

	return items
}

// scanDockerLogs 扫描 Docker/Podman 相关内容
func (s *ToolboxLogService) scanDockerLogs() []LogItem {
	var items []LogItem

	// 未使用的容器镜像 (Docker)
	images, err := s.containerImageRepo.List()
	if err == nil {
		// 计算未使用的镜像
		var unusedCount int
		for _, img := range images {
			if img.Containers == 0 {
				unusedCount++
			}
		}

		if unusedCount > 0 {
			items = append(items, LogItem{
				Name: s.t.Get("Unused container images: %d", unusedCount),
				Path: "docker:images",
				Size: s.t.Get("%d images", unusedCount),
			})
		}
	}

	// Docker 容器日志路径
	dockerLogPath := "/var/lib/docker/containers"
	if io.Exists(dockerLogPath) {
		totalSize, logCount := s.scanContainerLogDir(dockerLogPath)
		if logCount > 0 {
			items = append(items, LogItem{
				Name: s.t.Get("Docker container logs: %d files", logCount),
				Path: "docker:logs",
				Size: tools.FormatBytes(float64(totalSize)),
			})
		}
	}

	// Podman 容器日志路径
	podmanLogPaths := []string{
		"/var/lib/containers/storage/overlay-containers",
		"/run/containers/storage/overlay-containers",
	}
	for _, podmanLogPath := range podmanLogPaths {
		if io.Exists(podmanLogPath) {
			totalSize, logCount := s.scanContainerLogDir(podmanLogPath)
			if logCount > 0 {
				items = append(items, LogItem{
					Name: s.t.Get("Podman container logs: %d files", logCount),
					Path: "podman:logs",
					Size: tools.FormatBytes(float64(totalSize)),
				})
				break
			}
		}
	}

	return items
}

// scanContainerLogDir 扫描容器日志目录
func (s *ToolboxLogService) scanContainerLogDir(logPath string) (int64, int) {
	var totalSize int64
	var logCount int

	entries, err := os.ReadDir(logPath)
	if err != nil {
		return 0, 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		containerPath := filepath.Join(logPath, entry.Name())
		// 扫描 *.log 文件
		logFiles, _ := filepath.Glob(filepath.Join(containerPath, "*.log"))
		for _, logFile := range logFiles {
			if info, err := os.Stat(logFile); err == nil {
				totalSize += info.Size()
				logCount++
			}
		}
		// 扫描 userdata 子目录下的日志 (Podman)
		userdataPath := filepath.Join(containerPath, "userdata")
		if io.Exists(userdataPath) {
			userdataLogs, _ := filepath.Glob(filepath.Join(userdataPath, "*.log"))
			for _, logFile := range userdataLogs {
				if info, err := os.Stat(logFile); err == nil {
					totalSize += info.Size()
					logCount++
				}
			}
		}
	}

	return totalSize, logCount
}

// scanSystemLogs 扫描系统日志
func (s *ToolboxLogService) scanSystemLogs() []LogItem {
	var items []LogItem

	logFiles := []string{
		"/var/log/syslog",
		"/var/log/messages",
		"/var/log/auth.log",
		"/var/log/secure",
		"/var/log/kern.log",
		"/var/log/dmesg",
		"/var/log/btmp",
		"/var/log/wtmp",
		"/var/log/lastlog",
	}

	for _, logFile := range logFiles {
		if !io.Exists(logFile) {
			continue
		}
		info, err := os.Stat(logFile)
		if err != nil {
			continue
		}
		items = append(items, LogItem{
			Name: filepath.Base(logFile),
			Path: logFile,
			Size: tools.FormatBytes(float64(info.Size())),
		})
	}

	// /var/log/*.log 文件
	logPattern := "/var/log/*.log"
	matches, _ := filepath.Glob(logPattern)
	for _, match := range matches {
		// 跳过已经添加的文件
		if lo.Contains(logFiles, match) {
			continue
		}
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		items = append(items, LogItem{
			Name: filepath.Base(match),
			Path: match,
			Size: tools.FormatBytes(float64(info.Size())),
		})
	}

	// journal 日志大小
	journalOutput, _ := shell.Execf("journalctl --disk-usage 2>/dev/null | grep -oP '\\d+\\.?\\d*[KMGT]?' || echo '0'")
	journalSize := strings.TrimSpace(journalOutput)
	if journalSize != "" && journalSize != "0" {
		items = append(items, LogItem{
			Name: s.t.Get("Journal logs"),
			Path: "system:journal",
			Size: journalSize,
		})
	}

	return items
}

// cleanPanelLogs 清理面板日志
func (s *ToolboxLogService) cleanPanelLogs() (int64, error) {
	var cleaned int64
	logPath := filepath.Join(app.Root, "panel/storage/logs")

	if !io.Exists(logPath) {
		return 0, nil
	}

	entries, err := os.ReadDir(logPath)
	if err != nil {
		return 0, err
	}

	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(logPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		cleaned += info.Size()
		// 名称带日期的日志文件，删除旧文件
		if re.MatchString(entry.Name()) {
			_ = os.Remove(filePath)
		} else {
			_, _ = shell.Execf("cat /dev/null > '%s'", filePath)
		}
	}

	return cleaned, nil
}

// cleanWebsiteLogs 清理网站日志
func (s *ToolboxLogService) cleanWebsiteLogs() (int64, error) {
	var cleaned int64
	sitesPath := filepath.Join(app.Root, "sites")

	if !io.Exists(sitesPath) {
		return 0, nil
	}

	websites := make([]*biz.Website, 0)
	if err := s.db.Find(&websites).Error; err != nil {
		return 0, err
	}

	for _, website := range websites {
		logPath := filepath.Join(sitesPath, website.Name, "log")
		if !io.Exists(logPath) {
			continue
		}

		entries, err := os.ReadDir(logPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			filePath := filepath.Join(logPath, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}
			cleaned += info.Size()
			if _, err = shell.Execf("cat /dev/null > '%s'", filePath); err != nil {
				continue
			}
		}
	}

	return cleaned, nil
}

// cleanMySQLLogs 清理 MySQL 日志
func (s *ToolboxLogService) cleanMySQLLogs() (int64, error) {
	var cleaned int64
	mysqlPath := filepath.Join(app.Root, "server/mysql")

	if !io.Exists(mysqlPath) {
		return 0, nil
	}

	// 清空慢查询日志
	slowLogPath := filepath.Join(mysqlPath, "mysql-slow.log")
	if io.Exists(slowLogPath) {
		if info, err := os.Stat(slowLogPath); err == nil {
			cleaned += info.Size()
			_, _ = shell.Execf("cat /dev/null > '%s'", slowLogPath)
		}
	}

	// 清理二进制日志
	entries, err := os.ReadDir(mysqlPath)
	if err != nil {
		return cleaned, nil
	}

	binLogRegex := regexp.MustCompile(`^mysql-bin\.\d+$`)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if binLogRegex.MatchString(entry.Name()) {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			cleaned += info.Size()
		}
	}

	// 尝试通过 MySQL 清理二进制日志
	// 从面板设置获取 root 密码
	rootPassword, err := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
	if err == nil && rootPassword != "" {
		// 设置环境变量
		if err = os.Setenv("MYSQL_PWD", rootPassword); err == nil {
			_, _ = shell.Execf("mysql -u root -e 'PURGE BINARY LOGS BEFORE NOW()' 2>/dev/null")
			_ = os.Unsetenv("MYSQL_PWD")
		}
	}

	return cleaned, nil
}

// cleanDockerLogs 清理 Docker/Podman 相关内容
func (s *ToolboxLogService) cleanDockerLogs() (int64, error) {
	var cleaned int64

	// 清理未使用的镜像 (Docker)
	_ = s.containerImageRepo.Prune()

	// 清理 Docker 容器日志
	dockerLogPath := "/var/lib/docker/containers"
	cleaned += s.cleanContainerLogDir(dockerLogPath)

	// 清理 Podman 容器日志
	podmanLogPaths := []string{
		"/var/lib/containers/storage/overlay-containers",
		"/run/containers/storage/overlay-containers",
	}
	for _, podmanLogPath := range podmanLogPaths {
		cleaned += s.cleanContainerLogDir(podmanLogPath)
	}

	// 清理 Docker 系统
	_, _ = shell.Execf("docker system prune -f 2>/dev/null")

	// 清理 Podman 系统
	_, _ = shell.Execf("podman system prune -f 2>/dev/null")

	return cleaned, nil
}

// cleanContainerLogDir 清理容器日志目录
func (s *ToolboxLogService) cleanContainerLogDir(logPath string) int64 {
	var cleaned int64

	if !io.Exists(logPath) {
		return 0
	}

	entries, err := os.ReadDir(logPath)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		containerPath := filepath.Join(logPath, entry.Name())

		// 清理 *.log 文件
		logFiles, _ := filepath.Glob(filepath.Join(containerPath, "*.log"))
		for _, logFile := range logFiles {
			if info, err := os.Stat(logFile); err == nil {
				cleaned += info.Size()
				_, _ = shell.Execf("cat /dev/null > '%s'", logFile)
			}
		}

		// 清理 userdata 子目录下的日志 (Podman)
		userdataPath := filepath.Join(containerPath, "userdata")
		if io.Exists(userdataPath) {
			userdataLogs, _ := filepath.Glob(filepath.Join(userdataPath, "*.log"))
			for _, logFile := range userdataLogs {
				if info, err := os.Stat(logFile); err == nil {
					cleaned += info.Size()
					_, _ = shell.Execf("cat /dev/null > '%s'", logFile)
				}
			}
		}
	}

	return cleaned
}

// cleanSystemLogs 清理系统日志
func (s *ToolboxLogService) cleanSystemLogs() (int64, error) {
	var cleaned int64

	// 清理 journal 日志 (保留最近 1 天)
	_, _ = shell.Execf("journalctl --vacuum-time=1d 2>/dev/null")

	logFiles := []string{
		"/var/log/syslog",
		"/var/log/messages",
		"/var/log/auth.log",
		"/var/log/secure",
		"/var/log/kern.log",
		"/var/log/dmesg",
		"/var/log/btmp",
		"/var/log/wtmp",
	}

	for _, logFile := range logFiles {
		if !io.Exists(logFile) {
			continue
		}
		info, err := os.Stat(logFile)
		if err != nil {
			continue
		}
		cleaned += info.Size()
		// 清空日志文件
		_, _ = shell.Execf("cat /dev/null > '%s'", logFile)
	}

	// 清理 /var/log/*.log 文件
	matches, _ := filepath.Glob("/var/log/*.log")
	for _, match := range matches {
		if lo.Contains(logFiles, match) {
			continue
		}
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		cleaned += info.Size()
		_, _ = shell.Execf("cat /dev/null > '%s'", match)
	}

	return cleaned, nil
}
