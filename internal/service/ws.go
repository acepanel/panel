package service

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/leonelquinteros/gotext"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
	stdssh "golang.org/x/crypto/ssh"

	"github.com/acepanel/panel/internal/biz"
	"github.com/acepanel/panel/internal/http/request"
	"github.com/acepanel/panel/pkg/config"
	"github.com/acepanel/panel/pkg/docker"
	"github.com/acepanel/panel/pkg/shell"
	"github.com/acepanel/panel/pkg/ssh"
)

type WsService struct {
	t       *gotext.Locale
	conf    *config.Config
	log     *slog.Logger
	sshRepo biz.SSHRepo
}

func NewWsService(t *gotext.Locale, conf *config.Config, log *slog.Logger, ssh biz.SSHRepo) *WsService {
	return &WsService{
		t:       t,
		conf:    conf,
		log:     log,
		sshRepo: ssh,
	}
}

func (s *WsService) Exec(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("[Websocket] upgrade exec ws error", slog.Any("err", err))
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
		s.log.Warn("[Websocket] upgrade pty ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 读取第一条消息获取要执行的命令
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

	// 使用 PTY 执行命令
	ptyResult, err := shell.ExecWithPTY(ctx, command)
	if err != nil {
		_ = ws.Write(ctx, websocket.MessageBinary, []byte("\r\n"+s.t.Get("Failed to start command: %v", err)+"\r\n"))
		_ = ws.Close(websocket.StatusNormalClosure, "")
		return
	}
	defer func() { _ = ptyResult.Close() }()

	// 读取 WebSocket 输入并转发到 PTY
	go func() {
		for {
			msgType, data, err := ws.Read(ctx)
			if err != nil {
				// 通常是客户端关闭连接，取消运行
				cancel()
				return
			}
			if len(data) == 0 {
				continue
			}

			// 检查是否是 resize 消息（JSON 格式，以 { 开头）
			if msgType == websocket.MessageText && data[0] == '{' {
				var resizeMsg struct {
					Type string `json:"type"`
					Rows uint16 `json:"rows"`
					Cols uint16 `json:"cols"`
				}
				if err := json.Unmarshal(data, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
					_ = ptyResult.Resize(resizeMsg.Rows, resizeMsg.Cols)
					continue
				}
			}

			// 普通用户输入，写入 PTY
			_, _ = ptyResult.Write(data)
		}
	}()

	// 读取 PTY 输出并发送到 WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptyResult.Read(buf)
			if n > 0 {
				if writeErr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); writeErr != nil {
					s.log.Warn("[Websocket] write pty output error", slog.Any("err", writeErr))
					cancel()
					return
				}
			}
			if err != nil {
				if shell.IsPTYError(err) != nil {
					s.log.Debug("[Websocket] pty read error", slog.Any("err", err))
				}
				return
			}
		}
	}()

	// 等待命令完成
	if err = ptyResult.Wait(); err != nil {
		// 如果是因为被杀死，不输出错误
		if ctx.Err() == nil {
			_ = ws.Write(ctx, websocket.MessageBinary, []byte("\r\n"+s.t.Get("Command failed: %v", err)+"\r\n"))
		}
	}

	_ = ws.Close(websocket.StatusNormalClosure, "")
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
		s.log.Warn("[Websocket] upgrade session ws error", slog.Any("err", err))
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
		s.log.Warn("[Websocket] upgrade container terminal ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// 默认使用 bash 作为 shell，如果不存在则回退到 sh
	turn, err := docker.NewTurn(ctx, ws, req.ID, []string{"/bin/bash"})
	if err != nil {
		turn, err = docker.NewTurn(ctx, ws, req.ID, []string{"/bin/sh"})
		if err != nil {
			_ = ws.Close(websocket.StatusNormalClosure, s.t.Get("failed to start container terminal: %v", err))
			return
		}
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
		s.log.Warn("[Websocket] upgrade image pull ws error", slog.Any("err", err))
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
	apiClient, err := client.New(client.WithHost("unix:///var/run/docker.sock"))
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
			s.log.Warn("[Websocket] image pull error", slog.Any("err", err))
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
			s.log.Warn("[Websocket] write image pull progress error", slog.Any("err", err))
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
