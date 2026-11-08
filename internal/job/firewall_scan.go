package job

import (
	"context"
	"fmt"
	"log/slog"
	"net"
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

// healthKeyScanDB 扫描事件辅助数据库健康问题上报 key
const healthKeyScanDB = "database:scan"

// maxScanEntries 聚合缓冲/计数器条目上限,防伪造源 IP 洪水耗尽内存
const maxScanEntries = 100000

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
	scanner      *scan.Scanner
	geoIP        *geoip.GeoIP
	geoIPPath    string
	geoIPModTime time.Time
	buffer       map[string]*biz.ScanEvent // key: "ip:port:proto:date"
	ipCounters   map[string]*ipCounter     // per-IP 扫描计数
	blockedIPs   map[string]time.Time      // 已屏蔽 IP → 屏蔽时间
	cleanedAt    time.Time
	mu           sync.Mutex

	unsupportedLogged bool // 内核不支持仅告警一次
}

// NewFirewallScan 构造防火墙扫描感知任务
func NewFirewallScan(scanEventUsecase *biz.ScanEventUsecase, settingUsecase *biz.SettingUsecase, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "*/2 * * * *",
		Task: &FirewallScan{
			log:        log,
			setting:    settingUsecase,
			scanRepo:   scanEventUsecase,
			t:          t,
			buffer:     make(map[string]*biz.ScanEvent),
			ipCounters: make(map[string]*ipCounter),
			blockedIPs: make(map[string]time.Time),
		},
	}
}

func (r *FirewallScan) Run(ctx context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	enabled, err := r.setting.GetBool(biz.SettingKeyScanAware)
	if err != nil || !enabled {
		// 未启用时，确保 scanner 已停止
		r.stopScanner()
		return nil //nolint:nilerr
	}

	// 确保 scanner 已启动
	r.ensureScanner()

	// 热更新 GeoIP
	r.geoIP, r.geoIPPath, r.geoIPModTime = refreshGeoIP(r.setting, r.geoIP, r.geoIPPath, r.geoIPModTime, r.t, r.log)

	// flush 缓冲到数据库
	r.flush()

	// 自动屏蔽/解封
	r.autoBlock(ctx)

	// 清理过期数据
	r.cleanup()
	return nil
}

// ensureScanner 确保 scanner 正在运行
func (r *FirewallScan) ensureScanner() {
	if r.scanner != nil {
		return
	}

	if !scan.Supported() {
		if !r.unsupportedLogged {
			r.log.Warn(r.t.Get("eBPF scan detector unavailable on this kernel (TCX attach requires 6.6+)"))
			r.unsupportedLogged = true
		}
		return
	}

	setting, err := r.scanRepo.GetSetting()
	if err != nil {
		r.log.Warn(r.t.Get("failed to get scan setting"), slog.Any("err", err))
		return
	}

	scanner, err := scan.New(setting.Interfaces, r.t, r.log)
	if err != nil {
		r.log.Warn(r.t.Get("failed to start eBPF scan detector"), slog.Any("err", err))
		return
	}

	r.scanner = scanner

	// 启动后台事件聚合
	go r.aggregate()
}

// stopScanner 停止 scanner
func (r *FirewallScan) stopScanner() {
	if r.scanner == nil {
		return
	}
	_ = r.scanner.Close()
	r.scanner = nil

	// 清空计数器，保留 blockedIPs
	r.mu.Lock()
	r.ipCounters = make(map[string]*ipCounter)
	r.mu.Unlock()
}

// aggregate 持续读取 eBPF 事件并聚合到内存缓冲
func (r *FirewallScan) aggregate() {
	events := r.scanner.Events()
	if events == nil {
		return
	}

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

// autoBlock 自动屏蔽超阈值 IP 并解封过期 IP
func (r *FirewallScan) autoBlock(ctx context.Context) {
	setting, err := r.scanRepo.GetSetting()
	if err != nil || !setting.AutoBlock {
		// 未启用时清空计数器，防止无界增长
		r.mu.Lock()
		if len(r.ipCounters) > 0 {
			r.ipCounters = make(map[string]*ipCounter)
		}
		r.mu.Unlock()
		return
	}

	fw := firewall.NewFirewall()
	running, err := fw.Status(ctx)
	if err != nil {
		// 查询失败不等于防火墙没运行，清空计数器会丢掉本轮窗口内累计的扫描次数
		r.log.Warn(r.t.Get("failed to get firewall status"), slog.Any("err", err))
		return
	}
	if !running {
		// 防火墙未运行时也要清理计数器，防止无界增长
		r.mu.Lock()
		if len(r.ipCounters) > 0 {
			r.ipCounters = make(map[string]*ipCounter)
		}
		r.mu.Unlock()
		return
	}

	// 解析白名单
	whitelist := parseWhitelist(setting.Whitelist)
	now := time.Now()
	window := time.Duration(setting.BlockWindow) * time.Minute   //nolint:gosec
	duration := time.Duration(setting.BlockDuration) * time.Hour //nolint:gosec

	var toBlock []struct {
		ip    string
		count uint
	}
	var toUnblock []string

	r.mu.Lock()
	// 收集需要解封的 IP
	if setting.BlockDuration > 0 {
		for ip, blockedAt := range r.blockedIPs {
			if now.Sub(blockedAt) >= duration {
				toUnblock = append(toUnblock, ip)
			}
		}
	}

	// 遍历计数器，清除过期窗口，收集超阈值 IP
	for ip, counter := range r.ipCounters {
		if now.Sub(counter.firstSeen) >= window {
			// 窗口过期，重置
			delete(r.ipCounters, ip)
			continue
		}
		if counter.count >= setting.BlockThreshold {
			if _, blocked := r.blockedIPs[ip]; !blocked {
				toBlock = append(toBlock, struct {
					ip    string
					count uint
				}{ip, counter.count})
			}
			// 触发后重置计数器
			delete(r.ipCounters, ip)
		}
	}
	r.mu.Unlock()

	// 执行解封（锁外操作，防火墙命令耗时）
	for _, ip := range toUnblock {
		family := ipFamily(ip)
		if err = fw.RichRules(ctx, firewall.FireInfo{
			Family:    family,
			Address:   ip,
			Strategy:  firewall.StrategyDrop,
			Direction: firewall.DirectionIn,
		}, firewall.OperationRemove); err != nil {
			r.log.Warn(r.t.Get("failed to unblock IP"), slog.String("ip", ip), slog.Any("err", err))
			continue
		}
		r.mu.Lock()
		delete(r.blockedIPs, ip)
		r.mu.Unlock()
		r.log.Info(r.t.Get("auto unblocked IP (expired)"), slog.String("ip", ip), slog.Uint64("duration_hours", uint64(setting.BlockDuration)))
	}

	// 执行屏蔽
	for _, item := range toBlock {
		if isWhitelisted(item.ip, whitelist) {
			r.log.Info(r.t.Get("skip whitelisted IP"), slog.String("ip", item.ip), slog.Uint64("count", uint64(item.count)))
			continue
		}
		family := ipFamily(item.ip)
		if err = fw.RichRules(ctx, firewall.FireInfo{
			Family:    family,
			Address:   item.ip,
			Strategy:  firewall.StrategyDrop,
			Direction: firewall.DirectionIn,
		}, firewall.OperationAdd); err != nil {
			r.log.Warn(r.t.Get("failed to auto block IP"), slog.String("ip", item.ip), slog.Uint64("count", uint64(item.count)), slog.Any("err", err))
			continue
		}
		r.mu.Lock()
		r.blockedIPs[item.ip] = now
		r.mu.Unlock()
		r.log.Info(r.t.Get("auto blocked IP"), slog.String("ip", item.ip), slog.Uint64("count", uint64(item.count)), slog.Uint64("threshold", uint64(setting.BlockThreshold)), slog.Uint64("window_min", uint64(setting.BlockWindow)))
	}
}

// ipFamily 根据 IP 地址返回协议族
func ipFamily(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.To4() == nil {
		return "ipv6"
	}
	return "ipv4"
}

// parseWhitelist 解析白名单为 net.IPNet 列表
func parseWhitelist(list []string) []net.IPNet {
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
func isWhitelisted(ip string, whitelist []net.IPNet) bool {
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
