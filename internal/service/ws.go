package service

import (
	"bufio"
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/gofiber/fiber/v2"
	stdssh "golang.org/x/crypto/ssh"

	"github.com/tnborg/panel/internal/biz"
	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/shell"
	"github.com/tnborg/panel/pkg/ssh"
)

type WsService struct {
	t       *gotext.Locale
	conf    *koanf.Koanf
	sshRepo biz.SSHRepo
}

func NewWsService(t *gotext.Locale, conf *koanf.Koanf, ssh biz.SSHRepo) *WsService {
	return &WsService{
		t:       t,
		conf:    conf,
		sshRepo: ssh,
	}
}

func (s *WsService) Session(c fiber.Ctx) error {
	req, err := Bind[request.ID](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}
	info, err := s.sshRepo.Get(req.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "%v", err)
	}

	ws, err := s.upgrade(w, r)
	if err != nil {
		return ErrorSystem(c)
	}
	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	client, err := ssh.NewSSHClient(info.Config)
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()))
	}
	defer func(client *stdssh.Client) {
		_ = client.Close()
	}(client)

	turn, err := ssh.NewTurn(ws, client)
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error()))
	}
	defer func(turn *ssh.Turn) {
		_ = turn.Close()
	}(turn)

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = turn.Handle(ctx)
	}()
	go func() {
		defer wg.Done()
		_ = turn.Wait()
	}()

	wg.Wait()
	cancel()
}

func (s *WsService) Exec(c fiber.Ctx) error {
	ws, err := s.upgrade(w, r)
	if err != nil {
		return ErrorSystem(c)
	}
	defer func(ws *websocket.Conn) {
		_ = ws.Close()
	}(ws)

	// 第一条消息是命令
	_, cmd, err := ws.ReadMessage()
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, s.t.Get("failed to read command: %v", err)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	out, err := shell.ExecfWithPipe(ctx, string(cmd))
	if err != nil {
		_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, s.t.Get("failed to run command: %v", err)))
		cancel()
	}

	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			line := scanner.Text()
			_ = ws.WriteMessage(websocket.TextMessage, []byte(line))
		}
		if err = scanner.Err(); err != nil {
			_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, s.t.Get("failed to read command output: %v", err)))
		}
	}()

	s.readLoop(ws)
	cancel()
}

func (s *WsService) upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upGrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	// debug 模式下不校验 origin，方便 vite 代理调试
	if s.conf.Bool("app.debug") {
		upGrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}

	return upGrader.Upgrade(w, r, nil)
}

// readLoop 阻塞直到客户端关闭连接
func (s *WsService) readLoop(c *websocket.Conn) {
	for {
		if _, _, err := c.NextReader(); err != nil {
			_ = c.Close()
			break
		}
	}
}
