package job

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/firewall/scan"
	"github.com/acepanel/panel/v3/pkg/geoip"
)

// ipCounter 单个 IP 的扫描计数器
type ipCounter struct {
	count     uint
	firstSeen time.Time
}

// FirewallScan 防火墙扫描感知任务
type FirewallScan struct {
	log          *slog.Logger
	setting      *biz.SettingUsecase
	scanRepo     *biz.ScanEventUsecase
	t            *gotext.Locale
	fw           firewall.Firewall
	scanner      *scan.Scanner
	ifaces       []string // scanner 启动时的网卡设置
	geoIP        *geoip.GeoIP
	geoIPPath    string
	geoIPModTime time.Time
	buffer       map[string]*biz.ScanEvent // key: "ip:port:proto:date"
	ipCounters   map[string]*ipCounter     // per-IP 扫描计数
	cleanedAt    time.Time
	mu           sync.Mutex

	unsupportedLogged bool // 内核不支持仅告警一次
}

// NewFirewallScan 构造防火墙扫描感知任务
func NewFirewallScan(
	scanEventUsecase *biz.ScanEventUsecase,
	settingUsecase *biz.SettingUsecase,
	t *gotext.Locale,
	log *slog.Logger,
) Job {
	return Job{
		Spec: "*/2 * * * *",
		Task: newFirewallScan(scanEventUsecase, settingUsecase, firewall.NewFirewall(), t, log),
	}
}

func newFirewallScan(
	scanEventUsecase *biz.ScanEventUsecase,
	settingUsecase *biz.SettingUsecase,
	fw firewall.Firewall,
	t *gotext.Locale,
	log *slog.Logger,
) *FirewallScan {
	return &FirewallScan{
		log:        log,
		setting:    settingUsecase,
		scanRepo:   scanEventUsecase,
		t:          t,
		fw:         fw,
		buffer:     make(map[string]*biz.ScanEvent),
		ipCounters: make(map[string]*ipCounter),
	}
}

func (r *FirewallScan) Run(ctx context.Context) error {
	// 到期解封不看开关：关掉扫描感知或自动屏蔽后，已有的限时屏蔽照常到期
	r.unblockExpired(ctx)

	setting, err := r.scanRepo.GetSetting()
	if err != nil {
		r.log.Warn(r.t.Get("failed to get scan setting"), slog.Any("err", err))
		return nil
	}
	if !setting.Enabled {
		r.stopScanner()
		return nil
	}

	r.ensureScanner(setting.Interfaces)
	r.geoIP, r.geoIPPath, r.geoIPModTime = refreshGeoIP(r.setting, r.geoIP, r.geoIPPath, r.geoIPModTime, r.t, r.log)
	r.flush()
	r.autoBlock(ctx, setting)
	r.cleanup()
	return nil
}

// unblockExpired 解封到期的限时屏蔽
func (r *FirewallScan) unblockExpired(ctx context.Context) {
	blocks, err := r.scanRepo.ExpiredBlocks(time.Now())
	if err != nil {
		r.log.Warn(r.t.Get("failed to read scan blocks"), slog.Any("err", err))
		return
	}
	if len(blocks) == 0 {
		return
	}

	running, err := r.fw.Status(ctx)
	if err != nil {
		r.log.Warn(r.t.Get("failed to get firewall status"), slog.Any("err", err))
		return
	}
	// 防火墙没在运行时规则删不掉，记录留着等它恢复后再解封
	if !running {
		return
	}

	ips := lo.Map(blocks, func(block *biz.ScanBlock, _ int) string { return block.IP })
	rules := lo.FlatMap(blocks, func(block *biz.ScanBlock, _ int) []firewall.FireInfo {
		return r.blockRules(block.IP, block.Outbound)
	})
	if err = r.fw.RichRulesBatch(ctx, rules, firewall.OperationRemove); err != nil {
		r.log.Warn(r.t.Get("failed to unblock IP"), slog.Int("count", len(ips)), slog.Any("err", err))
		return
	}
	if err = r.scanRepo.DeleteBlocks(ips); err != nil {
		r.log.Warn(r.t.Get("failed to save scan blocks"), slog.Any("err", err))
		return
	}
	for _, block := range blocks {
		r.log.Info(r.t.Get("auto unblocked IP (expired)"), slog.String("ip", block.IP), slog.Time("blocked_at", block.CreatedAt))
	}
}

// ensureScanner 确保 scanner 挂在设置的网卡上
func (r *FirewallScan) ensureScanner(ifaces []string) {
	if r.scanner != nil {
		if slices.Equal(ifaces, r.ifaces) {
			return
		}
		// 监听网卡改了，重建
		r.stopScanner()
	}

	if !scan.Supported() {
		if !r.unsupportedLogged {
			r.log.Warn(r.t.Get("eBPF scan detector unavailable on this kernel (TCX attach requires 6.6+)"))
			r.unsupportedLogged = true
		}
		return
	}

	scanner, err := scan.New(ifaces, r.t, r.log)
	if err != nil {
		r.log.Warn(r.t.Get("failed to start eBPF scan detector"), slog.Any("err", err))
		return
	}

	r.scanner, r.ifaces = scanner, ifaces

	// 启动后台事件聚合
	go r.aggregate(scanner.Events())
}

// stopScanner 停止 scanner
func (r *FirewallScan) stopScanner() {
	if r.scanner == nil {
		return
	}
	_ = r.scanner.Close()
	r.scanner = nil

	r.resetCounters()
}

// resetCounters 清空扫描计数，重新分配以释放内存
func (r *FirewallScan) resetCounters() {
	r.mu.Lock()
	if len(r.ipCounters) > 0 {
		r.ipCounters = make(map[string]*ipCounter)
	}
	r.mu.Unlock()
}

// aggregate 持续读取 eBPF 事件并聚合到内存缓冲
func (r *FirewallScan) aggregate(events <-chan scan.Event) {
	// maxScanEntries 聚合缓冲/计数器条目上限,防伪造源 IP 洪水耗尽内存
	const maxScanEntries = 100000

	for evt := range events {
		date := evt.Timestamp.Format(time.DateOnly)
		key := fmt.Sprintf("%s:%d:%s:%s", evt.SourceIP, evt.Port, evt.Protocol, date)

		r.mu.Lock()
		if existing, ok := r.buffer[key]; ok {
			existing.Count++
			existing.LastSeen = evt.Timestamp
		} else if len(r.buffer) < maxScanEntries {
			r.buffer[key] = &biz.ScanEvent{
				SourceIP:  evt.SourceIP,
				Port:      uint(evt.Port),
				Protocol:  evt.Protocol,
				Date:      date,
				Count:     1,
				FirstSeen: evt.Timestamp,
				LastSeen:  evt.Timestamp,
			}
		}

		// per-IP 计数递增
		if counter, ok := r.ipCounters[evt.SourceIP]; ok {
			counter.count++
		} else if len(r.ipCounters) < maxScanEntries {
			r.ipCounters[evt.SourceIP] = &ipCounter{
				count:     1,
				firstSeen: evt.Timestamp,
			}
		}
		r.mu.Unlock()
	}
}

// flush 将内存缓冲写入数据库
func (r *FirewallScan) flush() {
	// healthKeyScanDB 扫描事件辅助数据库健康问题上报 key
	const healthKeyScanDB = "database:scan"

	r.mu.Lock()
	if len(r.buffer) == 0 {
		r.mu.Unlock()
		return
	}

	events := lo.Values(r.buffer)
	r.buffer = make(map[string]*biz.ScanEvent)
	r.mu.Unlock()

	// 解析 IP 地理位置
	if r.geoIP != nil {
		for _, evt := range events {
			geo := r.geoIP.Lookup(evt.SourceIP)
			evt.Country = geo.CountryCode
			evt.Region = geo.Region
			evt.City = geo.City
			evt.ISP = geo.ISP
		}
	}

	if err := r.scanRepo.Upsert(events); err != nil {
		r.log.Warn(r.t.Get("failed to upsert scan events"), slog.Any("err", err))
		app.Health.Report(healthKeyScanDB, app.HealthLevelError, err.Error())
		return
	}
	app.Health.Clear(healthKeyScanDB)
}

// autoBlock 自动屏蔽超阈值 IP
func (r *FirewallScan) autoBlock(ctx context.Context, setting *biz.ScanSetting) {
	if !setting.AutoBlock {
		// 未启用时清空计数器，防止无界增长
		r.resetCounters()
		return
	}

	running, err := r.fw.Status(ctx)
	if err != nil {
		// 查询失败不等于防火墙没运行，清空计数器会丢掉本轮窗口内累计的扫描次数
		r.log.Warn(r.t.Get("failed to get firewall status"), slog.Any("err", err))
		return
	}
	if !running {
		// 防火墙未运行时也要清理计数器，防止无界增长
		r.resetCounters()
		return
	}

	whitelist := r.parseWhitelist(setting.Whitelist)
	now := time.Now()
	window := time.Duration(setting.BlockWindow) * time.Minute //nolint:gosec

	// 遍历计数器，清除过期窗口，收集超阈值 IP
	counts := make(map[string]uint)
	r.mu.Lock()
	for ip, counter := range r.ipCounters {
		if now.Sub(counter.firstSeen) >= window {
			delete(r.ipCounters, ip)
			continue
		}
		if counter.count >= setting.BlockThreshold {
			counts[ip] = counter.count
			// 触发后重置计数器
			delete(r.ipCounters, ip)
		}
	}
	r.mu.Unlock()

	for ip, count := range counts {
		if r.isWhitelisted(ip, whitelist) {
			r.log.Info(r.t.Get("skip whitelisted IP"), slog.String("ip", ip), slog.Uint64("count", uint64(count)))
			delete(counts, ip)
		}
	}
	if len(counts) == 0 {
		return
	}

	// TCX 挂在 netfilter 之前，已屏蔽的 IP 仍会继续计数，不重复下发
	blocked, err := r.scanRepo.BlockedIPs(lo.Keys(counts))
	if err != nil {
		r.log.Warn(r.t.Get("failed to read scan blocks"), slog.Any("err", err))
		return
	}
	for _, ip := range blocked {
		delete(counts, ip)
	}
	if len(counts) == 0 {
		return
	}

	var expiresAt int64
	if setting.BlockDuration > 0 {
		expiresAt = now.Add(time.Duration(setting.BlockDuration) * time.Hour).Unix() //nolint:gosec
	}
	ips := lo.Keys(counts)
	rules := lo.FlatMap(ips, func(ip string, _ int) []firewall.FireInfo {
		return r.blockRules(ip, setting.BlockOutbound)
	})

	// 先落库再下发规则：中途被杀只会留下一条到期后空删的记录，不会留下没人解封的规则
	if err = r.scanRepo.AddBlocks(lo.Map(ips, func(ip string, _ int) *biz.ScanBlock {
		return &biz.ScanBlock{IP: ip, ExpiresAt: expiresAt, Outbound: setting.BlockOutbound}
	})); err != nil {
		r.log.Warn(r.t.Get("failed to save scan blocks"), slog.Any("err", err))
		return
	}
	if err = r.fw.RichRulesBatch(ctx, rules, firewall.OperationAdd); err != nil {
		r.log.Warn(r.t.Get("failed to auto block IP"), slog.Int("count", len(ips)), slog.Any("err", err))
		// 规则可能已写入一部分，撤回成功才删记录，否则留到期满再解封
		if r.fw.RichRulesBatch(ctx, rules, firewall.OperationRemove) == nil {
			_ = r.scanRepo.DeleteBlocks(ips)
		}
		return
	}
	for ip, count := range counts {
		r.log.Info(r.t.Get("auto blocked IP"), slog.String("ip", ip), slog.Uint64("count", uint64(count)), slog.Uint64("threshold", uint64(setting.BlockThreshold)), slog.Uint64("window_min", uint64(setting.BlockWindow)))
	}
}

// cleanup 清理过期数据
// 任务每 2 分钟触发，但数据按天过期，故限流到 6 小时一次
func (r *FirewallScan) cleanup() {
	if time.Since(r.cleanedAt) < 6*time.Hour {
		return
	}
	r.cleanedAt = time.Now()

	day, err := r.setting.GetInt(biz.SettingKeyScanAwareDays, 30)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -day).Format(time.DateOnly)
	if err = r.scanRepo.ClearBefore(cutoff); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired scan data"), slog.Any("err", err))
	}
}

// blockRules 屏蔽指定 IP 全部入站流量的规则，outbound 为真时再加一条屏蔽出站的
func (r *FirewallScan) blockRules(ip string, outbound bool) []firewall.FireInfo {
	rule := firewall.FireInfo{
		Family:    r.ipFamily(ip),
		Address:   ip,
		Strategy:  firewall.StrategyDrop,
		Direction: firewall.DirectionIn,
	}
	if !outbound {
		return []firewall.FireInfo{rule}
	}

	out := rule
	out.Direction = firewall.DirectionOut
	return []firewall.FireInfo{rule, out}
}

// ipFamily 根据 IP 地址返回协议族
func (r *FirewallScan) ipFamily(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.To4() == nil {
		return "ipv6"
	}
	return "ipv4"
}

// parseWhitelist 解析白名单为 net.IPNet 列表
func (r *FirewallScan) parseWhitelist(list []string) []net.IPNet {
	var nets []net.IPNet
	for _, entry := range list {
		// 尝试 CIDR
		_, cidr, err := net.ParseCIDR(entry)
		if err == nil {
			nets = append(nets, *cidr)
			continue
		}
		// 纯 IP 转为 /32 或 /128
		ip := net.ParseIP(entry)
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(bits, bits),
		})
	}
	return nets
}

// isWhitelisted 检查 IP 是否在白名单中
func (r *FirewallScan) isWhitelisted(ip string, whitelist []net.IPNet) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() || parsed.IsUnspecified() {
		return true
	}
	return lo.ContainsBy(whitelist, func(cidr net.IPNet) bool {
		return cidr.Contains(parsed)
	})
}
