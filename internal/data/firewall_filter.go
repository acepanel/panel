package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/firewall/ipset"
)

type firewallFilterRepo struct {
	t            *gotext.Locale
	setting      biz.SettingRepo
	subscription biz.FirewallSubscriptionRepo
	fw           firewall.Firewall
	table        ipset.Table
	cache        firewallSubscriptionCache

	mu      sync.Mutex
	pending atomic.Bool // 有调用在等着同步
	applied string      // 已下发内容的指纹
}

func NewFirewallFilterRepo(
	t *gotext.Locale,
	setting biz.SettingRepo,
	subscription biz.FirewallSubscriptionRepo,
) biz.FirewallFilterRepo {
	return &firewallFilterRepo{
		t:            t,
		setting:      setting,
		subscription: subscription,
		fw:           firewall.NewFirewall(),
	}
}

func (r *firewallFilterRepo) Sync(ctx context.Context) error {
	// 连续多次调用只需要最后一次的结果：等锁期间已有别的调用在自己之后读过状态，就不必再同步
	r.pending.Store(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.pending.Swap(false) {
		return nil
	}

	subs, err := r.subscription.ListEnabled()
	if err != nil {
		return err
	}
	containerPorts, err := r.setting.GetBool(biz.SettingKeyFirewallContainerPorts)
	if err != nil {
		return err
	}
	if len(subs) == 0 && !containerPorts {
		return r.remove(ctx)
	}
	if !r.table.Available() {
		return errors.New(r.t.Get("nftables is not installed, please install it first (e.g., apt install nftables or dnf install nftables)"))
	}

	// 系统防火墙没在运行时过滤表也不生效，和页面上「防火墙已关闭」的状态保持一致
	running, err := r.fw.Status(ctx)
	if err != nil {
		return err
	}
	if !running {
		return r.remove(ctx)
	}

	rules, err := r.fw.ListRule(ctx)
	if err != nil {
		return err
	}
	fingerprint := r.fingerprint(subs, containerPorts, rules)
	if fingerprint == r.applied && r.table.Exists(ctx) {
		return nil
	}

	allow := []ipset.Set{ipset.Of(r.allowedAddresses(rules)...)}
	var block []ipset.Set
	for _, sub := range subs {
		set, err := r.cache.load(sub.ID)
		if err != nil {
			return err
		}
		if sub.Type == biz.FirewallSubscriptionAllow {
			allow = append(allow, set)
		} else {
			block = append(block, set)
		}
	}
	if err = r.table.Apply(ctx, ipset.Ruleset{
		Allow:  ipset.Union(allow...),
		Block:  ipset.Union(block...),
		Mapped: containerPorts,
		Rules:  rules,
	}); err != nil {
		return err
	}
	r.applied = fingerprint

	return nil
}

// remove 撤掉内核里的过滤表
func (r *firewallFilterRepo) remove(ctx context.Context) error {
	r.applied = ""
	if !r.table.Exists(ctx) {
		return nil
	}
	return r.table.Delete(ctx)
}

// allowedAddresses 自定义规则里放行的入站来源地址
// 这些地址交给系统防火墙自己判断，订阅不再拦它们，自定义规则由此优先于订阅
func (r *firewallFilterRepo) allowedAddresses(rules []firewall.FireInfo) []string {
	allowed := lo.FilterMap(rules, func(rule firewall.FireInfo, _ int) (string, bool) {
		return rule.Address, rule.Address != "" && rule.Direction == firewall.DirectionIn && rule.Strategy == firewall.StrategyAccept
	})
	slices.Sort(allowed)
	return slices.Compact(allowed)
}

// fingerprint 决定下发内容的全部输入：启用的订阅及其缓存的修改时间、容器端口管控开关、自定义规则
func (r *firewallFilterRepo) fingerprint(
	subs []*biz.FirewallSubscription,
	containerPorts bool,
	rules []firewall.FireInfo,
) string {
	var sb strings.Builder
	for _, sub := range subs {
		var modTime int64
		if info, err := os.Stat(r.cache.path(sub.ID)); err == nil {
			modTime = info.ModTime().UnixNano()
		}
		_, _ = fmt.Fprintf(&sb, "%d %s %d\n", sub.ID, sub.Type, modTime)
	}
	_, _ = fmt.Fprintf(&sb, "%t %v", containerPorts, rules)
	return sb.String()
}
