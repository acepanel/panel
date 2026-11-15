package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/coder/websocket"
)

type controlMessage struct {
	Ping    bool `json:"ping"`
	Resize  bool `json:"resize"`
	Columns uint `json:"columns"`
	Rows    uint `json:"rows"`
}

// RelayInput 把 WebSocket 消息写入终端输入，ping 与 resize 控制消息就地处理
func RelayInput(ctx context.Context, ws *websocket.Conn, stdin io.Writer, resize func(rows, cols uint16) error) error {
	for {
		_, data, err := ws.Read(ctx)
		if err != nil {
			// 通常是客户端关闭连接
			return fmt.Errorf("failed to read ws message: %w", err)
		}

		// Unmarshal 不清零 JSON 里没出现的字段，复用变量会让上一条消息的标志残留
		var msg controlMessage
		if json.Unmarshal(data, &msg) == nil && (msg.Ping || msg.Resize) {
			if msg.Ping {
				_ = ws.Write(ctx, websocket.MessageText, []byte(`{"pong":true}`))
			}
			if msg.Resize && msg.Columns > 0 && msg.Rows > 0 && msg.Columns <= math.MaxUint16 && msg.Rows <= math.MaxUint16 {
				if err = resize(uint16(msg.Rows), uint16(msg.Columns)); err != nil {
					return fmt.Errorf("failed to resize terminal: %w", err)
				}
			}
			continue
		}

		if _, err = stdin.Write(data); err != nil {
			return fmt.Errorf("failed to write to terminal stdin: %w", err)
		}
	}
}
