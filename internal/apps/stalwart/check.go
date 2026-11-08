package stalwart

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/systemctl"
	"github.com/acepanel/panel/v3/pkg/tools"
)

// 出站 25 端口探测目标：国内外各取大厂 MX，任一可连通即认为未被封
var outboundProbes = []string{"mx1.qq.com:25", "aspmx.l.google.com:25", "outlook-com.olc.protection.outlook.com:25"}

// 常见 DNSBL，Spamhaus 对公共解析器返回 127.255.x 表示拒答
var dnsbls = []string{"zen.spamhaus.org", "bl.spamcop.net", "b.barracudacentral.org"}

// Check 自检：出站 25、主机名解析、PTR、黑名单、本机监听与冲突服务
func (s *App) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	hostname := ""
	if c, err := s.adminClient(); err == nil {
		if settings, err := getOne[swSystemSettings](ctx, c, "SystemSettings", singleton); err == nil {
			hostname = settings.DefaultHostname
		}
	}
	publicIPs, _ := s.settingRepo.GetSlice(biz.SettingKeyPublicIPs)
	if len(publicIPs) == 0 {
		if ip, err := tools.GetPublicIPv4(); err == nil && ip != "" {
			publicIPs = append(publicIPs, ip)
		}
	}
	ipv4 := lo.Filter(publicIPs, func(ip string, _ int) bool { parsed := net.ParseIP(ip); return parsed != nil && parsed.To4() != nil })

	var items []CheckItem
	var mu sync.Mutex
	var wg sync.WaitGroup
	run := func(fn func() CheckItem) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item := fn()
			mu.Lock()
			items = append(items, item)
			mu.Unlock()
		}()
	}

	run(func() CheckItem { return s.checkOutbound(ctx) })
	run(func() CheckItem { return s.checkHostname(ctx, hostname, publicIPs) })
	run(func() CheckItem { return s.checkPTR(ctx, hostname, publicIPs) })
	run(func() CheckItem { return s.checkDNSBL(ctx, ipv4) })
	run(func() CheckItem { return s.checkListening(ctx) })
	run(func() CheckItem { return s.checkConflicts(ctx) })
	wg.Wait()

	order := []string{"outbound_25", "hostname", "ptr", "dnsbl", "listening", "conflicts"}
	slices.SortFunc(items, func(a, b CheckItem) int { return slices.Index(order, a.Key) - slices.Index(order, b.Key) })

	s.log.Info(s.t.Get("mail server self check performed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.Any("results", lo.SliceToMap(items, func(i CheckItem) (string, string) { return i.Key, i.Status })))
	service.Success(w, items)
}

func (s *App) checkOutbound(ctx context.Context) CheckItem {
	item := CheckItem{Key: "outbound_25", Title: s.t.Get("Outbound port 25")}
	dialer := net.Dialer{Timeout: 5 * time.Second}
	var reached []string
	for _, target := range outboundProbes {
		conn, err := dialer.DialContext(ctx, "tcp", target)
		if err == nil {
			_ = conn.Close()
			reached = append(reached, target)
		}
	}
	if len(reached) > 0 {
		item.Status = "ok"
		item.Detail = s.t.Get("Connected to %s, direct delivery is possible", strings.Join(reached, ", "))
		return item
	}
	item.Status = "fail"
	item.Detail = s.t.Get("Cannot connect to any remote mail server on port 25. Most cloud providers block it by default; configure an outbound relay in settings or ask your provider to unblock it")
	return item
}

func (s *App) checkHostname(ctx context.Context, hostname string, publicIPs []string) CheckItem {
	item := CheckItem{Key: "hostname", Title: s.t.Get("Hostname resolution")}
	if hostname == "" {
		item.Status = "skip"
		item.Detail = s.t.Get("Mail server is not initialized")
		return item
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, hostname)
	if err != nil || len(ips) == 0 {
		item.Status = "fail"
		item.Detail = s.t.Get("%s does not resolve, add an A record pointing to this server", hostname)
		return item
	}
	if len(publicIPs) == 0 {
		item.Status = "warn"
		item.Detail = s.t.Get("%s resolves to %s, but the panel does not know this server's public IP to compare", hostname, strings.Join(ips, ", "))
		return item
	}
	if lo.Some(ips, publicIPs) {
		item.Status = "ok"
		item.Detail = s.t.Get("%s resolves to %s", hostname, strings.Join(ips, ", "))
		return item
	}
	item.Status = "fail"
	item.Detail = s.t.Get("%s resolves to %s, which does not match this server's public IP %s", hostname, strings.Join(ips, ", "), strings.Join(publicIPs, ", "))
	return item
}

func (s *App) checkPTR(ctx context.Context, hostname string, publicIPs []string) CheckItem {
	item := CheckItem{Key: "ptr", Title: s.t.Get("Reverse DNS (PTR)")}
	if hostname == "" || len(publicIPs) == 0 {
		item.Status = "skip"
		item.Detail = s.t.Get("Requires an initialized server and a known public IP")
		return item
	}
	var details []string
	status := "ok"
	for _, ip := range publicIPs {
		names, err := net.DefaultResolver.LookupAddr(ctx, ip)
		names = lo.Map(names, func(n string, _ int) string { return strings.TrimSuffix(n, ".") })
		switch {
		case err != nil || len(names) == 0:
			status = "fail"
			details = append(details, s.t.Get("%s has no PTR record", ip))
		case !lo.Contains(names, hostname):
			status = "fail"
			details = append(details, s.t.Get("%s points to %s instead of %s", ip, strings.Join(names, ", "), hostname))
		default:
			details = append(details, s.t.Get("%s points to %s", ip, hostname))
		}
	}
	item.Status = status
	item.Detail = strings.Join(details, "; ")
	if status == "fail" {
		item.Detail += ". " + s.t.Get("PTR records are set in your cloud provider's console, not in your DNS zone")
	}
	return item
}

func (s *App) checkDNSBL(ctx context.Context, ipv4 []string) CheckItem {
	item := CheckItem{Key: "dnsbl", Title: s.t.Get("IP blacklists")}
	if len(ipv4) == 0 {
		item.Status = "skip"
		item.Detail = s.t.Get("No public IPv4 address to check")
		return item
	}
	var listed, unknown []string
	for _, ip := range ipv4 {
		parts := strings.Split(ip, ".")
		slices.Reverse(parts)
		reversed := strings.Join(parts, ".")
		for _, bl := range dnsbls {
			addrs, err := net.DefaultResolver.LookupHost(ctx, reversed+"."+bl)
			if err != nil {
				continue
			}
			// Spamhaus 用 127.255.255.x 表示查询被拒绝，不代表被列入
			if lo.SomeBy(addrs, func(a string) bool { return strings.HasPrefix(a, "127.255.") }) {
				unknown = append(unknown, bl)
				continue
			}
			if len(addrs) > 0 {
				listed = append(listed, fmt.Sprintf("%s@%s", ip, bl))
			}
		}
	}
	switch {
	case len(listed) > 0:
		item.Status = "fail"
		item.Detail = s.t.Get("Listed on %s, delivery to major providers will be rejected until delisted", strings.Join(listed, ", "))
	case len(unknown) > 0:
		item.Status = "warn"
		item.Detail = s.t.Get("Not listed on the blacklists that answered; %s refused the query from this resolver", strings.Join(lo.Uniq(unknown), ", "))
	default:
		item.Status = "ok"
		item.Detail = s.t.Get("Not listed on %s", strings.Join(dnsbls, ", "))
	}
	return item
}

func (s *App) checkListening(ctx context.Context) CheckItem {
	item := CheckItem{Key: "listening", Title: s.t.Get("Mail ports")}
	var down []string
	dialer := net.Dialer{Timeout: time.Second}
	for _, port := range []string{"25", "465", "993"} {
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
		if err != nil {
			down = append(down, port)
			continue
		}
		_ = conn.Close()
	}
	if len(down) == 0 {
		item.Status = "ok"
		item.Detail = s.t.Get("Ports 25, 465 and 993 are listening locally; make sure they are open in the firewall and security group")
		return item
	}
	item.Status = "fail"
	item.Detail = s.t.Get("Ports %s are not listening, check the service status and log", strings.Join(down, ", "))
	return item
}

func (s *App) checkConflicts(ctx context.Context) CheckItem {
	item := CheckItem{Key: "conflicts", Title: s.t.Get("Conflicting mail services")}
	var active []string
	for _, name := range []string{"postfix", "sendmail", "exim4", "exim", "opensmtpd", "dovecot"} {
		if ok, _ := systemctl.Status(ctx, name); ok {
			active = append(active, name)
		}
	}
	if len(active) == 0 {
		item.Status = "ok"
		item.Detail = s.t.Get("No other mail service is running")
		return item
	}
	item.Status = "warn"
	item.Detail = s.t.Get("%s is running and may compete for mail ports, stop and disable it", strings.Join(active, ", "))
	return item
}
