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

	client, err := ssh.NewSSHClient(info.Config)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}
	defer func(client *stdssh.Client) { _ = client.Close() }(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	turn, err := ssh.NewTurn(ctx, ws, client)
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

func (s *WsService) Exec(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("[Websocket] upgrade exec ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	// 第一条消息是命令
	ctx, cancel := context.WithCancel(context.Background())
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

// ContainerTerminal 容器终端 WebSocket 处理
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

	ctx, cancel := context.WithCancel(context.Background())
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

// ContainerImagePull 镜像拉取 WebSocket 处理
func (s *WsService) ContainerImagePull(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrade(w, r)
	if err != nil {
		s.log.Warn("[Websocket] upgrade image pull ws error", slog.Any("err", err))
		return
	}
	defer func(ws *websocket.Conn) { _ = ws.CloseNow() }(ws)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, message, _ := ws.Read(ctx)
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
