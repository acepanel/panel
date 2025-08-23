package service

import (
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/biz"
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

// WebSocket methods temporarily disabled during Fiber migration
// TODO: Implement WebSocket support for Fiber v3