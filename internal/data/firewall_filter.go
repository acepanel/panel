package data

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
	scan         biz.ScanEventRepo
	fw           firewall.Firewall
	table        ipset.Table
	cache        firewallSubscriptionCache

	mu      sync.Mutex
	applied string // 已下发内容的指纹
}

func NewFirewallFilterRepo(
	t *gotext.Locale,
	setting biz.SettingRepo,
	subscription biz.FirewallSubscriptionRepo,
	scan biz.ScanEventRepo,
) biz.FirewallFilterRepo {
	return &firewallFilterRepo{
		t:            t,
		setting:      setting,
		subscription: subscription,
		scan:         scan,
		fw:           firewall.NewFirewall(),
	}
}

func (r *firewallFilterRepo) Available() bool {
	return r.table.Available()
}

func (r *firewallFilterRepo) Sync(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 客户端断连也要下发完，否则系统防火墙的规则改了而过滤表没跟上
	ctx = context.WithoutCancel(ctx)

	subs, err := r.subscription.ListEnabled()
	if err != nil {
		return err
	}
	blocks, err := r.scan.ActiveBlocks(time.Now())
	if err != nil {
		return err
	}
	containerPorts, err := r.setting.GetBool(biz.SettingKeyFirewallContainerPorts)
	if err != nil {
		return err
	}
	if len(subs) == 0 && len(blocks) == 0 && !containerPorts {
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
	fingerprint := r.fingerprint(subs, blocks, containerPorts, rules)
	if fingerprint == r.applied && r.table.Exists(ctx) {
		return nil
	}

	allPortAllows, scopedAllows := r.allowedRules(rules)
	allow := []ipset.Set{ipset.Of(allPortAllows...)}
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

	blockIn := lo.Map(blocks, func(block *biz.ScanBlock, _ int) string { return block.IP })
	blockOut := lo.FilterMap(blocks, func(block *biz.ScanBlock, _ int) (string, bool) { return block.IP, block.Outbound })

	if err = r.table.Apply(ctx, ipset.Ruleset{
		Allow:      ipset.Union(allow...),
		AllowRules: scopedAllows,
		Block:      ipset.Union(block...),
		BlockIn:    ipset.Of(blockIn...),
		BlockOut:   ipset.Of(blockOut...),
		Mapped:     containerPorts,
		Rules:      rules,
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

// allowedRules 拆分自定义放行入站规则：全端口全协议的进全局白名单集合，带端口或协议限制的按精确规则豁免
func (r *firewallFilterRepo) allowedRules(rules []firewall.FireInfo) ([]string, []firewall.FireInfo) {
	var allPorts []string
	var scoped []firewall.FireInfo
	for _, rule := range rules {
		if rule.Direction != firewall.DirectionIn || rule.Strategy != firewall.StrategyAccept {
			continue
		}
		// 不限来源的放行（不带地址或写成 0.0.0.0/0、::/0）不算豁免，否则黑名单对这些端口就没用了
		if rule.Address == "" || strings.HasSuffix(rule.Address, "/0") {
			continue
		}
		if rule.PortStart == 1 && rule.PortEnd == 65535 && rule.Protocol == firewall.ProtocolTCPUDP {
			allPorts = append(allPorts, rule.Address)
			continue
		}
		scoped = append(scoped, rule)
	}
	return allPorts, scoped
}

// fingerprint 决定下发内容的全部输入：启用的订阅缓存、扫描自动屏蔽、容器端口管控开关、自定义规则
func (r *firewallFilterRepo) fingerprint(
	subs []*biz.FirewallSubscription,
	blocks []*biz.ScanBlock,
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
	for _, block := range blocks {
		_, _ = fmt.Fprintf(&sb, "%s %t\n", block.IP, block.Outbound)
	}
	_, _ = fmt.Fprintf(&sb, "%t %v", containerPorts, rules)
	return sb.String()
}
