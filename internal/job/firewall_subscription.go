package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

// FirewallSubscription 防火墙 IP 列表订阅的更新与下发任务
type FirewallSubscription struct {
	log          *slog.Logger
	subscription *biz.FirewallSubscriptionUsecase
	t            *gotext.Locale
	syncedAt     time.Time
	syncErr      string // 上次下发失败的原因，同一个原因只记一次日志
}

// NewFirewallSubscription 构造防火墙 IP 列表订阅任务
// 过滤表不落盘，重启后要靠面板启动时立即执行的这一次重新下发
func NewFirewallSubscription(
	subscriptionUsecase *biz.FirewallSubscriptionUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) Job {
	return Job{
		Spec:      "* * * * *",
		Immediate: true,
		Task: &FirewallSubscription{
			log:          log,
			subscription: subscriptionUsecase,
			t:            t,
		},
	}
}

func (r *FirewallSubscription) Run(ctx context.Context) error {
	// syncInterval 没有订阅更新时的下发间隔，用来跟上面板之外的变化：防火墙启停、命令行改的规则、表被清掉
	const syncInterval = 5 * time.Minute

	// 先按现有缓存下发再去拉取：重启后表是空的，规则的恢复不能等在列表源的响应上
	if time.Since(r.syncedAt) >= syncInterval {
		r.sync(ctx)
	}
	if r.subscription.RefreshDue(ctx) {
		r.sync(ctx)
	}

	return nil
}

// sync 下发一次，同一个失败原因只记一次日志
func (r *FirewallSubscription) sync(ctx context.Context) {
	r.syncedAt = time.Now()

	var syncErr string
	err := r.subscription.Sync(ctx)
	if err != nil {
		syncErr = err.Error()
	}
	if err != nil && syncErr != r.syncErr {
		r.log.Warn(r.t.Get("failed to apply firewall subscriptions"), slog.String("type", biz.OperationTypeFirewall), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}
	r.syncErr = syncErr
}
