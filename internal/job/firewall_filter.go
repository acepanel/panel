package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

// FirewallFilter 防火墙过滤表的下发与 IP 列表订阅的更新任务
type FirewallFilter struct {
	log          *slog.Logger
	filter       *biz.FirewallFilterUsecase
	subscription *biz.FirewallSubscriptionUsecase
	t            *gotext.Locale
	syncedAt     time.Time
	syncErr      string // 上次下发失败的原因，同一个原因只记一次日志
}

// NewFirewallFilter 构造防火墙过滤表任务
// 过滤表不落盘，重启后要靠面板启动时立即执行的这一次重新下发
func NewFirewallFilter(
	filterUsecase *biz.FirewallFilterUsecase,
	subscriptionUsecase *biz.FirewallSubscriptionUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) Job {
	return Job{
		Spec:      "* * * * *",
		Immediate: true,
		Task: &FirewallFilter{
			log:          log,
			filter:       filterUsecase,
			subscription: subscriptionUsecase,
			t:            t,
		},
	}
}

func (r *FirewallFilter) Run(ctx context.Context) error {
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
func (r *FirewallFilter) sync(ctx context.Context) {
	r.syncedAt = time.Now()

	var syncErr string
	err := r.filter.Sync(ctx)
	if err != nil {
		syncErr = err.Error()
	}
	if err != nil && syncErr != r.syncErr {
		r.log.Warn(r.t.Get("failed to apply firewall filter rules"), slog.String("type", biz.OperationTypeFirewall), slog.Uint64("operator_id", 0), slog.Any("err", err))
	}
	r.syncErr = syncErr
}
