package ssh

import (
	"context"
	"io"

	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/acepanel/panel/v3/pkg/shell"
)

type Turn struct {
	ctx     context.Context
	stdin   io.WriteCloser
	session *ssh.Session
	ws      *websocket.Conn
}

func NewTurn(ctx context.Context, ws *websocket.Conn, client *ssh.Client) (*Turn, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}

	stdin, err := sess.StdinPipe()
	if err != nil {
		return nil, err
	}

	turn := &Turn{ctx: ctx, stdin: stdin, session: sess, ws: ws}
	sess.Stdout = turn
	sess.Stderr = turn

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	if err = sess.RequestPty("xterm", 150, 80, modes); err != nil {
		return nil, err
	}
	if err = sess.Shell(); err != nil {
		return nil, err
	}

	return turn, nil
}

func (t *Turn) Write(p []byte) (n int, err error) {
	if err = t.ws.Write(t.ctx, websocket.MessageBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (t *Turn) Close() {
	// 关闭 stdin 并发 SIGTERM，再关闭 session 触发远程 SIGHUP
	_ = t.stdin.Close()
	_ = t.session.Signal(ssh.SIGTERM)
	_ = t.session.Close()
}

func (t *Turn) Handle(ctx context.Context) error {
	return shell.RelayInput(ctx, t.ws, t.stdin, func(rows, cols uint16) error {
		return t.session.WindowChange(int(rows), int(cols))
	})
}

func (t *Turn) Wait() {
	_ = t.session.Wait()
}
