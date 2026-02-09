package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/types"
)

// migrationState 全局迁移状态（内部实现）
type migrationState struct {
	mu         sync.RWMutex
	Step       types.MigrationStep                 `json:"step"`
	Connection *request.ToolboxMigrationConnection `json:"connection,omitempty"`
	Items      *request.ToolboxMigrationItems      `json:"items,omitempty"`
	Results    []types.MigrationItemResult         `json:"results"`
	Logs       []string                            `json:"logs"`
	StartedAt  *time.Time                          `json:"started_at"`
	EndedAt    *time.Time                          `json:"ended_at"`
}

// ToolboxMigrationService 迁移服务
type ToolboxMigrationService struct {
	t               *gotext.Locale
	conf            *config.Config
	log             *slog.Logger
	settingRepo     biz.SettingRepo
	websiteRepo     biz.WebsiteRepo
	databaseRepo    biz.DatabaseRepo
	projectRepo     biz.ProjectRepo
	appRepo         biz.AppRepo
	environmentRepo biz.EnvironmentRepo

	state migrationState
}

// NewToolboxMigrationService 创建迁移服务
func NewToolboxMigrationService(
	t *gotext.Locale,
	conf *config.Config,
	log *slog.Logger,
	setting biz.SettingRepo,
	website biz.WebsiteRepo,
	database biz.DatabaseRepo,
	project biz.ProjectRepo,
	appRepo biz.AppRepo,
	environment biz.EnvironmentRepo,
) *ToolboxMigrationService {
	return &ToolboxMigrationService{
		t:               t,
		conf:            conf,
		log:             log,
		settingRepo:     setting,
		websiteRepo:     website,
		databaseRepo:    database,
		projectRepo:     project,
		appRepo:         appRepo,
		environmentRepo: environment,
		state: migrationState{
			Step: types.MigrationStepIdle,
		},
	}
}

// GetStatus 获取当前迁移状态
func (s *ToolboxMigrationService) GetStatus(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	Success(w, chix.M{
		"step":       s.state.Step,
		"results":    s.state.Results,
		"started_at": s.state.StartedAt,
		"ended_at":   s.state.EndedAt,
	})
}

// PreCheck 连接远程服务器并获取环境信息
func (s *ToolboxMigrationService) PreCheck(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxMigrationConnection](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	// 检查是否有正在进行的迁移
	s.state.mu.RLock()
	if s.state.Step == types.MigrationStepRunning {
		s.state.mu.RUnlock()
		Error(w, http.StatusConflict, s.t.Get("migration is already running"))
		return
	}
	s.state.mu.RUnlock()

	// 请求远程面板 InstalledEnvironment 接口
	remoteEnv, err := s.fetchRemoteEnvironment(req)
	if err != nil {
		Error(w, http.StatusBadGateway, s.t.Get("failed to connect remote server: %v", err))
		return
	}

	// 保存连接信息
	s.state.mu.Lock()
	s.state.Connection = req
	s.state.Step = types.MigrationStepPreCheck
	s.state.mu.Unlock()

	Success(w, chix.M{
		"remote": remoteEnv,
	})
}

// GetItems 获取本地可迁移项
func (s *ToolboxMigrationService) GetItems(w http.ResponseWriter, r *http.Request) {
	// 网站列表
	websites, _, err := s.websiteRepo.List("", 1, 10000)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get website list: %v", err))
		return
	}

	// 数据库列表
	databases, _, err := s.databaseRepo.List(1, 10000)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get database list: %v", err))
		return
	}

	// 项目列表
	projects, _, err := s.projectRepo.List("", 1, 10000)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get project list: %v", err))
		return
	}

	s.state.mu.Lock()
	if s.state.Step == types.MigrationStepPreCheck {
		s.state.Step = types.MigrationStepSelect
	}
	s.state.mu.Unlock()

	Success(w, chix.M{
		"websites":  websites,
		"databases": databases,
		"projects":  projects,
	})
}

// Start 开始迁移
func (s *ToolboxMigrationService) Start(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxMigrationItems](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	s.state.mu.Lock()
	if s.state.Step == types.MigrationStepRunning {
		s.state.mu.Unlock()
		Error(w, http.StatusConflict, s.t.Get("migration is already running"))
		return
	}
	if s.state.Connection == nil {
		s.state.mu.Unlock()
		Error(w, http.StatusBadRequest, s.t.Get("please complete pre-check first"))
		return
	}

	now := time.Now()
	s.state.Step = types.MigrationStepRunning
	s.state.Items = req
	s.state.Results = nil
	s.state.Logs = nil
	s.state.StartedAt = &now
	s.state.EndedAt = nil
	conn := *s.state.Connection
	s.state.mu.Unlock()

	// 异步执行迁移
	go s.runMigration(&conn, req)

	Success(w, nil)
}

// Reset 重置迁移状态
func (s *ToolboxMigrationService) Reset(w http.ResponseWriter, r *http.Request) {
	s.state.mu.Lock()
	if s.state.Step == types.MigrationStepRunning {
		s.state.mu.Unlock()
		Error(w, http.StatusConflict, s.t.Get("migration is running, cannot reset"))
		return
	}
	s.state.Step = types.MigrationStepIdle
	s.state.Connection = nil
	s.state.Items = nil
	s.state.Results = nil
	s.state.Logs = nil
	s.state.StartedAt = nil
	s.state.EndedAt = nil
	s.state.mu.Unlock()

	Success(w, nil)
}

// GetResults 获取迁移结果
func (s *ToolboxMigrationService) GetResults(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	Success(w, chix.M{
		"step":       s.state.Step,
		"results":    s.state.Results,
		"logs":       s.state.Logs,
		"started_at": s.state.StartedAt,
		"ended_at":   s.state.EndedAt,
	})
}

// Progress 通过 WebSocket 推送迁移进度
func (s *ToolboxMigrationService) Progress(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	}
	if s.conf.App.Debug {
		opts.InsecureSkipVerify = true
	}

	ws, err := websocket.Accept(w, r, opts)
	if err != nil {
		s.log.Warn("[Migration] websocket upgrade error", slog.Any("err", err))
		return
	}
	defer func() { _ = ws.CloseNow() }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	lastLogIdx := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.state.mu.RLock()
			msg := chix.M{
				"step":       s.state.Step,
				"results":    s.state.Results,
				"started_at": s.state.StartedAt,
				"ended_at":   s.state.EndedAt,
			}
			// 增量发送日志
			if len(s.state.Logs) > lastLogIdx {
				msg["new_logs"] = s.state.Logs[lastLogIdx:]
				lastLogIdx = len(s.state.Logs)
			}
			s.state.mu.RUnlock()

			data, _ := json.Marshal(msg)
			if err = ws.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}

			// 迁移完成后发送最终状态并关闭
			s.state.mu.RLock()
			done := s.state.Step == types.MigrationStepDone || s.state.Step == types.MigrationStepIdle
			s.state.mu.RUnlock()
			if done {
				_ = ws.Close(websocket.StatusNormalClosure, "")
				return
			}
		}
	}
}

// runMigration 执行迁移流程
func (s *ToolboxMigrationService) runMigration(conn *request.ToolboxMigrationConnection, items *request.ToolboxMigrationItems) {
	s.addLog("===== " + s.t.Get("Migration started") + " =====")

	// 迁移网站
	for _, site := range items.Websites {
		s.migrateWebsite(conn, &site, items.StopOnMig)
	}

	// 迁移数据库
	for _, db := range items.Databases {
		s.migrateDatabase(conn, &db, items.StopOnMig)
	}

	// 迁移项目
	for _, proj := range items.Projects {
		s.migrateProject(conn, &proj, items.StopOnMig)
	}

	now := time.Now()
	s.state.mu.Lock()
	s.state.Step = types.MigrationStepDone
	s.state.EndedAt = &now
	s.state.mu.Unlock()

	s.addLog("===== " + s.t.Get("Migration completed") + " =====")
}

// migrateWebsite 迁移单个网站
func (s *ToolboxMigrationService) migrateWebsite(conn *request.ToolboxMigrationConnection, site *request.ToolboxMigrationWebsite, stopOnMig bool) {
	result := types.MigrationItemResult{
		Type:   "website",
		Name:   site.Name,
		Status: types.MigrationItemRunning,
	}
	now := time.Now()
	result.StartedAt = &now
	s.addResult(result)

	s.addLog(fmt.Sprintf("[%s] %s: %s", s.t.Get("Website"), s.t.Get("start migrating"), site.Name))

	// 获取网站详情
	websiteDetail, err := s.websiteRepo.Get(site.ID)
	if err != nil {
		s.failResult("website", site.Name, s.t.Get("failed to get website detail: %v", err))
		return
	}

	// 在远程面板创建网站
	s.addLog(fmt.Sprintf("[%s] %s", site.Name, s.t.Get("creating website on remote server")))
	createBody := map[string]any{
		"name":    websiteDetail.Name,
		"domains": websiteDetail.Listens,
		"path":    websiteDetail.Path,
		"type":    "static",
		"php":     0,
	}
	_, err = s.remoteAPIRequest(conn, "POST", "/api/website", createBody)
	if err != nil {
		s.addLog(fmt.Sprintf("[%s] %s: %v", site.Name, s.t.Get("warning: failed to create remote website, trying rsync directly"), err))
	}

	// 用 rsync 发送网站文件
	siteDir := fmt.Sprintf("/opt/ace/sites/%s/", site.Name)
	s.addLog(fmt.Sprintf("[%s] %s %s", site.Name, s.t.Get("syncing directory:"), siteDir))

	remoteHost := extractHost(conn.URL)
	rsyncCmd := fmt.Sprintf("rsync -avz --progress -e 'ssh -o StrictHostKeyChecking=no' %s root@%s:%s", siteDir, remoteHost, siteDir)
	s.addLog(fmt.Sprintf("$ %s", rsyncCmd))

	output, err := shell.Exec(rsyncCmd)
	if output != "" {
		s.addLog(output)
	}
	if err != nil {
		s.failResult("website", site.Name, s.t.Get("rsync failed: %v", err))
		return
	}

	// 如果有自定义路径，也需要同步
	if site.Path != "" && site.Path != siteDir+"public" && site.Path != siteDir {
		s.addLog(fmt.Sprintf("[%s] %s %s", site.Name, s.t.Get("syncing custom directory:"), site.Path))
		rsyncCmd = fmt.Sprintf("rsync -avz --progress -e 'ssh -o StrictHostKeyChecking=no' %s/ root@%s:%s/", site.Path, remoteHost, site.Path)
		s.addLog(fmt.Sprintf("$ %s", rsyncCmd))
		output, err = shell.Exec(rsyncCmd)
		if output != "" {
			s.addLog(output)
		}
		if err != nil {
			s.addLog(fmt.Sprintf("[%s] %s: %v", site.Name, s.t.Get("warning: custom path sync failed"), err))
		}
	}

	s.succeedResult("website", site.Name)
	s.addLog(fmt.Sprintf("[%s] %s", site.Name, s.t.Get("website migration completed")))
}

// migrateDatabase 迁移单个数据库
func (s *ToolboxMigrationService) migrateDatabase(conn *request.ToolboxMigrationConnection, db *request.ToolboxMigrationDatabase, stopOnMig bool) {
	result := types.MigrationItemResult{
		Type:   "database",
		Name:   fmt.Sprintf("%s (%s)", db.Name, db.Type),
		Status: types.MigrationItemRunning,
	}
	now := time.Now()
	result.StartedAt = &now
	s.addResult(result)

	s.addLog(fmt.Sprintf("[%s] %s: %s (%s)", s.t.Get("Database"), s.t.Get("start migrating"), db.Name, db.Type))

	remoteHost := extractHost(conn.URL)
	backupPath := fmt.Sprintf("/tmp/ace_migration_%s_%s.sql", db.Type, db.Name)

	var dumpCmd, restoreCmd string
	switch db.Type {
	case "mysql":
		rootPassword, _ := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
		dumpCmd = fmt.Sprintf("mysqldump -uroot -p'%s' --socket=/tmp/mysql.sock --single-transaction --quick '%s' > %s", rootPassword, db.Name, backupPath)
		restoreCmd = fmt.Sprintf("rsync -avz --progress -e 'ssh -o StrictHostKeyChecking=no' %s root@%s:%s", backupPath, remoteHost, backupPath)
	case "postgresql":
		dumpCmd = fmt.Sprintf("sudo -u postgres pg_dump '%s' > %s", db.Name, backupPath)
		restoreCmd = fmt.Sprintf("rsync -avz --progress -e 'ssh -o StrictHostKeyChecking=no' %s root@%s:%s", backupPath, remoteHost, backupPath)
	default:
		s.failResult("database", db.Name, s.t.Get("unsupported database type: %s", db.Type))
		return
	}

	// 导出数据库
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("exporting database")))
	s.addLog(fmt.Sprintf("$ %s", maskPassword(dumpCmd)))
	_, err := shell.Exec(dumpCmd)
	if err != nil {
		s.failResult("database", db.Name, s.t.Get("database export failed: %v", err))
		return
	}

	// 发送备份文件到远程
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("sending backup to remote server")))
	s.addLog(fmt.Sprintf("$ %s", restoreCmd))
	output, err := shell.Exec(restoreCmd)
	if output != "" {
		s.addLog(output)
	}
	if err != nil {
		s.failResult("database", db.Name, s.t.Get("backup transfer failed: %v", err))
		return
	}

	// 在远程导入数据库
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("importing database on remote server")))
	var remoteImportCmd string
	switch db.Type {
	case "mysql":
		// 先在远程创建数据库，再导入
		createBody := map[string]any{
			"type":      "mysql",
			"name":      db.Name,
			"server_id": 0,
			"encoding":  "utf8mb4",
		}
		_, _ = s.remoteAPIRequest(conn, "POST", "/api/database", createBody)
		remoteImportCmd = fmt.Sprintf("ssh -o StrictHostKeyChecking=no root@%s \"mysql -uroot --socket=/tmp/mysql.sock '%s' < %s\"", remoteHost, db.Name, backupPath)
	case "postgresql":
		createBody := map[string]any{
			"type":      "postgresql",
			"name":      db.Name,
			"server_id": 0,
			"encoding":  "utf8",
		}
		_, _ = s.remoteAPIRequest(conn, "POST", "/api/database", createBody)
		remoteImportCmd = fmt.Sprintf("ssh -o StrictHostKeyChecking=no root@%s \"sudo -u postgres psql '%s' < %s\"", remoteHost, db.Name, backupPath)
	}

	s.addLog(fmt.Sprintf("$ %s", remoteImportCmd))
	output, err = shell.Exec(remoteImportCmd)
	if output != "" {
		s.addLog(output)
	}
	if err != nil {
		s.failResult("database", db.Name, s.t.Get("remote import failed: %v", err))
		return
	}

	// 清理临时文件
	_, _ = shell.Exec(fmt.Sprintf("rm -f %s", backupPath))

	s.succeedResult("database", db.Name)
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("database migration completed")))
}

// migrateProject 迁移单个项目
func (s *ToolboxMigrationService) migrateProject(conn *request.ToolboxMigrationConnection, proj *request.ToolboxMigrationProject, stopOnMig bool) {
	result := types.MigrationItemResult{
		Type:   "project",
		Name:   proj.Name,
		Status: types.MigrationItemRunning,
	}
	now := time.Now()
	result.StartedAt = &now
	s.addResult(result)

	s.addLog(fmt.Sprintf("[%s] %s: %s", s.t.Get("Project"), s.t.Get("start migrating"), proj.Name))

	// 获取项目详情
	projectDetail, err := s.projectRepo.Get(proj.ID)
	if err != nil {
		s.failResult("project", proj.Name, s.t.Get("failed to get project detail: %v", err))
		return
	}

	// 在远程面板创建项目
	s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("creating project on remote server")))
	createBody := map[string]any{
		"name": projectDetail.Name,
		"type": projectDetail.Type,
		"path": projectDetail.RootDir,
	}
	_, err = s.remoteAPIRequest(conn, "POST", "/api/project", createBody)
	if err != nil {
		s.addLog(fmt.Sprintf("[%s] %s: %v", proj.Name, s.t.Get("warning: failed to create remote project, trying rsync directly"), err))
	}

	remoteHost := extractHost(conn.URL)

	// 同步项目目录
	if proj.Path != "" {
		s.addLog(fmt.Sprintf("[%s] %s %s", proj.Name, s.t.Get("syncing directory:"), proj.Path))
		rsyncCmd := fmt.Sprintf("rsync -avz --progress -e 'ssh -o StrictHostKeyChecking=no' %s/ root@%s:%s/", proj.Path, remoteHost, proj.Path)
		s.addLog(fmt.Sprintf("$ %s", rsyncCmd))
		output, err := shell.Exec(rsyncCmd)
		if output != "" {
			s.addLog(output)
		}
		if err != nil {
			s.failResult("project", proj.Name, s.t.Get("rsync failed: %v", err))
			return
		}
	}

	// 同步 systemd 服务文件
	serviceName := fmt.Sprintf("ace-project-%s", proj.Name)
	serviceFile := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("syncing systemd service file")))
	rsyncCmd := fmt.Sprintf("rsync -avz --progress -e 'ssh -o StrictHostKeyChecking=no' %s root@%s:%s", serviceFile, remoteHost, serviceFile)
	s.addLog(fmt.Sprintf("$ %s", rsyncCmd))
	output, err := shell.Exec(rsyncCmd)
	if output != "" {
		s.addLog(output)
	}
	if err != nil {
		s.addLog(fmt.Sprintf("[%s] %s: %v", proj.Name, s.t.Get("warning: service file sync failed"), err))
	}

	s.succeedResult("project", proj.Name)
	s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("project migration completed")))
}

// addLog 添加日志
func (s *ToolboxMigrationService) addLog(msg string) {
	s.state.mu.Lock()
	s.state.Logs = append(s.state.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	s.state.mu.Unlock()
	s.log.Info("[Migration] " + msg)
}

// addResult 添加迁移结果
func (s *ToolboxMigrationService) addResult(result types.MigrationItemResult) {
	s.state.mu.Lock()
	s.state.Results = append(s.state.Results, result)
	s.state.mu.Unlock()
}

// failResult 标记迁移项失败
func (s *ToolboxMigrationService) failResult(typ, name, errMsg string) {
	s.state.mu.Lock()
	for i := range s.state.Results {
		if s.state.Results[i].Type == typ && s.state.Results[i].Name == name {
			s.state.Results[i].Status = types.MigrationItemFailed
			s.state.Results[i].Error = errMsg
			now := time.Now()
			s.state.Results[i].EndedAt = &now
			if s.state.Results[i].StartedAt != nil {
				s.state.Results[i].Duration = now.Sub(*s.state.Results[i].StartedAt).Seconds()
			}
			break
		}
	}
	s.state.mu.Unlock()
	s.addLog(fmt.Sprintf("❌ %s [%s]: %s", s.t.Get("failed"), name, errMsg))
}

// succeedResult 标记迁移项成功
func (s *ToolboxMigrationService) succeedResult(typ, name string) {
	s.state.mu.Lock()
	for i := range s.state.Results {
		if s.state.Results[i].Type == typ && s.state.Results[i].Name == name {
			s.state.Results[i].Status = types.MigrationItemSuccess
			now := time.Now()
			s.state.Results[i].EndedAt = &now
			if s.state.Results[i].StartedAt != nil {
				s.state.Results[i].Duration = now.Sub(*s.state.Results[i].StartedAt).Seconds()
			}
			break
		}
	}
	s.state.mu.Unlock()
}

// fetchRemoteEnvironment 获取远程面板的环境信息
func (s *ToolboxMigrationService) fetchRemoteEnvironment(conn *request.ToolboxMigrationConnection) (map[string]any, error) {
	body, err := s.remoteAPIRequest(conn, "GET", "/api/home/installed_environment", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err = json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}

	return resp.Data, nil
}

// remoteAPIRequest 向远程面板发送 API 请求
func (s *ToolboxMigrationService) remoteAPIRequest(conn *request.ToolboxMigrationConnection, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
		reqBody = bytes.NewReader(bodyBytes)
	}

	url := strings.TrimRight(conn.URL, "/") + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// 签名请求
	if err = signRequest(req, conn.TokenID, conn.Token); err != nil {
		return nil, fmt.Errorf("sign request failed: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return respBody, fmt.Errorf("remote API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// signRequest 对请求进行 HMAC-SHA256 签名
func signRequest(req *http.Request, tokenID uint, token string) error {
	var body []byte
	var err error

	if req.Body != nil {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	// 规范化路径
	canonicalPath := req.URL.Path
	if !strings.HasPrefix(canonicalPath, "/api") {
		index := strings.Index(canonicalPath, "/api")
		if index != -1 {
			canonicalPath = canonicalPath[index:]
		}
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s",
		req.Method,
		canonicalPath,
		req.URL.Query().Encode(),
		sha256Hex(string(body)))

	timestamp := time.Now().Unix()
	req.Header.Set("X-Timestamp", fmt.Sprintf("%d", timestamp))

	stringToSign := fmt.Sprintf("%s\n%d\n%s",
		"HMAC-SHA256",
		timestamp,
		sha256Hex(canonicalRequest))

	signature := hmacSHA256(stringToSign, token)

	authHeader := fmt.Sprintf("HMAC-SHA256 Credential=%d, Signature=%s", tokenID, signature)
	req.Header.Set("Authorization", authHeader)

	return nil
}

func sha256Hex(str string) string {
	sum := sha256.Sum256([]byte(str))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(data, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// extractHost 从 URL 中提取主机地址
func extractHost(rawURL string) string {
	// 去掉协议前缀
	host := rawURL
	if idx := strings.Index(host, "://"); idx != -1 {
		host = host[idx+3:]
	}
	// 去掉路径和端口
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}
	return host
}

// maskPassword 掩盖命令中的密码
func maskPassword(cmd string) string {
	// 简单掩盖 -p'...' 模式的密码
	if idx := strings.Index(cmd, "-p'"); idx != -1 {
		end := strings.Index(cmd[idx+3:], "'")
		if end != -1 {
			return cmd[:idx+3] + "***" + cmd[idx+3+end:]
		}
	}
	return cmd
}
