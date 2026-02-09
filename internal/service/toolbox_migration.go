package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/acepanel/panel/internal/app"
	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/types"
)

const migrationChunkSize = 10 << 20 // 10MB

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

// Exec SSE 实时执行命令（供远程面板调用）
func (s *ToolboxMigrationService) Exec(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ToolboxMigrationExec](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	cmd := exec.Command("bash", "-c", req.Command)
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err = cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// 等待命令结束后关闭 pipe writer
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		_ = pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", scanner.Text())
		flusher.Flush()
	}

	// 发送完成/错误事件
	if waitErr := <-waitCh; waitErr != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", waitErr.Error())
	} else {
		_, _ = fmt.Fprintf(w, "event: done\ndata: ok\n\n")
	}
	flusher.Flush()
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
	websites, _, err := s.websiteRepo.List("all", 1, 10000)
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

// DownloadLog 下载迁移日志
func (s *ToolboxMigrationService) DownloadLog(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	logs := make([]string, len(s.state.Logs))
	copy(logs, s.state.Logs)
	s.state.mu.RUnlock()

	if len(logs) == 0 {
		Error(w, http.StatusNotFound, s.t.Get("no migration logs available"))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=migration.log")
	_, _ = w.Write([]byte(strings.Join(logs, "\n")))
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

	// 迁移前停止网站
	if stopOnMig {
		s.addLog(fmt.Sprintf("[%s] %s", site.Name, s.t.Get("stopping website")))
		if err := s.websiteRepo.UpdateStatus(site.ID, false); err != nil {
			s.addLog(fmt.Sprintf("[%s] %s: %v", site.Name, s.t.Get("warning: failed to stop website"), err))
		}
	}

	// 获取网站详情
	websiteDetail, err := s.websiteRepo.Get(site.ID)
	if err != nil {
		s.failResult("website", site.Name, s.t.Get("failed to get website detail: %v", err))
		return
	}

	// 在远程面板创建网站
	s.addLog(fmt.Sprintf("[%s] %s", site.Name, s.t.Get("creating website on remote server")))
	var listens []string
	for _, l := range websiteDetail.Listens {
		listens = append(listens, l.Address)
	}
	if len(listens) == 0 {
		listens = []string{"80"}
	}
	websiteCreateReq := &request.WebsiteCreate{
		Type:    websiteDetail.Type,
		Name:    websiteDetail.Name,
		Listens: listens,
		Domains: websiteDetail.Domains,
		Path:    websiteDetail.Path,
		PHP:     websiteDetail.PHP,
	}
	if websiteDetail.Type == "proxy" && len(websiteDetail.Proxies) > 0 {
		websiteCreateReq.Proxy = websiteDetail.Proxies[0].Pass
	}
	_, err = s.remoteAPIRequest(conn, "POST", "/api/website", websiteCreateReq)
	if err != nil {
		s.addLog(fmt.Sprintf("[%s] %s: %v", site.Name, s.t.Get("warning: failed to create remote website, trying to upload directly"), err))
	}

	// 上传网站目录到远程
	siteDir := filepath.Join(app.Root, "sites", site.Name)
	s.addLog(fmt.Sprintf("[%s] %s %s", site.Name, s.t.Get("uploading directory:"), siteDir))
	if err = s.uploadDirToRemote(conn, siteDir, siteDir); err != nil {
		s.failResult("website", site.Name, s.t.Get("upload failed: %v", err))
		return
	}

	// 如果有自定义路径，也需要上传
	if site.Path != "" && site.Path != filepath.Join(siteDir, "public") && site.Path != siteDir {
		s.addLog(fmt.Sprintf("[%s] %s %s", site.Name, s.t.Get("uploading website directory:"), site.Path))
		if err = s.uploadDirToRemote(conn, site.Path, site.Path); err != nil {
			s.addLog(fmt.Sprintf("[%s] %s: %v", site.Name, s.t.Get("warning: website directory upload failed"), err))
		}
	}

	s.succeedResult("website", site.Name)
	s.addLog(fmt.Sprintf("[%s] %s", site.Name, s.t.Get("website migration completed")))
}

// migrateDatabase 迁移单个数据库
func (s *ToolboxMigrationService) migrateDatabase(conn *request.ToolboxMigrationConnection, db *request.ToolboxMigrationDatabase, stopOnMig bool) {
	displayName := fmt.Sprintf("%s (%s)", db.Name, db.Type)
	result := types.MigrationItemResult{
		Type:   "database",
		Name:   displayName,
		Status: types.MigrationItemRunning,
	}
	now := time.Now()
	result.StartedAt = &now
	s.addResult(result)

	s.addLog(fmt.Sprintf("[%s] %s: %s", s.t.Get("Database"), s.t.Get("start migrating"), displayName))

	backupPath := fmt.Sprintf("/tmp/ace_migration_%s_%s.sql", db.Type, db.Name)

	var dumpCmd string
	switch db.Type {
	case "mysql":
		rootPassword, _ := s.settingRepo.Get(biz.SettingKeyMySQLRootPassword)
		dumpCmd = fmt.Sprintf("MYSQL_PWD='%s' mysqldump -u root --single-transaction --quick '%s' > %s", rootPassword, db.Name, backupPath)
	case "postgresql":
		postgresPassword, _ := s.settingRepo.Get(biz.SettingKeyPostgresPassword)
		dumpCmd = fmt.Sprintf("PGPASSWORD='%s' pg_dump -h 127.0.0.1 -U postgres '%s' > %s", postgresPassword, db.Name, backupPath)
	default:
		s.failResult("database", displayName, s.t.Get("unsupported database type: %s", db.Type))
		return
	}

	// 导出数据库
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("exporting database")))
	s.addLog(fmt.Sprintf("$ %s", s.maskPassword(dumpCmd)))
	_, err := shell.Exec(dumpCmd)
	if err != nil {
		s.failResult("database", displayName, s.t.Get("database export failed: %v", err))
		return
	}
	defer func() { _ = os.Remove(backupPath) }()

	// 在远程创建数据库
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("creating database on remote server")))
	dbCreateReq := &request.DatabaseCreate{
		ServerID: db.ServerID,
		Name:     db.Name,
	}
	_, _ = s.remoteAPIRequest(conn, "POST", "/api/database", dbCreateReq)

	// 分片上传备份文件到远程
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("sending backup to remote server")))
	if err = s.remoteUploadFile(conn, backupPath, backupPath); err != nil {
		s.failResult("database", displayName, s.t.Get("backup transfer failed: %v", err))
		return
	}

	// 在远程执行导入命令
	s.addLog(fmt.Sprintf("[%s] %s", db.Name, s.t.Get("importing database on remote server")))
	var remoteImportCmd string
	switch db.Type {
	case "mysql":
		remoteImportCmd = fmt.Sprintf("MYSQL_PWD=$(acepanel setting get mysql_root_password) mysql -u root '%s' < %s && rm -f %s", db.Name, backupPath, backupPath)
	case "postgresql":
		remoteImportCmd = fmt.Sprintf("PGPASSWORD=$(acepanel setting get postgres_password) psql -h 127.0.0.1 -U postgres '%s' < %s && rm -f %s", db.Name, backupPath, backupPath)
	}

	if err = s.remoteExec(conn, remoteImportCmd); err != nil {
		s.failResult("database", displayName, s.t.Get("remote import failed: %v", err))
		return
	}

	s.succeedResult("database", displayName)
	s.addLog(fmt.Sprintf("[%s] %s", displayName, s.t.Get("database migration completed")))
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

	// 迁移前停止项目服务
	if stopOnMig {
		s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("stopping project service")))
		if _, err := shell.Exec(fmt.Sprintf("systemctl stop %s", proj.Name)); err != nil {
			s.addLog(fmt.Sprintf("[%s] %s: %v", proj.Name, s.t.Get("warning: failed to stop service"), err))
		}
	}

	// 获取项目详情
	projectDetail, err := s.projectRepo.Get(proj.ID)
	if err != nil {
		s.failResult("project", proj.Name, s.t.Get("failed to get project detail: %v", err))
		return
	}

	// 在远程面板创建项目
	s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("creating project on remote server")))
	projectCreateReq := &request.ProjectCreate{
		Name:        projectDetail.Name,
		Type:        projectDetail.Type,
		Description: projectDetail.Description,
		RootDir:     projectDetail.RootDir,
		WorkingDir:  projectDetail.WorkingDir,
		ExecStart:   projectDetail.ExecStart,
		User:        projectDetail.User,
	}
	_, err = s.remoteAPIRequest(conn, "POST", "/api/project", projectCreateReq)
	if err != nil {
		s.addLog(fmt.Sprintf("[%s] %s: %v", proj.Name, s.t.Get("warning: failed to create remote project, trying to upload directly"), err))
	}

	// 上传项目目录
	if proj.Path != "" {
		s.addLog(fmt.Sprintf("[%s] %s %s", proj.Name, s.t.Get("uploading directory:"), proj.Path))
		if err = s.uploadDirToRemote(conn, proj.Path, proj.Path); err != nil {
			s.failResult("project", proj.Name, s.t.Get("upload failed: %v", err))
			return
		}
	}

	// 上传 systemd 服务文件
	serviceFile := fmt.Sprintf("/etc/systemd/system/%s.service", proj.Name)
	if _, statErr := os.Stat(serviceFile); statErr == nil {
		s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("uploading systemd service file")))
		if err = s.remoteUploadFile(conn, serviceFile, serviceFile); err != nil {
			s.addLog(fmt.Sprintf("[%s] %s: %v", proj.Name, s.t.Get("warning: service file upload failed"), err))
		} else {
			// 远程重新加载 systemd
			_ = s.remoteExec(conn, "systemctl daemon-reload")
		}
	}

	s.succeedResult("project", proj.Name)
	s.addLog(fmt.Sprintf("[%s] %s", proj.Name, s.t.Get("project migration completed")))
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

// remoteExec 调用远程面板 SSE exec 接口执行命令
func (s *ToolboxMigrationService) remoteExec(conn *request.ToolboxMigrationConnection, command string) error {
	bodyBytes, _ := json.Marshal(map[string]string{"command": command})

	url := strings.TrimRight(conn.URL, "/") + "/api/toolbox_migration/exec"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if err = signRequest(req, conn.TokenID, conn.Token); err != nil {
		return fmt.Errorf("sign request failed: %w", err)
	}

	client := &http.Client{
		// SSE 流可能很长，不设超时
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote exec returned status %d: %s", resp.StatusCode, string(body))
	}

	// 读取 SSE 流
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			s.addLog("  " + data)
		} else if strings.HasPrefix(line, "event: error") {
			// 下一行是 data
			if scanner.Scan() {
				errData := strings.TrimPrefix(scanner.Text(), "data: ")
				return fmt.Errorf("remote exec error: %s", errData)
			}
		} else if strings.HasPrefix(line, "event: done") {
			// 读取 data 行然后返回
			scanner.Scan()
			return nil
		}
	}

	return scanner.Err()
}

// remoteUploadFile 分片上传文件到远程面板
func (s *ToolboxMigrationService) remoteUploadFile(conn *request.ToolboxMigrationConnection, localPath, remotePath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer func() { _ = f.Close() }()

	// 流式计算文件 SHA256
	hasher := sha256.New()
	if _, err = io.Copy(hasher, f); err != nil {
		return fmt.Errorf("hash file failed: %w", err)
	}
	fileHash := hex.EncodeToString(hasher.Sum(nil))
	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	info, err := f.Stat()
	if err != nil {
		return err
	}
	chunkCount := int((info.Size() + migrationChunkSize - 1) / migrationChunkSize)
	if chunkCount == 0 {
		chunkCount = 1
	}
	dir := filepath.Dir(remotePath)
	fileName := filepath.Base(remotePath)

	// 调用远程 chunk/start
	startReq := map[string]any{
		"path":        dir,
		"file_name":   fileName,
		"file_hash":   fileHash,
		"chunk_count": chunkCount,
		"force":       true,
	}
	startResp, err := s.remoteAPIRequest(conn, "POST", "/api/file/chunk/start", startReq)
	if err != nil {
		return fmt.Errorf("chunk start failed: %w", err)
	}

	// 解析已上传分片列表（支持断点续传）
	uploadedChunks := s.parseUploadedChunks(startResp)

	// 逐分片上传
	buf := make([]byte, migrationChunkSize)
	uploadStart := time.Now()
	var uploaded int64
	for i := 0; i < chunkCount; i++ {
		n, readErr := f.Read(buf)
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("read chunk %d failed: %w", i, readErr)
		}
		chunk := buf[:n]

		// 跳过已上传分片
		if slices.Contains(uploadedChunks, i) {
			uploaded += int64(n)
			continue
		}

		// 计算分片 hash
		h := sha256.Sum256(chunk)
		chunkHash := hex.EncodeToString(h[:])

		// multipart 上传
		_, err = s.remoteMultipartUpload(conn, "/api/file/chunk/upload", map[string]string{
			"path":        dir,
			"file_name":   fileName,
			"file_hash":   fileHash,
			"chunk_index": strconv.Itoa(i),
			"chunk_hash":  chunkHash,
		}, "file", fileName, chunk)
		if err != nil {
			return fmt.Errorf("upload chunk %d failed: %w", i, err)
		}

		uploaded += int64(n)
		elapsed := time.Since(uploadStart).Seconds()
		speed := float64(uploaded) / elapsed
		remaining := float64(info.Size()-uploaded) / speed

		s.addLog(fmt.Sprintf("  %s: %d/%d (%.1f%%) %s/s ETA %s",
			s.t.Get("uploading"), i+1, chunkCount, float64(i+1)/float64(chunkCount)*100,
			s.formatBytes(speed), s.formatETA(remaining)))
	}

	// 调用远程 chunk/finish
	finishReq := map[string]any{
		"path":        dir,
		"file_name":   fileName,
		"file_hash":   fileHash,
		"chunk_count": chunkCount,
		"force":       true,
	}
	_, err = s.remoteAPIRequest(conn, "POST", "/api/file/chunk/finish", finishReq)
	if err != nil {
		return fmt.Errorf("chunk finish failed: %w", err)
	}

	return nil
}

// remoteMultipartUpload 带签名的 multipart 上传
func (s *ToolboxMigrationService) remoteMultipartUpload(
	conn *request.ToolboxMigrationConnection, path string, fields map[string]string,
	fileField, fileName string, fileData []byte,
) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(fileData); err != nil {
		return nil, err
	}
	_ = writer.Close()

	url := strings.TrimRight(conn.URL, "/") + path
	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err = signRequest(req, conn.TokenID, conn.Token); err != nil {
		return nil, fmt.Errorf("sign request failed: %w", err)
	}

	client := &http.Client{
		Timeout:   5 * time.Minute,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
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
		return respBody, fmt.Errorf("multipart upload returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// uploadDirToRemote 上传目录到远程
func (s *ToolboxMigrationService) uploadDirToRemote(conn *request.ToolboxMigrationConnection, localDir, remoteDir string) error {
	tarPath := fmt.Sprintf("/tmp/ace_mig_%d.tar.xz", time.Now().UnixNano())

	// 本地打包
	s.addLog("  " + s.t.Get("compressing directory: %s", localDir))
	_, err := shell.Exec(fmt.Sprintf("tar cJf %s -C %s .", tarPath, localDir))
	if err != nil {
		return fmt.Errorf("compress failed: %w", err)
	}
	defer func() { _ = os.Remove(tarPath) }()

	// 分片上传到远程 /tmp/
	s.addLog("  " + s.t.Get("uploading to remote server"))
	if err = s.remoteUploadFile(conn, tarPath, tarPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	// 远程解压并清理
	s.addLog("  " + s.t.Get("extracting on remote server"))
	extractCmd := fmt.Sprintf("mkdir -p %s && tar xJf %s -C %s && rm -f %s", remoteDir, tarPath, remoteDir, tarPath)
	if err = s.remoteExec(conn, extractCmd); err != nil {
		return fmt.Errorf("remote extract failed: %w", err)
	}

	return nil
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

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
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

// maskPassword 掩盖命令中的密码
func (s *ToolboxMigrationService) maskPassword(cmd string) string {
	for _, prefix := range []string{"MYSQL_PWD='", "PGPASSWORD='"} {
		if idx := strings.Index(cmd, prefix); idx != -1 {
			start := idx + len(prefix)
			end := strings.Index(cmd[start:], "'")
			if end != -1 {
				return cmd[:idx] + prefix + "***" + cmd[start+end:]
			}
		}
	}
	return cmd
}

// formatBytes 格式化字节数为人类可读格式
func (s *ToolboxMigrationService) formatBytes(b float64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", b/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MB", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KB", b/(1<<10))
	default:
		return fmt.Sprintf("%.0f B", b)
	}
}

// formatETA 格式化剩余时间
func (s *ToolboxMigrationService) formatETA(seconds float64) string {
	if seconds < 0 || seconds > 86400 {
		return "--:--"
	}
	sec := int(seconds)
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm%ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%dh%dm", sec/3600, (sec%3600)/60)
}

// parseUploadedChunks 从 chunk/start 响应中解析已上传分片索引
func (s *ToolboxMigrationService) parseUploadedChunks(respBody []byte) []int {
	var resp struct {
		Data struct {
			UploadedChunks []int `json:"uploaded_chunks"`
		} `json:"data"`
	}
	if json.Unmarshal(respBody, &resp) == nil {
		return resp.Data.UploadedChunks
	}
	return nil
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
