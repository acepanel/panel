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
}

func NewToolboxLogService(t *gotext.Locale, db *gorm.DB, containerImageRepo biz.ContainerImageRepo) *ToolboxLogService {
	return &ToolboxLogService{
		t:                  t,
		db:                 db,
		containerImageRepo: containerImageRepo,
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

// scanDockerLogs 扫描 Docker 相关内容
func (s *ToolboxLogService) scanDockerLogs() []LogItem {
	var items []LogItem

	// 未使用的容器镜像
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
		entries, err := os.ReadDir(dockerLogPath)
		if err == nil {
			var totalSize int64
			var logCount int
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				containerPath := filepath.Join(dockerLogPath, entry.Name())
				logFiles, _ := filepath.Glob(filepath.Join(containerPath, "*.log"))
				for _, logFile := range logFiles {
					if info, err := os.Stat(logFile); err == nil {
						totalSize += info.Size()
						logCount++
					}
				}
			}
			if logCount > 0 {
				items = append(items, LogItem{
					Name: s.t.Get("Container logs: %d files", logCount),
					Path: "docker:logs",
					Size: tools.FormatBytes(float64(totalSize)),
				})
			}
		}
	}

	return items
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
	_, _ = shell.Execf("mysql -u root -e 'PURGE BINARY LOGS BEFORE NOW()' 2>/dev/null")

	return cleaned, nil
}

// cleanDockerLogs 清理 Docker 相关内容
func (s *ToolboxLogService) cleanDockerLogs() (int64, error) {
	var cleaned int64

	// 清理未使用的镜像
	if err := s.containerImageRepo.Prune(); err != nil {
		// 忽略 Docker 未安装或未运行的错误
		if !strings.Contains(err.Error(), "Cannot connect") &&
			!strings.Contains(err.Error(), "No such file") {
			return 0, err
		}
	}

	// 清理容器日志
	dockerLogPath := "/var/lib/docker/containers"
	if io.Exists(dockerLogPath) {
		entries, err := os.ReadDir(dockerLogPath)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				containerPath := filepath.Join(dockerLogPath, entry.Name())
				logFiles, _ := filepath.Glob(filepath.Join(containerPath, "*.log"))
				for _, logFile := range logFiles {
					if info, err := os.Stat(logFile); err == nil {
						cleaned += info.Size()
						// 清空日志文件
						_, _ = shell.Execf("cat /dev/null > '%s'", logFile)
					}
				}
			}
		}
	}

	// 清理 Docker 系统
	_, _ = shell.Execf("docker system prune -f 2>/dev/null")

	return cleaned, nil
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
