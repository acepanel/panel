package service

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/collect"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	stdssh "golang.org/x/crypto/ssh"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/docker"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/ssh"
	"github.com/acepanel/panel/v3/pkg/tools"
)

type WsService struct {
	t           *gotext.Locale
	conf        *config.Config
	log         *slog.Logger
	api         *api.API
	sshRepo     *biz.SSHUsecase
	settingRepo *biz.SettingUsecase
	certRepo    *biz.CertUsecase
	backupRepo  *biz.BackupUsecase
	taskRepo    *biz.TaskUsecase
}

func NewWsService(backupUsecase *biz.BackupUsecase, certUsecase *biz.CertUsecase, sshUsecase *biz.SSHUsecase, settingUsecase *biz.SettingUsecase, taskUsecase *biz.TaskUsecase, conf *config.Config, t *gotext.Locale, log *slog.Logger) *WsService {
	return &WsService{
		t:           t,
		conf:        conf,
		log:         log,
		api:         api.NewAPI(app.Version, app.Locale),
		sshRepo:     sshUsecase,
		settingRepo: settingUsecase,
		certRepo:    certUsecase,
		backupRepo:  backupUsecase,
		taskRepo:    taskUsecase,
	}
}

func (s *WsService) Exec(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade exec ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	// 第一条消息是命令
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	_, cmd, err := ws.Read(ctx)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to read command: %v", err))
		return
	}

	out, err := shell.ExecfWithPipe(ctx, string(cmd))
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to run command: %v", err))
		return
	}

	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			line := scanner.Text()
			_ = ws.Write(ctx, websocket.MessageText, []byte(line))
		}
		if err = scanner.Err(); err != nil {
			_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to read command output: %v", err))
		}
	}()

	s.readLoop(ctx, ws)
}

// Follow 文件或 systemd 服务实时跟踪
// path 给定时用 tail -F；service 给定时用 journalctl -f
func (s *WsService) Follow(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.FileFollow](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	if req.Path == "" && req.Service == "" && req.Container == "" {
		Error(w, http.StatusUnprocessableEntity, s.t.Get("path, service or container is required"))
		return
	}

	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade follow ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	if req.Container != "" {
		s.followContainer(ctx, ws, req.Container)
		return
	}

	var cmd *exec.Cmd
	if req.Service != "" {
		cmd = exec.CommandContext(ctx, "journalctl", "--no-pager", "-n", "0", "-f", "-u", req.Service)
	} else if req.Offset > 0 {
		// 从首屏锚点接着跟踪，补上首屏读取到建立连接之间写入的日志（-c 的字节偏移从 1 开始）
		cmd = exec.CommandContext(ctx, "tail", "-c", fmt.Sprintf("+%d", req.Offset+1), "-F", req.Path)
	} else {
		cmd = exec.CommandContext(ctx, "tail", "-n", "0", "-F", req.Path)
	}
	shell.ApplyEnv(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel() // 进程输出结束时取消，促使下方读取循环退出
		buf := make([]byte, 4096)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				if werr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
			}
			if rerr != nil {
				return
			}
		}
	}()

	// 连接结束时取消 ctx 杀掉进程，待输出读取完成后 Wait 回收，避免残留僵尸进程
	defer func() {
		cancel()
		<-done
		_ = cmd.Wait()
	}()

	for {
		_, _, rerr := ws.Read(ctx)
		if rerr != nil {
			return
		}
	}
}

// PTY 通用 PTY 命令执行
// 前端发送第一条消息为要执行的命令，后端通过 PTY 执行并实时返回输出
func (s *WsService) PTY(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade pty ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 要执行的命令
	_, message, err := ws.Read(ctx)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to read command: %v", err))
		return
	}
	command := string(message)
	if command == "" {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("command is empty"))
		return
	}

	// PTY 执行命令
	turn, err := shell.NewPTYTurn(ctx, ws, command)
	if err != nil {
		_ = ws.Write(ctx, websocket.MessageBinary, []byte("\r\n"+s.t.Get("Failed to start command: %v", err)+"\r\n"))
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return
	}

	go func() {
		defer turn.Close()
		_ = turn.Handle(ctx)
	}()

	turn.Wait()
}

func (s *WsService) Session(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	info, err := s.sshRepo.Get(req.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade session ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	sshClient, err := ssh.NewSSHClient(info.Config)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}
	defer func(sshClient *stdssh.Client) { _ = sshClient.Close() }(sshClient)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	turn, err := ssh.NewTurn(ctx, ws, sshClient)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}

	go func() {
		defer turn.Close() // Handle 退出后关闭 SSH 连接，以结束 Wait 阶段
		_ = turn.Handle(ctx)
	}()

	turn.Wait()
}

// SSHTransfer 通过 WebSocket 在主机间传输文件并实时推送进度
// 连接后发送 JSON 参数,断开连接即取消传输
func (s *WsService) SSHTransfer(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade ssh transfer ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 读取参数,10 秒超时防止连接后不发消息
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	_, message, err := ws.Read(readCtx)
	readCancel()
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to read params: %v", err))
		return
	}
	var req request.SSHTransfer
	if err = json.Unmarshal(message, &req); err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("invalid params: %v", err))
		return
	}
	if req.SrcPath == "" || req.DstPath == "" {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("source and destination path are required"))
		return
	}
	if req.SrcID == req.DstID && req.SrcPath == req.DstPath {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("source and destination are the same file"))
		return
	}

	// 客户端断开时取消传输
	go func() {
		for {
			if _, _, rerr := ws.Read(ctx); rerr != nil {
				cancel()
				return
			}
		}
	}()

	// 进度节流推送,完成时必推
	var lastPush time.Time
	progress := func(transferred, total int64) {
		if transferred != total && time.Since(lastPush) < 500*time.Millisecond {
			return
		}
		lastPush = time.Now()
		data, _ := json.Marshal(map[string]any{
			"status":      "progress",
			"transferred": transferred,
			"total":       total,
		})
		_ = ws.Write(ctx, websocket.MessageText, data)
	}

	if err = s.sshRepo.TransferFile(ctx, req.SrcID, req.SrcPath, req.DstID, req.DstPath, progress); err != nil {
		errMsg, _ := json.Marshal(map[string]any{
			"status": "error",
			"msg":    err.Error(),
		})
		_ = ws.Write(ctx, websocket.MessageText, errMsg)
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return
	}

	successMsg, _ := json.Marshal(map[string]any{"status": "success"})
	_ = ws.Write(ctx, websocket.MessageText, successMsg)
	_ = ws.Close(websocket.StatusNormalClosure, "")
}

// ContainerTerminal 容器终端
func (s *WsService) ContainerTerminal(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.ContainerID](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade container terminal ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sock := s.getContainerSock()

	// 通过 /bin/sh 启动，优先切换到 bash 并强制交互模式；只对存在性探测静默 stderr，不影响实际运行的 Shell
	turn, err := docker.NewTurn(ctx, ws, req.ID, []string{"/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash -i; else exec sh -i; fi"}, sock)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to start container terminal: %v", err))
		return
	}

	go func() {
		defer turn.Close()
		_ = turn.Handle(ctx)
	}()

	turn.Wait()
}

// ContainerImagePull 镜像拉取
func (s *WsService) ContainerImagePull(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade image pull ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	_, message, err := ws.Read(ctx)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to read params: %v", err))
		return
	}
	var req request.ContainerImagePull
	if err = json.Unmarshal(message, &req); err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("invalid params: %v", err))
		return
	}

	// 创建 Docker 客户端
	apiClient, err := client.New(client.WithHost(s.getContainerSock()))
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to create docker client: %v", err))
		return
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 构建拉取选项
	options := client.ImagePullOptions{}
	if req.Auth {
		authConfig := registry.AuthConfig{
			Username: req.Username,
			Password: req.Password,
		}
		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to encode auth: %v", err))
			return
		}
		options.RegistryAuth = base64.URLEncoding.EncodeToString(encodedJSON)
	}

	// 拉取镜像
	resp, err := apiClient.ImagePull(ctx, req.Name, options)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to pull image: %v", err))
		return
	}

	// 迭代进度
	for msg, err := range resp.JSONMessages(ctx) {
		if err != nil {
			s.log.Warn("image pull error", slog.Any("err", err))
			errorMsg, _ := json.Marshal(map[string]any{
				"status": "error",
				"error":  err.Error(),
			})
			_ = ws.Write(ctx, websocket.MessageText, errorMsg)
			return
		}

		// 如果有错误，发送错误消息
		if msg.Error != nil {
			errorMsg, _ := json.Marshal(map[string]any{
				"status": "error",
				"error":  msg.Error.Message,
			})
			_ = ws.Write(ctx, websocket.MessageText, errorMsg)
			return
		}

		// 转发进度信息
		progressMsg, _ := json.Marshal(msg)
		if err = ws.Write(ctx, websocket.MessageText, progressMsg); err != nil {
			s.log.Warn("write image pull progress error", slog.Any("err", err))
			return
		}
	}

	// 拉取完成
	completeMsg, _ := json.Marshal(map[string]any{
		"status":   "complete",
		"complete": true,
	})
	_ = ws.Write(ctx, websocket.MessageText, completeMsg)
	_ = ws.Close(websocket.StatusNormalClosure, "")
}

// CertObtain 通过 WebSocket 签发证书并实时推送进度
func (s *WsService) CertObtain(w http.ResponseWriter, r *http.Request) {
	s.handleCertWs(w, r, "obtain", func(ctx context.Context, id uint, cb func(string)) error {
		_, err := s.certRepo.ObtainAutoWithProgressCallback(ctx, id, cb)
		return err
	})
}

// CertRenew 通过 WebSocket 续签证书并实时推送进度
func (s *WsService) CertRenew(w http.ResponseWriter, r *http.Request) {
	s.handleCertWs(w, r, "renew", func(ctx context.Context, id uint, cb func(string)) error {
		_, err := s.certRepo.RenewWithProgressCallback(ctx, id, cb)
		return err
	})
}

// PanelUpdate 通过 WebSocket 升级面板并实时推送进度
func (s *WsService) PanelUpdate(w http.ResponseWriter, r *http.Request) {
	// 前置检查在建连前完成（此时 app.Status 仍为 Normal，握手可过状态中间件）
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		Error(w, http.StatusForbidden, s.t.Get("unable to update in offline mode"))
		return
	}
	if s.taskRepo.HasRunningTask() {
		Error(w, http.StatusInternalServerError, s.t.Get("background task is running, updating is prohibited, please try again later"))
		return
	}
	channel, _ := s.settingRepo.Get(biz.SettingKeyChannel)
	panel, err := s.api.LatestVersion(channel)
	if err != nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the latest version: %v", err))
		return
	}
	download := collect.First(panel.Downloads)
	if download == nil {
		Error(w, http.StatusInternalServerError, s.t.Get("failed to get the latest version download link"))
		return
	}
	url := fmt.Sprintf("https://%s%s", s.conf.App.DownloadEndpoint, download.URL)
	checksum := fmt.Sprintf("https://%s%s", s.conf.App.DownloadEndpoint, download.Checksum)

	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("upgrade panel update ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	// 写入用带超时的独立 context：与请求/WS 生命周期解耦，
	// 用户关闭页面导致连接断开也不会中断升级（升级内部 shell 执行不带 ctx）
	write := func(status, msg string) {
		data, _ := json.Marshal(map[string]any{"status": status, "msg": msg})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ws.Write(ctx, websocket.MessageText, data)
	}

	if err = s.backupRepo.UpdatePanel(panel.Version, url, checksum, func(msg string) { write("progress", msg) }); err != nil {
		write("error", err.Error())
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return
	}

	write("success", "success")
	_ = ws.Close(websocket.StatusNormalClosure, "")

	// 升级成功，由本入口负责重启面板（唯一一次重启）
	tools.RestartPanel()
}

// handleCertWs 证书操作的公共 WebSocket 处理逻辑
func (s *WsService) handleCertWs(w http.ResponseWriter, r *http.Request, action string, fn func(ctx context.Context, id uint, cb func(string)) error) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn(fmt.Sprintf("upgrade cert %s ws error", action), slog.Any("err", err))
		return
	}
	defer func() { _ = ws.CloseNow() }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 读取参数，10 秒超时防止连接后不发消息
	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	_, message, err := ws.Read(readCtx)
	readCancel()
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to read params: %v", err))
		return
	}
	var req struct {
		ID uint `json:"id"`
	}
	if err = json.Unmarshal(message, &req); err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("invalid params: %v", err))
		return
	}

	progressCallback := func(msg string) {
		if ctx.Err() != nil {
			return
		}
		data, _ := json.Marshal(map[string]any{
			"status": "progress",
			"msg":    msg,
		})
		if err = ws.Write(ctx, websocket.MessageText, data); err != nil {
			s.log.Warn("write cert progress error", slog.Any("err", err), slog.String("action", action))
		}
	}

	if err = fn(ctx, req.ID, progressCallback); err != nil {
		errMsg, _ := json.Marshal(map[string]any{
			"status": "error",
			"msg":    err.Error(),
		})
		_ = ws.Write(ctx, websocket.MessageText, errMsg)
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return
	}

	completeMsg, _ := json.Marshal(map[string]any{
		"status": "success",
		"msg":    "success",
		"data":   nil,
	})
	_ = ws.Write(ctx, websocket.MessageText, completeMsg)
	_ = ws.Close(websocket.StatusNormalClosure, "")
}

// getContainerSock 获取容器 socket 路径
func (s *WsService) getContainerSock() string {
	sock, _ := s.settingRepo.Get(biz.SettingKeyContainerSock)
	if sock == "" {
		sock = "/var/run/docker.sock"
	}
	if !strings.Contains(sock, "://") {
		sock = "unix://" + sock
	}
	return sock
}

// wsBinaryWriter 将写入转发为 WebSocket 二进制帧
type wsBinaryWriter struct {
	ctx context.Context
	ws  *websocket.Conn
}

func (w *wsBinaryWriter) Write(p []byte) (int, error) {
	if err := w.ws.Write(w.ctx, websocket.MessageBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// followContainer 通过 Docker SDK 实时跟踪容器日志并转发到 WebSocket
func (s *WsService) followContainer(ctx context.Context, ws *websocket.Conn, id string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	apiClient, err := client.New(client.WithHost(s.getContainerSock()))
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}
	defer func(apiClient *client.Client) { _ = apiClient.Close() }(apiClient)

	// 非 TTY 容器日志为多路复用流，需按 TTY 设置决定是否解复用
	inspect, err := apiClient.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}
	tty := inspect.Container.Config != nil && inspect.Container.Config.Tty

	// Tail "0" 表示只推送新增日志，历史由 HTTP /file/tail 反向分页加载
	reader, err := apiClient.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "0",
	})
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel() // 日志流结束(如容器停止)时取消，促使下方读取循环退出
		_ = docker.CopyLogs(&wsBinaryWriter{ctx: ctx, ws: ws}, reader, tty)
	}()

	// 连接结束时取消 ctx 并关闭日志流，待拷贝协程退出后返回，避免泄漏
	defer func() {
		cancel()
		_ = reader.Close()
		<-done
	}()

	for {
		if _, _, rerr := ws.Read(ctx); rerr != nil {
			return
		}
	}
}

func (s *WsService) upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	opts := &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	}

	// debug 模式下不校验 origin，方便 vite 代理调试
	if s.conf.App.Debug {
		opts.InsecureSkipVerify = true
	}

	return websocket.Accept(w, r, opts)
}

// readLoop 阻塞直到客户端关闭连接
func (s *WsService) readLoop(ctx context.Context, c *websocket.Conn) {
	for {
		if _, _, err := c.Read(ctx); err != nil {
			_ = c.CloseNow()
			break
		}
	}
}
