package service

import (
	"bufio"
	"context"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/leonelquinteros/gotext"
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

	// 默认使用 /bin/sh 作为 shell，如果不存在则回退到 sh
	command := []string{"/bin/sh"}

	turn, err := docker.NewTurn(ctx, ws, req.ID, command)
	if err != nil {
		_ = ws.Close(websocket.StatusNormalClosure, err.Error())
		return
	}

	go func() {
		defer turn.Close()
		_ = turn.Handle(ctx)
	}()

	turn.Wait()
}
