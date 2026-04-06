package service

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/leonelquinteros/gotext"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	stdssh "golang.org/x/crypto/ssh"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/http/request"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/docker"
	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/ssh"
)

type WsService struct {
	t           *gotext.Locale
	conf        *config.Config
	log         *slog.Logger
	sshRepo     biz.SSHRepo
	settingRepo biz.SettingRepo
	certRepo    biz.CertRepo
}

func NewWsService(t *gotext.Locale, conf *config.Config, log *slog.Logger, ssh biz.SSHRepo, settingRepo biz.SettingRepo, certRepo biz.CertRepo) *WsService {
	return &WsService{
		t:           t,
		conf:        conf,
		log:         log,
		sshRepo:     ssh,
		settingRepo: settingRepo,
		certRepo:    certRepo,
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

	// 通过 /bin/sh 启动，自动尝试切换到 bash，不存在则留在 sh
	turn, err := docker.NewTurn(ctx, ws, req.ID, []string{"/bin/sh", "-c", "exec bash 2>/dev/null || exec sh"}, sock)
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
		sock = fmt.Sprintf("unix://%s", sock)
	}
	return sock
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
