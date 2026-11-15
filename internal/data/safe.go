package data

import (
	"context"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/firewall"
)

type safeRepo struct{}

func NewSafeRepo() biz.SafeRepo {
	return &safeRepo{}
}

func (r *safeRepo) GetPingStatus(ctx context.Context) (bool, error) {
	return firewall.NewFirewall().PingStatus(ctx)
}

func (r *safeRepo) FirewallRunning(ctx context.Context) (bool, error) {
	return firewall.NewFirewall().Status(ctx)
}

func (r *safeRepo) SetPingStatus(ctx context.Context, status bool) error {
	return firewall.NewFirewall().UpdatePingStatus(ctx, status)
}
