package shell

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestRelayInput(t *testing.T) {
	var stdin bytes.Buffer
	var sizes [][2]uint16
	done := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.CloseNow() }()
		_ = RelayInput(r.Context(), ws, &stdin, func(rows, cols uint16) error {
			sizes = append(sizes, [2]uint16{rows, cols})
			return nil
		})
	}))
	defer srv.Close()

	ws, _, err := websocket.Dial(t.Context(), srv.URL, nil) //nolint:bodyclose
	must.NoError(t, err)
	defer func() { _ = ws.CloseNow() }()

	for _, msg := range []string{
		`{"resize":true,"columns":80,"rows":24}`,
		`{"ping":true}`,
		// ping 之后的 resize 不能被当成 ping 吞掉
		`{"resize":true,"columns":120,"rows":40}`,
		`{"resize":true,"columns":0,"rows":40}`,
		// 能反序列化但不是控制消息的输入要原样进终端
		`{"name":"acepanel"}`,
		`null`,
		`ls -l`,
	} {
		must.NoError(t, ws.Write(t.Context(), websocket.MessageText, []byte(msg)))
	}

	_, pong, err := ws.Read(t.Context())
	must.NoError(t, err)
	check.Equal(t, string(pong), `{"pong":true}`)

	must.NoError(t, ws.Close(websocket.StatusNormalClosure, ""))
	<-done

	check.DeepEqual(t, sizes, [][2]uint16{{24, 80}, {40, 120}})
	check.Equal(t, stdin.String(), `{"name":"acepanel"}nullls -l`)
}
