package biz

import "context"

type SafeRepo interface {
	GetSSH() (uint, bool, error)
	UpdateSSH(ctx context.Context, port uint, status bool) error
	GetPingStatus() (bool, error)
	UpdatePingStatus(ctx context.Context, status bool) error
}
