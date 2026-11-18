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
				if _, ok := s[block.IP]; !ok {
					s[block.IP] = block
				}
			}
			return nil
		},
		BlockedIPsFunc: func(ips []string) ([]string, error) {
			return lo.Filter(ips, func(ip string, _ int) bool { return s[ip] != nil }), nil
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

// fakeFirewall 只记屏蔽了哪些 IP 和重载了几次
// outbound 不初始化时，下发出站规则会 panic
type fakeFirewall struct {
	firewall.Firewall
	running  bool
	blocked  map[string]bool
	outbound map[string]bool
	reloads  int
	addErr   error
}

func (f *fakeFirewall) Status(context.Context) (bool, error) {
	return f.running, nil
}

func (f *fakeFirewall) RichRulesBatch(_ context.Context, rules []firewall.FireInfo, operation firewall.Operation) error {
	if operation == firewall.OperationAdd && f.addErr != nil {
		return f.addErr
	}
	for _, rule := range rules {
		blocked := f.blocked
		if rule.Direction == firewall.DirectionOut {
			blocked = f.outbound
		}
		if operation == firewall.OperationAdd {
			blocked[rule.Address] = true
		} else {
			delete(blocked, rule.Address)
		}
	}
	f.reloads++
	return nil
}

type scanSettings map[biz.SettingKey]string

func newFirewallScanForTest(store scanBlockStore, settings scanSettings, fw firewall.Firewall) *FirewallScan {
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
	return newFirewallScan(biz.NewScanEventUsecase(t, log, store.repo(), repo), biz.NewSettingUsecase(t, log, repo, nil), fw, t, log)
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
	fw := &fakeFirewall{running: true, blocked: map[string]bool{}}
	settings := autoBlockSettings()

	before := newFirewallScanForTest(store, settings, fw)
	before.ipCounters["198.51.100.7"] = &ipCounter{count: 20, firstSeen: time.Now()}
	before.ipCounters["2001:db8::7"] = &ipCounter{count: 20, firstSeen: time.Now()}
	before.autoBlockForTest(t)
	check.DeepEqual(t, fw.blocked, map[string]bool{"198.51.100.7": true, "2001:db8::7": true})
	check.Equal(t, fw.reloads, 1, check.Msgf("一批 IP 只重载一次"))
	must.Len(t, store, 2)
	for _, block := range store {
		check.Greater(t, block.ExpiresAt, time.Now().Add(59*time.Minute).Unix())
		check.LessOrEqual(t, block.ExpiresAt, time.Now().Add(time.Hour).Unix())
	}

	// 没到期不解封
	settings[biz.SettingKeyScanAware] = "false"
	after := newFirewallScanForTest(store, settings, fw)
	must.NoError(t, after.Run(t.Context()))
	check.Len(t, fw.blocked, 2)

	for _, block := range store {
		block.ExpiresAt = time.Now().Add(-time.Second).Unix()
	}
	must.NoError(t, after.Run(t.Context()))
	check.Len(t, fw.blocked, 0)
	check.Len(t, store, 0)
	check.Equal(t, fw.reloads, 2)
}

// 关掉自动屏蔽后，之前的限时屏蔽到期仍解封，永久屏蔽和没到期的不动
func TestFirewallScanUnblocksWithAutoBlockOff(t *testing.T) {
	store := scanBlockStore{
		"198.51.100.1": {IP: "198.51.100.1", ExpiresAt: time.Now().Add(-time.Minute).Unix()},
		"198.51.100.2": {IP: "198.51.100.2", ExpiresAt: time.Now().Add(time.Hour).Unix()},
		"198.51.100.3": {IP: "198.51.100.3"},
	}
	fw := &fakeFirewall{running: true, blocked: map[string]bool{"198.51.100.1": true, "198.51.100.2": true, "198.51.100.3": true}}
	settings := autoBlockSettings()
	settings[biz.SettingKeyScanAwareAutoBlock] = "false"

	task := newFirewallScanForTest(store, settings, fw)
	// 占位，让任务认为检测器已在运行，测试里不真的挂 eBPF
	task.scanner = new(scan.Scanner)
	task.ipCounters["198.51.100.9"] = &ipCounter{count: 20, firstSeen: time.Now()}
	must.NoError(t, task.Run(t.Context()))

	check.DeepEqual(t, fw.blocked, map[string]bool{"198.51.100.2": true, "198.51.100.3": true})
	check.Len(t, store, 2)
	check.Len(t, task.ipCounters, 0)
}

// 防火墙没在运行时规则删不掉，记录要留到它恢复
func TestFirewallScanKeepsBlocksWhileFirewallDown(t *testing.T) {
	store := scanBlockStore{"198.51.100.1": {IP: "198.51.100.1", ExpiresAt: time.Now().Add(-time.Minute).Unix()}}
	fw := &fakeFirewall{blocked: map[string]bool{"198.51.100.1": true}}
	task := newFirewallScanForTest(store, scanSettings{}, fw)

	must.NoError(t, task.Run(t.Context()))
	check.Len(t, store, 1)

	fw.running = true
	must.NoError(t, task.Run(t.Context()))
	check.Len(t, store, 0)
	check.Len(t, fw.blocked, 0)
}

func TestFirewallScanAutoBlock(t *testing.T) {
	store := scanBlockStore{"198.51.100.4": {IP: "198.51.100.4"}}
	fw := &fakeFirewall{running: true, blocked: map[string]bool{"198.51.100.4": true}}
	task := newFirewallScanForTest(store, autoBlockSettings(), fw)

	now := time.Now()
	task.ipCounters = map[string]*ipCounter{
		"198.51.100.1": {count: 10, firstSeen: now},
		"198.51.100.2": {count: 9, firstSeen: now},                         // 没到阈值
		"198.51.100.3": {count: 50, firstSeen: now.Add(-10 * time.Minute)}, // 窗口已过期
		"198.51.100.4": {count: 50, firstSeen: now},                        // 已屏蔽
		"192.0.2.8":    {count: 50, firstSeen: now},                        // 白名单
	}
	task.autoBlockForTest(t)

	check.DeepEqual(t, fw.blocked, map[string]bool{"198.51.100.1": true, "198.51.100.4": true})
	check.Equal(t, fw.reloads, 1)
	check.Len(t, store, 2)
	check.Equal(t, store["198.51.100.4"].ExpiresAt, int64(0), check.Msgf("已有的永久屏蔽不被改成限时"))
	check.DeepEqual(t, lo.Keys(task.ipCounters), []string{"198.51.100.2"})

	// 下发失败时撤回，不留下没有对应规则的记录
	fw.addErr = errors.New("firewalld is down")
	task.ipCounters["198.51.100.5"] = &ipCounter{count: 10, firstSeen: now}
	task.autoBlockForTest(t)
	check.MapNotContainsKey(t, store, "198.51.100.5")
	check.False(t, fw.blocked["198.51.100.5"])
}

// 开了出站屏蔽后入站出站一起下发；解封按屏蔽当时的记录删，不看现在的开关
func TestFirewallScanBlockOutbound(t *testing.T) {
	store := scanBlockStore{}
	fw := &fakeFirewall{running: true, blocked: map[string]bool{}, outbound: map[string]bool{}}
	settings := autoBlockSettings()
	settings[biz.SettingKeyScanAwareBlockOutbound] = "true"
	task := newFirewallScanForTest(store, settings, fw)

	task.ipCounters["198.51.100.1"] = &ipCounter{count: 20, firstSeen: time.Now()}
	task.autoBlockForTest(t)
	check.DeepEqual(t, fw.blocked, map[string]bool{"198.51.100.1": true})
	check.DeepEqual(t, fw.outbound, map[string]bool{"198.51.100.1": true})
	check.Equal(t, fw.reloads, 1, check.Msgf("入站出站同一批下发"))
	must.MapContainsKey(t, store, "198.51.100.1")
	check.True(t, store["198.51.100.1"].Outbound)

	// 关掉开关后新屏蔽的只有入站
	settings[biz.SettingKeyScanAwareBlockOutbound] = "false"
	task.ipCounters["198.51.100.2"] = &ipCounter{count: 20, firstSeen: time.Now()}
	task.autoBlockForTest(t)
	check.DeepEqual(t, fw.blocked, map[string]bool{"198.51.100.1": true, "198.51.100.2": true})
	check.DeepEqual(t, fw.outbound, map[string]bool{"198.51.100.1": true})
	must.MapContainsKey(t, store, "198.51.100.2")
	check.False(t, store["198.51.100.2"].Outbound)

	// 没屏蔽过出站的 IP 解封时不碰出站规则
	fw.outbound["198.51.100.2"] = true
	for _, block := range store {
		block.ExpiresAt = time.Now().Add(-time.Second).Unix()
	}
	must.NoError(t, task.Run(t.Context()))
	check.Len(t, fw.blocked, 0)
	check.DeepEqual(t, fw.outbound, map[string]bool{"198.51.100.2": true})
	check.Len(t, store, 0)
}
