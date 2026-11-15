// Package notify 提供面板通知渠道的统一抽象
package notify

import (
	"context"
	"encoding/json"
	"fmt"
)

// 通知渠道类型
const (
	typeSMTP = "smtp"
)

// Message 通知消息
type Message struct {
	Subject string
	Body    string // HTML 正文
}

// Notifier 通知渠道
type Notifier interface {
	Send(ctx context.Context, msg *Message) error
}

// New 按渠道类型构造通知器，config 为渠道的 JSON 配置
func New(typ string, config json.RawMessage) (Notifier, error) {
	switch typ {
	case typeSMTP:
		return newSMTP(config)
	default:
		return nil, fmt.Errorf("unsupported notify channel type: %s", typ)
	}
}
