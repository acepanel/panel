package firewall

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/pkg/shell"
)

type ufw struct {
	ruleRegex *regexp.Regexp
	natRegex  *regexp.Regexp
}

func newUFW() *ufw {
	return &ufw{
		// 匹配 ufw status numbered 的输出行，From 列后可能跟 (v6)、on eth0、(out)、(log) 与 # 注释
		// [ 1] 22/tcp                     ALLOW IN    Anywhere
		// [ 2] 80/tcp                     DENY IN     192.168.1.0/24
		// [ 3] 1.2.3.4 443/tcp            ALLOW OUT   Anywhere                   (out) # api
		ruleRegex: regexp.MustCompile(`^\[\s*\d+\]\s+(.+?)\s+(ALLOW|DENY|REJECT|LIMIT)\s+(IN|OUT)\s+(.+)$`),
		// 匹配 before.rules 中的 NAT PREROUTING 规则
		natRegex: regexp.MustCompile(`^-A PREROUTING -p (\w+) --dport (\d+) -j DNAT --to-destination (.+):(\d+)$`),
	}
}

func (r *ufw) Status(ctx context.Context) (bool, error) {
	out, err := shell.Execf(ctx, "ufw status")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "Status: active"), nil
}

func (r *ufw) Enable(ctx context.Context) error {
	_, err := shell.Execf(ctx, "ufw --force enable")
	return err
}

func (r *ufw) Disable(ctx context.Context) error {
	_, err := shell.Execf(ctx, "ufw disable")
	return err
}

func (r *ufw) ListRule(ctx context.Context) ([]FireInfo, error) {
	out, err := shell.Execf(ctx, "ufw status numbered")
	if err != nil {
		return nil, err
	}

	var data []FireInfo
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if !r.ruleRegex.MatchString(line) {
			continue
		}
		match := r.ruleRegex.FindStringSubmatch(line)
		if len(match) < 5 {
			continue
		}

		target := match[1]    // 如 22/tcp, 80:443/tcp, Anywhere
		action := match[2]    // ALLOW, DENY, REJECT, LIMIT
		direction := match[3] // IN, OUT
		source := match[4]    // Anywhere, 192.168.1.0/24

		info := r.parseRule(target, action, direction, source)
		if info != nil {
			data = append(data, *info)
		}
	}

	slices.SortFunc(data, func(a FireInfo, b FireInfo) int {
		if a.PortStart != b.PortStart {
			return cmp.Compare(a.PortStart, b.PortStart)
		}
		if a.PortEnd != b.PortEnd {
			return cmp.Compare(a.PortEnd, b.PortEnd)
		}
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})

	return mergeRules(data), nil
}

// ufwPortRegex 匹配列中的端口规格：22、80:443、22/tcp、80:443/udp
var ufwPortRegex = regexp.MustCompile(`^(\d{1,5})(?::(\d{1,5}))?(?:/(tcp|udp))?$`)

// ufwEndpoint ufw status 一列（To 或 From）解析出的地址、端口与协议
type ufwEndpoint struct {
	address   string
	portStart uint
	portEnd   uint
	protocol  string
	v6        bool
}

// parseEndpoint 解析 ufw status numbered 的 To 或 From 列
// 列形如 "Anywhere"、"22/tcp"、"1.2.3.4"、"10.0.0.8/tcp"、"1.2.3.4 443/tcp"，后面可能跟 (v6)、(out)、(log)、on eth0
// 绑定网卡、应用配置名（如 "Nginx Full"）、多端口列表（如 80,443/tcp）面板无法表示，返回 false
func parseEndpoint(col string) (ufwEndpoint, bool) {
	var ep ufwEndpoint
	for _, token := range strings.Fields(col) {
		switch {
		case token == "(v6)":
			ep.v6 = true
		case token == "(out)", token == "(log)", token == "(log-all)":
		case token == "on":
			return ep, false
		case strings.HasPrefix(token, "Anywhere"):
			_, proto := stripProtocolSuffix(token)
			ep.protocol = cmp.Or(proto, ep.protocol)
		case ufwPortRegex.MatchString(token):
			m := ufwPortRegex.FindStringSubmatch(token)
			ep.portStart = cast.ToUint(m[1])
			ep.portEnd = cmp.Or(cast.ToUint(m[2]), ep.portStart)
			ep.protocol = cmp.Or(m[3], ep.protocol)
		default:
			addr, proto := stripProtocolSuffix(token)
			if !isIPOrCIDR(addr) {
				return ep, false
			}
			ep.address = addr
			ep.protocol = cmp.Or(proto, ep.protocol)
		}
	}
	return ep, true
}

// parseRule 解析 ufw status numbered 的单行规则，To 列是目的地址/端口，From 列是来源地址/端口
// 面板模型只有目的端口加一个地址（入站取来源、出站取目的），带来源端口或两侧都带地址的规则无法表示，返回 nil
func (r *ufw) parseRule(target, action, direction, source string) *FireInfo {
	// 行尾注释（如 Fail2Ban 的 "1.2.3.4  # by Fail2Ban ..."）会被正则并入 source，需先去掉
	source, _, _ = strings.Cut(source, "#")

	to, ok := parseEndpoint(target)
	if !ok {
		return nil
	}
	from, ok := parseEndpoint(source)
	if !ok {
		return nil
	}

	info := &FireInfo{Type: TypeNormal, Family: "ipv4", Protocol: ProtocolTCPUDP, Direction: DirectionIn}
	if strings.EqualFold(direction, "out") {
		info.Direction = DirectionOut
	}

	info.Address = from.address
	other := to.address
	if info.Direction == DirectionOut {
		info.Address, other = to.address, from.address
	}
	if from.portStart != 0 || other != "" {
		return nil
	}

	switch strings.ToUpper(action) {
	case "ALLOW", "LIMIT":
		info.Strategy = StrategyAccept
	case "DENY":
		info.Strategy = StrategyDrop
	case "REJECT":
		info.Strategy = StrategyReject
	}

	// 列里带具体 IPv6 地址时 ufw 不再标 (v6)
	if to.v6 || from.v6 || strings.Contains(info.Address, ":") {
		info.Family = "ipv6"
	}
	if proto := cmp.Or(to.protocol, from.protocol); proto != "" {
		info.Protocol = Protocol(proto)
	}

	info.PortStart, info.PortEnd = to.portStart, to.portEnd
	// 无目的端口即 IP 规则，面板以全端口范围表示
	if to.portStart == 0 {
		info.PortStart, info.PortEnd = 1, 65535
		info.Type = TypeRich
	}
	if info.Address != "" {
		info.Type = TypeRich
	}

	return info
}

// stripProtocolSuffix 从地址中剥离协议后缀（如 "8.8.8.8/tcp" → "8.8.8.8", "tcp"）
// CIDR 格式（如 "192.168.1.0/24"）不受影响
func stripProtocolSuffix(addr string) (string, string) {
	idx := strings.LastIndex(addr, "/")
	if idx == -1 {
		return addr, ""
	}
	suffix := strings.ToLower(addr[idx+1:])
	switch suffix {
	case "tcp", "udp":
		return addr[:idx], suffix
	default:
		return addr, ""
	}
}

// isIPOrCIDR 判断是否为 IP 或 CIDR
func isIPOrCIDR(s string) bool {
	if _, err := netip.ParseAddr(s); err == nil {
		return true
	}
	_, err := netip.ParsePrefix(s)
	return err == nil
}

func (r *ufw) Port(ctx context.Context, rule FireInfo, operation Operation) error {
	if rule.PortEnd == 0 {
		rule.PortEnd = rule.PortStart
	}
	if rule.PortStart > rule.PortEnd {
		return fmt.Errorf("invalid port range: %d-%d", rule.PortStart, rule.PortEnd)
	}

	// 有地址或非默认策略，使用 RichRules
	if rule.Address != "" || rule.Type == TypeRich {
		return r.RichRules(ctx, rule, operation)
	}

	if rule.Protocol == "" {
		rule.Protocol = ProtocolTCPUDP
	}

	if operation == OperationRemove {
		return r.deletePort(ctx, rule)
	}

	// 添加规则：使用简单语法
	for _, protocol := range buildProtocols(rule.Protocol) {
		cmd := r.buildSimplePortCmd(rule, protocol)
		if _, err := shell.Exec(ctx, cmd); err != nil {
			return err
		}
	}

	return nil
}

// deletePort 删除端口规则
func (r *ufw) deletePort(ctx context.Context, rule FireInfo) error {
	// tcp/udp 时先尝试无协议删除（匹配 ufw allow 8888 这种原生规则）
	if rule.Protocol == ProtocolTCPUDP {
		_, _ = shell.Execf(ctx, "ufw delete %s %s", r.strategyToUFW(rule.Strategy), r.formatPort(rule))
	}

	for _, protocol := range buildProtocols(rule.Protocol) {
		// 简单语法: ufw delete allow 443/tcp
		_, _ = shell.Execf(ctx, "ufw delete %s %s/%s", r.strategyToUFW(rule.Strategy), r.formatPort(rule), protocol)
		// 扩展语法: ufw delete allow in proto tcp to any port 443
		_, _ = shell.Exec(ctx, r.buildPortCmd(rule, protocol, OperationRemove))
	}

	return nil
}

// formatPort 格式化端口（单端口或范围）
func (r *ufw) formatPort(rule FireInfo) string {
	if rule.PortStart == rule.PortEnd {
		return strconv.FormatUint(uint64(rule.PortStart), 10)
	}
	return fmt.Sprintf("%d:%d", rule.PortStart, rule.PortEnd)
}

// buildSimplePortCmd 构建简单语法命令: ufw allow 443/tcp
func (r *ufw) buildSimplePortCmd(rule FireInfo, protocol string) string {
	if protocol != "" {
		return fmt.Sprintf("ufw %s %s/%s", r.strategyToUFW(rule.Strategy), r.formatPort(rule), protocol)
	}
	return fmt.Sprintf("ufw %s %s", r.strategyToUFW(rule.Strategy), r.formatPort(rule))
}

// buildPortCmd 构建扩展语法命令（带协议、方向）
func (r *ufw) buildPortCmd(rule FireInfo, protocol string, operation Operation) string {
	var sb strings.Builder

	if operation == OperationRemove {
		sb.WriteString("ufw delete ")
	} else {
		sb.WriteString("ufw ")
	}

	sb.WriteString(r.strategyToUFW(rule.Strategy))
	sb.WriteString(" ")

	if strings.ToLower(string(rule.Direction)) == "out" {
		sb.WriteString("out ")
	} else {
		sb.WriteString("in ")
	}

	_, _ = fmt.Fprintf(&sb, "proto %s to any port %s", protocol, r.formatPort(rule))
	return sb.String()
}

func (r *ufw) RichRules(ctx context.Context, rule FireInfo, operation Operation) error {
	// 出站规则下，必须指定具体的地址
	if rule.Direction == "out" && rule.Address == "" {
		return errors.New("outbound rules must specify an address")
	}

	if rule.Protocol == "" {
		rule.Protocol = ProtocolTCPUDP
	}

	// 删除时额外尝试无协议命令（匹配 ufw allow 8888 这种原生合并规则）
	if operation == OperationRemove && rule.Protocol == ProtocolTCPUDP {
		_, _ = shell.Exec(ctx, r.buildRichCmd(rule, "", operation))
	}

	for _, protocol := range buildProtocols(rule.Protocol) {
		_, err := shell.Exec(ctx, r.buildRichCmd(rule, protocol, operation))
		if err != nil && operation != OperationRemove {
			return err
		}
	}

	return nil
}

// buildRichCmd 构建 ufw 富规则命令
// UFW 扩展语法: ufw [delete] allow|deny|reject [in|out] [proto PROTO] [from ADDR] [to ADDR port PORT]
func (r *ufw) buildRichCmd(rule FireInfo, protocol string, operation Operation) string {
	var sb strings.Builder

	if operation == OperationRemove {
		sb.WriteString("ufw delete ")
	} else {
		sb.WriteString("ufw ")
	}

	sb.WriteString(r.strategyToUFW(rule.Strategy))
	sb.WriteString(" ")

	dir := strings.ToLower(string(rule.Direction))
	if dir == "out" {
		sb.WriteString("out ")
	} else {
		sb.WriteString("in ")
	}

	hasPort := rule.PortStart != 0 && rule.PortEnd != 0 && (rule.PortStart != 1 || rule.PortEnd != 65535)
	if protocol != "" && (hasPort || rule.Address != "") {
		_, _ = fmt.Fprintf(&sb, "proto %s ", protocol)
	}

	if rule.Address != "" {
		if dir == "out" {
			_, _ = fmt.Fprintf(&sb, "to %s ", rule.Address)
		} else {
			_, _ = fmt.Fprintf(&sb, "from %s ", rule.Address)
		}
	}

	if hasPort {
		_, _ = fmt.Fprintf(&sb, "to any port %s", r.formatPort(rule))
	}

	return sb.String()
}

// strategyToUFW 将内部策略映射到 ufw 命令关键字
func (r *ufw) strategyToUFW(strategy Strategy) string {
	switch strategy {
	case StrategyDrop:
		return "deny"
	case StrategyReject:
		return "reject"
	default:
		return "allow"
	}
}

const beforeRulesPath = "/etc/ufw/before.rules"

const natMarker = "# acepanel-forward"

func (r *ufw) ListForward(ctx context.Context) ([]FireForwardInfo, error) {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	var data []FireForwardInfo
	lines := strings.Split(string(content), "\n")
	inNat := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "*nat" {
			inNat = true
			continue
		}
		if trimmed == "COMMIT" && inNat {
			break
		}
		if !inNat {
			continue
		}

		if r.natRegex.MatchString(trimmed) {
			match := r.natRegex.FindStringSubmatch(trimmed)
			if len(match) < 5 {
				continue
			}
			data = append(data, FireForwardInfo{
				Protocol:   Protocol(match[1]),
				Port:       cast.ToUint(match[2]),
				TargetIP:   match[3],
				TargetPort: cast.ToUint(match[4]),
			})
		}
	}

	slices.SortFunc(data, func(a FireForwardInfo, b FireForwardInfo) int {
		if a.Port != b.Port {
			return cmp.Compare(a.Port, b.Port)
		}
		if a.TargetPort != b.TargetPort {
			return cmp.Compare(a.TargetPort, b.TargetPort)
		}
		return strings.Compare(string(a.Protocol), string(b.Protocol))
	})

	return data, nil
}

func (r *ufw) Forward(ctx context.Context, rule Forward, operation Operation) error {
	if operation == OperationAdd {
		return r.addForward(ctx, rule)
	}
	return r.removeForward(ctx, rule)
}

func (r *ufw) addForward(ctx context.Context, rule Forward) error {
	// 启用 IP 转发
	_, _ = shell.Execf(ctx, "sysctl -w net.ipv4.ip_forward=1")
	_, _ = shell.Execf(ctx, "sysctl -w net.ipv6.conf.all.forwarding=1")
	_ = os.WriteFile("/etc/sysctl.d/99-acepanel-forward.conf", []byte("net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n"), 0644)

	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", beforeRulesPath, err)
	}

	text := string(content)

	for _, protocol := range buildProtocols(rule.Protocol) {
		dnatRule := fmt.Sprintf("-A PREROUTING -p %s --dport %d -j DNAT --to-destination %s:%d", protocol, rule.Port, rule.TargetIP, rule.TargetPort)
		masqRule := fmt.Sprintf("-A POSTROUTING -d %s -p %s --dport %d -j MASQUERADE", rule.TargetIP, protocol, rule.TargetPort)

		// 检查规则是否已存在
		if strings.Contains(text, dnatRule) {
			continue
		}

		// 查找或创建 *nat 段
		if !strings.Contains(text, "*nat") {
			// 在文件开头（filter 段之前）插入 *nat 段
			natBlock := fmt.Sprintf("*nat\n:PREROUTING ACCEPT [0:0]\n:POSTROUTING ACCEPT [0:0]\n%s\n%s\n%s\nCOMMIT\n\n", natMarker, dnatRule, masqRule)
			text = natBlock + text
		} else {
			// 在 *nat 段的 COMMIT 前插入规则
			commitIdx := r.findNatCommit(text)
			if commitIdx == -1 {
				return fmt.Errorf("malformed %s: *nat section without COMMIT", beforeRulesPath)
			}
			insert := fmt.Sprintf("%s\n%s\n%s\n", natMarker, dnatRule, masqRule)
			text = text[:commitIdx] + insert + text[commitIdx:]
		}
	}

	if err = os.WriteFile(beforeRulesPath, []byte(text), 0644); err != nil {
		return err
	}

	// 允许所有转发流量
	_, _ = shell.Execf(ctx, "ufw default allow routed")
	_, err = shell.Execf(ctx, "ufw reload")
	return err
}

func (r *ufw) removeForward(ctx context.Context, rule Forward) error {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return nil //nolint:nilerr
	}

	lines := strings.Split(string(content), "\n")
	var result []string

	for _, protocol := range buildProtocols(rule.Protocol) {
		dnatRule := fmt.Sprintf("-A PREROUTING -p %s --dport %d -j DNAT --to-destination %s:%d", protocol, rule.Port, rule.TargetIP, rule.TargetPort)
		masqRule := fmt.Sprintf("-A POSTROUTING -d %s -p %s --dport %d -j MASQUERADE", rule.TargetIP, protocol, rule.TargetPort)

		result = make([]string, 0, len(lines))
		skipNextMarker := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// 删除标记行（当下一行是要删除的规则时）
			if trimmed == natMarker {
				// 检查下一行是否是要删除的规则
				if i+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[i+1])
					if nextLine == dnatRule || nextLine == masqRule {
						skipNextMarker = true
						continue
					}
				}
			}
			if skipNextMarker {
				skipNextMarker = false
				continue
			}
			if trimmed == dnatRule || trimmed == masqRule {
				continue
			}
			result = append(result, line)
		}
		lines = result
	}

	if err = os.WriteFile(beforeRulesPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}

	_, err = shell.Execf(ctx, "ufw reload")
	return err
}

// findNatCommit 找到 *nat 段中 COMMIT 的位置
func (r *ufw) findNatCommit(text string) int {
	natIdx := strings.Index(text, "*nat")
	if natIdx == -1 {
		return -1
	}
	// 从 *nat 之后找第一个 COMMIT
	rest := text[natIdx:]
	commitIdx := strings.Index(rest, "COMMIT")
	if commitIdx == -1 {
		return -1
	}
	return natIdx + commitIdx
}

func (r *ufw) PingStatus(ctx context.Context) (bool, error) {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return true, nil //nolint:nilerr
	}

	// 检查 icmp echo-request 规则
	// 默认 before.rules 包含: -A ufw-before-input -p icmp --icmp-type echo-request -j ACCEPT
	text := string(content)
	if strings.Contains(text, "-p icmp --icmp-type echo-request -j DROP") {
		return false, nil
	}

	return true, nil
}

func (r *ufw) UpdatePingStatus(ctx context.Context, status bool) error {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", beforeRulesPath, err)
	}

	text := string(content)

	if status {
		// 允许 Ping: 将 DROP 改回 ACCEPT
		text = strings.ReplaceAll(text,
			"-p icmp --icmp-type echo-request -j DROP",
			"-p icmp --icmp-type echo-request -j ACCEPT")
	} else {
		// 禁止 Ping: 将 ACCEPT 改为 DROP
		text = strings.ReplaceAll(text,
			"-p icmp --icmp-type echo-request -j ACCEPT",
			"-p icmp --icmp-type echo-request -j DROP")
	}

	if err = os.WriteFile(beforeRulesPath, []byte(text), 0644); err != nil {
		return err
	}

	_, err = shell.Execf(ctx, "ufw reload")
	return err
}
