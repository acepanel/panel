package biz

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/cron"

	"github.com/acepanel/panel/v3/internal/request"
)

const (
	FirewallSubscriptionBlock = "block" // 黑名单：丢弃名单内来源的入站连接
	FirewallSubscriptionAllow = "allow" // 白名单：名单内的来源不受黑名单拦截
)

// FirewallSubscription 防火墙 IP 列表订阅
type FirewallSubscription struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null;default:''" json:"name"`
	URL         string    `gorm:"not null;default:''" json:"url"`
	Type        string    `gorm:"not null;default:''" json:"type"`
	Schedule    string    `gorm:"not null;default:''" json:"schedule"` // 更新周期，cron 表达式
	Status      bool      `gorm:"not null;default:false" json:"status"`
	Count       uint      `gorm:"not null;default:0" json:"count"`  // 合并后的地址段数
	Error       string    `gorm:"not null;default:''" json:"error"` // 最近一次更新失败的原因，成功后清空
	RefreshedAt time.Time `json:"refreshed_at"`                     // 最近一次尝试更新的时间
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type FirewallSubscriptionRepo interface {
	List(page, limit uint) ([]*FirewallSubscription, int64, error)
	ListEnabled() ([]*FirewallSubscription, error)
	Get(id uint) (*FirewallSubscription, error)
	Create(sub *FirewallSubscription) error
	Save(sub *FirewallSubscription) error
	// Delete 连同缓存的列表一起删除
	Delete(sub *FirewallSubscription) error
	// Fetch 拉取列表并缓存到本地，成功时更新 sub.Count，失败时缓存保持原样
	Fetch(ctx context.Context, sub *FirewallSubscription) error
	// Cached 本地是否有缓存的列表
	Cached(id uint) bool
	// Sync 让内核里的过滤表与启用的订阅、防火墙运行状态和自定义规则保持一致
	Sync(ctx context.Context) error
}

// FirewallSubscriptionUsecase 防火墙 IP 列表订阅业务逻辑
type FirewallSubscriptionUsecase struct {
	t      *gotext.Locale
	log    *slog.Logger
	repo   FirewallSubscriptionRepo
	parser *cron.StandardParser
}

func NewFirewallSubscriptionUsecase(
	t *gotext.Locale,
	log *slog.Logger,
	repo FirewallSubscriptionRepo,
) *FirewallSubscriptionUsecase {
	return &FirewallSubscriptionUsecase{t: t, log: log, repo: repo, parser: cron.NewStandardParser()}
}

func (uc *FirewallSubscriptionUsecase) List(page, limit uint) ([]*FirewallSubscription, int64, error) {
	return uc.repo.List(page, limit)
}

// Create 列表拉得下来、规则下发得了才保留订阅
func (uc *FirewallSubscriptionUsecase) Create(
	ctx context.Context,
	req *request.FirewallSubscriptionCreate,
) (*FirewallSubscription, error) {
	if err := uc.checkSchedule(req.Schedule); err != nil {
		return nil, err
	}

	sub := &FirewallSubscription{
		Name:        req.Name,
		URL:         req.URL,
		Type:        req.Type,
		Schedule:    req.Schedule,
		Status:      req.Status,
		RefreshedAt: time.Now(),
	}
	if err := uc.repo.Create(sub); err != nil {
		return nil, err
	}
	if err := uc.repo.Fetch(ctx, sub); err != nil {
		_ = uc.repo.Delete(sub)
		return nil, err
	}
	if err := uc.repo.Save(sub); err != nil {
		return nil, err
	}
	// 下发不了（没装 nftables、容器里没权限）就不留下一个永远不生效的订阅
	if err := uc.apply(ctx); err != nil {
		_ = uc.repo.Delete(sub)
		return nil, err
	}

	uc.log.Info(uc.t.Get("firewall subscription created"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(sub.ID)), slog.String("name", sub.Name), slog.String("url", sub.URL), slog.String("subscription_type", sub.Type), slog.String("schedule", sub.Schedule), slog.Bool("status", sub.Status), slog.Uint64("count", uint64(sub.Count)))

	return sub, nil
}

// Update 地址变了就重新拉取，拉取失败则不保存
func (uc *FirewallSubscriptionUsecase) Update(ctx context.Context, req *request.FirewallSubscriptionUpdate) error {
	if err := uc.checkSchedule(req.Schedule); err != nil {
		return err
	}

	sub, err := uc.repo.Get(req.ID)
	if err != nil {
		return err
	}

	refetch := sub.URL != req.URL
	sub.Name, sub.URL, sub.Type, sub.Schedule, sub.Status = req.Name, req.URL, req.Type, req.Schedule, req.Status
	if refetch {
		if err = uc.repo.Fetch(ctx, sub); err != nil {
			return err
		}
		sub.Error, sub.RefreshedAt = "", time.Now()
	}
	if err = uc.repo.Save(sub); err != nil {
		return err
	}

	uc.log.Info(uc.t.Get("firewall subscription updated"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(sub.ID)), slog.String("name", sub.Name), slog.String("url", sub.URL), slog.String("subscription_type", sub.Type), slog.String("schedule", sub.Schedule), slog.Bool("status", sub.Status))

	return uc.apply(ctx)
}

func (uc *FirewallSubscriptionUsecase) Delete(ctx context.Context, id uint) error {
	sub, err := uc.repo.Get(id)
	if err != nil {
		return err
	}
	if err = uc.repo.Delete(sub); err != nil {
		return err
	}

	uc.log.Warn(uc.t.Get("firewall subscription deleted"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(sub.ID)), slog.String("name", sub.Name), slog.String("url", sub.URL))

	return uc.apply(ctx)
}

// Refresh 立即更新一个订阅
func (uc *FirewallSubscriptionUsecase) Refresh(ctx context.Context, id uint) error {
	sub, err := uc.repo.Get(id)
	if err != nil {
		return err
	}
	if err = uc.refresh(ctx, sub); err != nil {
		return err
	}

	return uc.apply(ctx)
}

// RefreshDue 更新到期的订阅，返回是否有订阅更新成功
func (uc *FirewallSubscriptionUsecase) RefreshDue(ctx context.Context) bool {
	subs, err := uc.repo.ListEnabled()
	if err != nil {
		uc.log.Warn(uc.t.Get("failed to list firewall subscriptions"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", 0), slog.Any("err", err))
		return false
	}

	now := time.Now()
	refreshed := false
	for _, sub := range subs {
		schedule, err := uc.parser.Parse(sub.Schedule)
		if err != nil {
			continue
		}
		// 缓存丢了（比如还原面板备份之后）不等周期，上次拉取失败过的除外，免得每分钟都去撞一个失效的地址
		lost := sub.Error == "" && !uc.repo.Cached(sub.ID)
		if schedule.Next(sub.RefreshedAt).After(now) && !lost {
			continue
		}
		if uc.refresh(ctx, sub) == nil {
			refreshed = true
		}
	}

	return refreshed
}

// Sync 把订阅下发到内核，防火墙状态或自定义规则变化后都要调用
func (uc *FirewallSubscriptionUsecase) Sync(ctx context.Context) error {
	return uc.repo.Sync(ctx)
}

// apply 订阅变化后下发，失败时说明出错的是下发这一步
func (uc *FirewallSubscriptionUsecase) apply(ctx context.Context) error {
	if err := uc.repo.Sync(ctx); err != nil {
		return errors.New(uc.t.Get("failed to apply firewall subscriptions: %v", err))
	}

	return nil
}

// refresh 拉取一次列表并记下结果，失败时旧列表继续生效
func (uc *FirewallSubscriptionUsecase) refresh(ctx context.Context, sub *FirewallSubscription) error {
	err := uc.repo.Fetch(ctx, sub)
	sub.Error, sub.RefreshedAt = "", time.Now()
	if err != nil {
		sub.Error = err.Error()
	}
	if saveErr := uc.repo.Save(sub); saveErr != nil {
		return saveErr
	}

	if err != nil {
		uc.log.Warn(uc.t.Get("failed to refresh firewall subscription"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(sub.ID)), slog.String("name", sub.Name), slog.String("url", sub.URL), slog.Any("err", err))
		return err
	}

	uc.log.Info(uc.t.Get("firewall subscription refreshed"), slog.String("type", OperationTypeFirewall), slog.Uint64("operator_id", OperatorID(ctx)), slog.Uint64("id", uint64(sub.ID)), slog.String("name", sub.Name), slog.String("url", sub.URL), slog.Uint64("count", uint64(sub.Count)))
	return nil
}

// checkSchedule 更新周期要能算出下次时间，校验规则放行的秒级表达式和 @reboot 在这里不适用
func (uc *FirewallSubscriptionUsecase) checkSchedule(schedule string) error {
	if _, err := uc.parser.Parse(schedule); err != nil {
		return errors.New(uc.t.Get("invalid update schedule: %v", err))
	}

	return nil
}
