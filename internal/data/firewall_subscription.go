package data

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/firewall/ipset"
)

type firewallSubscriptionRepo struct {
	t     *gotext.Locale
	db    *gorm.DB
	fw    firewall.Firewall
	table ipset.Table

	mu      sync.Mutex
	pending atomic.Bool // 有调用在等着同步
	applied string      // 已下发内容的指纹
}

func NewFirewallSubscriptionRepo(db *gorm.DB, t *gotext.Locale) biz.FirewallSubscriptionRepo {
	return &firewallSubscriptionRepo{
		t:  t,
		db: db,
		fw: firewall.NewFirewall(),
	}
}

func (r *firewallSubscriptionRepo) List(page, limit uint) ([]*biz.FirewallSubscription, int64, error) {
	subs := make([]*biz.FirewallSubscription, 0)
	var total int64
	err := r.db.Model(&biz.FirewallSubscription{}).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&subs).Error
	return subs, total, err
}

func (r *firewallSubscriptionRepo) ListEnabled() ([]*biz.FirewallSubscription, error) {
	subs := make([]*biz.FirewallSubscription, 0)
	err := r.db.Where("status = ?", true).Order("id asc").Find(&subs).Error
	return subs, err
}

func (r *firewallSubscriptionRepo) Get(id uint) (*biz.FirewallSubscription, error) {
	sub := new(biz.FirewallSubscription)
	if err := r.db.First(sub, id).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func (r *firewallSubscriptionRepo) Create(sub *biz.FirewallSubscription) error {
	return r.db.Create(sub).Error
}

func (r *firewallSubscriptionRepo) Save(sub *biz.FirewallSubscription) error {
	return r.db.Save(sub).Error
}

func (r *firewallSubscriptionRepo) Delete(sub *biz.FirewallSubscription) error {
	if err := r.db.Delete(sub).Error; err != nil {
		return err
	}
	_ = os.Remove(r.cachePath(sub.ID))
	return nil
}

func (r *firewallSubscriptionRepo) Fetch(ctx context.Context, sub *biz.FirewallSubscription) error {
	// maxListSize 列表体积上限，几十万条的列表也就十来兆
	const maxListSize = 64 << 20

	client := resty.New().SetTimeout(time.Minute)
	defer func(client *resty.Client) { _ = client.Close() }(client)

	resp, err := client.R().SetContext(ctx).SetResponseBodyLimit(maxListSize).Get(sub.URL)
	if err != nil {
		return err
	}
	if resp.IsStatusFailure() {
		return errors.New(r.t.Get("unexpected status code: %d", resp.StatusCode()))
	}

	set, err := ipset.Parse(bytes.NewReader(resp.Bytes()))
	if err != nil {
		return err
	}
	// 一条都解析不出来多半是地址错了或返回了报错页面，不能拿它覆盖掉还能用的旧列表
	if set.Len() == 0 {
		return errors.New(r.t.Get("no valid IP address found in the list"))
	}

	var content bytes.Buffer
	if _, err = set.WriteTo(&content); err != nil {
		return err
	}
	sub.Count = uint(set.Len())

	// 内容没变就不重写，缓存的修改时间不动，Sync 也就不用重新下发
	path := r.cachePath(sub.ID)
	if old, readErr := os.ReadFile(path); readErr == nil && bytes.Equal(old, content.Bytes()) {
		return nil
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, content.Bytes(), 0600)
}

func (r *firewallSubscriptionRepo) Cached(id uint) bool {
	_, err := os.Stat(r.cachePath(id))
	return err == nil
}

func (r *firewallSubscriptionRepo) Sync(ctx context.Context) error {
	// 连续多次调用只需要最后一次的结果：等锁期间已有别的调用在自己之后读过状态，就不必再同步
	r.pending.Store(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.pending.Swap(false) {
		return nil
	}

	subs, err := r.ListEnabled()
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return r.remove(ctx)
	}
	if !r.table.Available() {
		return errors.New(r.t.Get("nftables is not installed, please install it first (e.g., apt install nftables or dnf install nftables)"))
	}

	// 系统防火墙没在运行时订阅也不生效，和页面上「防火墙已关闭」的状态保持一致
	running, err := r.fw.Status(ctx)
	if err != nil {
		return err
	}
	if !running {
		return r.remove(ctx)
	}

	allowed, err := r.allowedAddresses(ctx)
	if err != nil {
		return err
	}
	fingerprint := r.fingerprint(subs, allowed)
	if fingerprint == r.applied && r.table.Exists(ctx) {
		return nil
	}

	allow := []ipset.Set{ipset.Of(allowed...)}
	var block []ipset.Set
	for _, sub := range subs {
		set, err := r.load(sub.ID)
		if err != nil {
			return err
		}
		if sub.Type == biz.FirewallSubscriptionAllow {
			allow = append(allow, set)
		} else {
			block = append(block, set)
		}
	}
	if err = r.table.Apply(ctx, ipset.Union(allow...), ipset.Union(block...)); err != nil {
		return err
	}
	r.applied = fingerprint

	return nil
}

// cachePath 订阅列表的本地缓存，内容是合并后的地址段
func (r *firewallSubscriptionRepo) cachePath(id uint) string {
	return filepath.Join(app.Root, "panel/storage/firewall", fmt.Sprintf("subscription-%d.txt", id))
}

// load 读出缓存的列表，缓存不存在时是空集合
func (r *firewallSubscriptionRepo) load(id uint) (ipset.Set, error) {
	f, err := os.Open(r.cachePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return ipset.Set{}, nil
	}
	if err != nil {
		return ipset.Set{}, err
	}
	defer func() { _ = f.Close() }()

	return ipset.Parse(f)
}

// remove 撤掉内核里的过滤表
func (r *firewallSubscriptionRepo) remove(ctx context.Context) error {
	r.applied = ""
	if !r.table.Exists(ctx) {
		return nil
	}
	return r.table.Delete(ctx)
}

// allowedAddresses 自定义规则里放行的入站来源地址
// 这些地址交给系统防火墙自己判断，订阅不再拦它们，自定义规则由此优先于订阅
func (r *firewallSubscriptionRepo) allowedAddresses(ctx context.Context) ([]string, error) {
	rules, err := r.fw.ListRule(ctx)
	if err != nil {
		return nil, err
	}

	allowed := lo.FilterMap(rules, func(rule firewall.FireInfo, _ int) (string, bool) {
		return rule.Address, rule.Address != "" && rule.Direction == firewall.DirectionIn && rule.Strategy == firewall.StrategyAccept
	})
	slices.Sort(allowed)
	return slices.Compact(allowed), nil
}

// fingerprint 决定下发内容的全部输入：启用的订阅及其缓存的修改时间、放行的地址
func (r *firewallSubscriptionRepo) fingerprint(subs []*biz.FirewallSubscription, allowed []string) string {
	var sb strings.Builder
	for _, sub := range subs {
		var modTime int64
		if info, err := os.Stat(r.cachePath(sub.ID)); err == nil {
			modTime = info.ModTime().UnixNano()
		}
		_, _ = fmt.Fprintf(&sb, "%d %s %d\n", sub.ID, sub.Type, modTime)
	}
	sb.WriteString(strings.Join(allowed, ","))
	return sb.String()
}
