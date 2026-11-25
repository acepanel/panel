package job

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/samber/lo"
	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/internal/biz"
	mocks "github.com/acepanel/panel/v3/internal/mocks/biz"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/firewall/scan"
)

// scanBlockStore 顶替扫描库里的屏蔽记录，不随任务实例消失
type scanBlockStore map[string]*biz.ScanBlock

func (s scanBlockStore) repo() biz.ScanEventRepo {
	return &mocks.ScanEventRepo{
		AddBlocksFunc: func(blocks []*biz.ScanBlock) error {
			for _, block := range blocks {
				s[block.IP] = block
			}
			return nil
		},
		ActiveBlocksFunc: func(now time.Time) ([]*biz.ScanBlock, error) {
			return lo.Filter(lo.Values(s), func(block *biz.ScanBlock, _ int) bool {
				return block.ExpiresAt == 0 || block.ExpiresAt > now.Unix()
			}), nil
		},
		ExpiredBlocksFunc: func(now time.Time) ([]*biz.ScanBlock, error) {
			return lo.Filter(lo.Values(s), func(block *biz.ScanBlock, _ int) bool {
				return block.ExpiresAt > 0 && block.ExpiresAt <= now.Unix()
			}), nil
		},
		DeleteBlocksFunc: func(ips []string) error {
			for _, ip := range ips {
				delete(s, ip)
			}
			return nil
		},
		ClearBeforeFunc: func(string) error { return nil },
	}
}

// fakeFirewall 只提供系统防火墙运行状态和手动规则，自动屏蔽不再调用 RichRulesBatch
type fakeFirewall struct {
	firewall.Firewall
	running bool
	manual  map[string]bool
}

func (f *fakeFirewall) Status(context.Context) (bool, error) {
	return f.running, nil
}

func (f *fakeFirewall) ListRule(context.Context) ([]firewall.FireInfo, error) {
	return lo.MapToSlice(f.manual, func(address string, _ bool) firewall.FireInfo {
		return firewall.FireInfo{Type: firewall.TypeRich, Address: address, PortStart: 1, PortEnd: 65535, Protocol: firewall.ProtocolTCPUDP, Strategy: firewall.StrategyDrop, Direction: firewall.DirectionIn}
	}), nil
}

// fakeFilter 模拟 nftables 过滤表下发
type fakeFilter struct {
	syncs   int
	syncErr error
}

func (f *fakeFilter) repo() biz.FirewallFilterRepo {
	return &mocks.FirewallFilterRepo{
		AvailableFunc: func() bool { return true },
		SyncFunc: func(context.Context) error {
			if f.syncErr != nil {
				return f.syncErr
			}
			f.syncs++
			return nil
		},
	}
}

type scanSettings map[biz.SettingKey]string

func newFirewallScanForTest(store scanBlockStore, settings scanSettings, fw firewall.Firewall, filter *fakeFilter) *FirewallScan {
	repo := &mocks.SettingRepo{
		GetFunc: func(key biz.SettingKey, _ ...string) (string, error) {
			return settings[key], nil
		},
		GetBoolFunc: func(key biz.SettingKey, _ ...bool) (bool, error) {
			return cast.ToBool(settings[key]), nil
		},
		GetIntFunc: func(key biz.SettingKey, _ ...int) (int, error) {
			return cast.ToInt(settings[key]), nil
		},
		GetSliceFunc: func(key biz.SettingKey, _ ...[]string) ([]string, error) {
			return strings.Fields(settings[key]), nil
		},
	}
	t := gotext.NewLocale("", "en")
	log := slog.New(slog.DiscardHandler)
	filterRepo := filter.repo()
	return newFirewallScan(
		biz.NewScanEventUsecase(t, log, store.repo(), repo, filterRepo),
		biz.NewSettingUsecase(t, log, repo, nil),
		biz.NewFirewallFilterUsecase(t, log, filterRepo, repo),
		fw,
		t,
		log,
	)
}

func autoBlockSettings() scanSettings {
	return scanSettings{
		biz.SettingKeyScanAware:               "true",
		biz.SettingKeyScanAwareAutoBlock:      "true",
		biz.SettingKeyScanAwareBlockThreshold: "10",
		biz.SettingKeyScanAwareBlockWindow:    "5",
		biz.SettingKeyScanAwareBlockDuration:  "1",
		biz.SettingKeyScanAwareWhitelist:      "192.0.2.0/24",
	}
}

func (r *FirewallScan) autoBlockForTest(t *testing.T) {
	t.Helper()
	setting, err := r.scanRepo.GetSetting()
	must.NoError(t, err)
	r.autoBlock(t.Context(), setting)
}

// 屏蔽记录在库里，面板重启丢了内存状态后到期照样解封；此时扫描感知已关也不影响
func TestFirewallScanUnblocksAfterRestart(t *testing.T) {
	store := scanBlockStore{}
	fw := &fakeFirewall{running: true}
	filter := &fakeFilter{}
	settings := autoBlockSettings()

	before := newFirewallScanForTest(store, settings, fw, filter)
	before.ipCounters["198.51.100.7"] = &ipCounter{count: 20, firstSeen: time.Now()}
	before.ipCounters["2001:db8::7"] = &ipCounter{count: 20, firstSeen: time.Now()}
	before.autoBlockForTest(t)
	check.Equal(t, filter.syncs, 1, check.Msgf("一批 IP 只同步一次 nftables"))
	must.Len(t, store, 2)
	for _, block := range store {
		check.Greater(t, block.ExpiresAt, time.Now().Add(59*time.Minute).Unix())
		check.LessOrEqual(t, block.ExpiresAt, time.Now().Add(time.Hour).Unix())
	}

	// 没到期不解封
	settings[biz.SettingKeyScanAware] = "false"
	after := newFirewallScanForTest(store, settings, fw, filter)
	must.NoError(t, after.Run(t.Context()))
	check.Len(t, store, 2)
	check.Equal(t, filter.syncs, 1)

	for _, block := range store {
		block.ExpiresAt = time.Now().Add(-time.Second).Unix()
	}
	must.NoError(t, after.Run(t.Context()))
	check.Len(t, store, 0)
	check.Equal(t, filter.syncs, 2)
}

// 关掉自动屏蔽后，之前的限时屏蔽到期仍解封，永久屏蔽和没到期的不动
func TestFirewallScanUnblocksWithAutoBlockOff(t *testing.T) {
	store := scanBlockStore{
		"198.51.100.1": {IP: "198.51.100.1", ExpiresAt: time.Now().Add(-time.Minute).Unix()},
		"198.51.100.2": {IP: "198.51.100.2", ExpiresAt: time.Now().Add(time.Hour).Unix()},
		"198.51.100.3": {IP: "198.51.100.3"},
	}
	fw := &fakeFirewall{running: true}
	filter := &fakeFilter{}
	settings := autoBlockSettings()
	settings[biz.SettingKeyScanAwareAutoBlock] = "false"

	task := newFirewallScanForTest(store, settings, fw, filter)
	// 占位，让任务认为检测器已在运行，测试里不真的挂 eBPF
	task.scanner = new(scan.Scanner)
	task.ipCounters["198.51.100.9"] = &ipCounter{count: 20, firstSeen: time.Now()}
	must.NoError(t, task.Run(t.Context()))

	check.MapNotContainsKey(t, store, "198.51.100.1")
	check.MapContainsKey(t, store, "198.51.100.2")
	check.MapContainsKey(t, store, "198.51.100.3")
	check.Equal(t, filter.syncs, 1)
	check.Len(t, task.ipCounters, 0)
}

// 防火墙未运行时到期记录直接清理，避免防火墙重新开启后又把过期记录下发回内核
func TestFirewallScanUnblocksWhileFirewallDown(t *testing.T) {
	store := scanBlockStore{"198.51.100.1": {IP: "198.51.100.1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}}
	fw := &fakeFirewall{running: false}
	filter := &fakeFilter{}
	task := newFirewallScanForTest(store, scanSettings{}, fw, filter)

	must.NoError(t, task.Run(t.Context()))
	check.Len(t, store, 0)
	check.Equal(t, filter.syncs, 1)
}

func TestFirewallScanAutoBlock(t *testing.T) {
	store := scanBlockStore{"198.51.100.4": {IP: "198.51.100.4"}}
	fw := &fakeFirewall{running: true, manual: map[string]bool{"203.0.113.0/24": true}}
	filter := &fakeFilter{}
	task := newFirewallScanForTest(store, autoBlockSettings(), fw, filter)

	now := time.Now()
	task.ipCounters = map[string]*ipCounter{
		"198.51.100.1": {count: 10, firstSeen: now},
		"198.51.100.2": {count: 9, firstSeen: now},                         // 没到阈值
		"198.51.100.3": {count: 50, firstSeen: now.Add(-10 * time.Minute)}, // 窗口已过期
		"198.51.100.4": {count: 50, firstSeen: now},                        // 已在自动屏蔽中
		"203.0.113.9":  {count: 50, firstSeen: now},                        // 落在手动屏蔽的网段里
		"192.0.2.8":    {count: 50, firstSeen: now},                        // 白名单
	}
	task.autoBlockForTest(t)

	check.MapContainsKey(t, store, "198.51.100.1")
	check.MapContainsKey(t, store, "198.51.100.4")
	check.Equal(t, filter.syncs, 1)
	check.Len(t, store, 2)
	check.Equal(t, store["198.51.100.4"].ExpiresAt, int64(0), check.Msgf("已有的永久屏蔽不被改成限时"))
	check.DeepEqual(t, lo.Keys(task.ipCounters), []string{"198.51.100.2"})

	// nftables 下发失败时撤回，不留下没有生效的记录
	filter.syncErr = errors.New("nftables is not installed")
	task.ipCounters["198.51.100.5"] = &ipCounter{count: 10, firstSeen: now}
	task.autoBlockForTest(t)
	check.MapNotContainsKey(t, store, "198.51.100.5")
}

// 开了出站屏蔽后记录 Outbound 标志，关掉开关后新屏蔽的不带 Outbound
func TestFirewallScanBlockOutbound(t *testing.T) {
	store := scanBlockStore{}
	fw := &fakeFirewall{running: true}
	filter := &fakeFilter{}
	settings := autoBlockSettings()
	settings[biz.SettingKeyScanAwareBlockOutbound] = "true"
	task := newFirewallScanForTest(store, settings, fw, filter)

	task.ipCounters["198.51.100.1"] = &ipCounter{count: 20, firstSeen: time.Now()}
	task.autoBlockForTest(t)
	check.Equal(t, filter.syncs, 1)
	must.MapContainsKey(t, store, "198.51.100.1")
	check.True(t, store["198.51.100.1"].Outbound)

	settings[biz.SettingKeyScanAwareBlockOutbound] = "false"
	task.ipCounters["198.51.100.2"] = &ipCounter{count: 20, firstSeen: time.Now()}
	task.autoBlockForTest(t)
	check.Equal(t, filter.syncs, 2)
	must.MapContainsKey(t, store, "198.51.100.2")
	check.False(t, store["198.51.100.2"].Outbound)

	for _, block := range store {
		block.ExpiresAt = time.Now().Add(-time.Second).Unix()
	}
	must.NoError(t, task.Run(t.Context()))
	check.Len(t, store, 0)
	check.Equal(t, filter.syncs, 3)
}
