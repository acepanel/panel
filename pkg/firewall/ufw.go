package firewall

import (
	"cmp"
	"fmt"
	"os"
	"regexp"
	"slices"
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
		// 匹配 ufw status numbered 的输出行
		// [ 1] 22/tcp                     ALLOW IN    Anywhere
		// [ 2] 80/tcp                     DENY IN     192.168.1.0/24
		ruleRegex: regexp.MustCompile(`^\[\s*\d+\]\s+(.+?)\s+(ALLOW|DENY|REJECT|LIMIT)\s+(IN|OUT)\s+(.+)$`),
		// 匹配 before.rules 中的 NAT PREROUTING 规则
		natRegex: regexp.MustCompile(`^-A PREROUTING -p (\w+) --dport (\d+) -j DNAT --to-destination (.+):(\d+)$`),
	}
}

func (r *ufw) Status() (bool, error) {
	out, err := shell.Execf("ufw status")
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "Status: active"), nil
}

func (r *ufw) Enable() error {
	_, err := shell.Execf("ufw --force enable")
	return err
}

func (r *ufw) Disable() error {
	_, err := shell.Execf("ufw disable")
	return err
}

func (r *ufw) ListRule() ([]FireInfo, error) {
	out, err := shell.Execf("ufw status numbered")
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

// parseRule 解析 ufw status numbered 的单行规则
func (r *ufw) parseRule(target, action, direction, source string) *FireInfo {
	info := &FireInfo{
		Family: "ipv4",
	}

	// 解析方向
	switch strings.ToLower(direction) {
	case "out":
		info.Direction = "out"
	default:
		info.Direction = "in"
	}

	// 解析策略
	switch strings.ToUpper(action) {
	case "ALLOW", "LIMIT":
		info.Strategy = StrategyAccept
	case "DENY":
		info.Strategy = StrategyDrop
	case "REJECT":
		info.Strategy = StrategyReject
	}

	// 判断 IPv6
	if strings.Contains(target, "(v6)") || strings.Contains(source, "(v6)") {
		info.Family = "ipv6"
		target = strings.ReplaceAll(target, " (v6)", "")
		source = strings.ReplaceAll(source, " (v6)", "")
	}

	// 从 source 中提取可能附带的协议后缀（如 8.8.8.8/tcp）
	// target 的 /tcp 是端口协议规格（如 80/tcp），不能剥离
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	sourceProto := ""
	source, sourceProto = stripProtocolSuffix(source)

	// target 仅在 "Anywhere/tcp" 形式下才需要剥离协议后缀
	targetProto := ""
	if strings.HasPrefix(target, "Anywhere") {
		target, targetProto = stripProtocolSuffix(target)
	}

	// 解析地址
	if source != "Anywhere" {
		info.Address = source
	}

	// 确定协议（source 和 target 侧的协议后缀取先出现的）
	extractedProto := targetProto
	if extractedProto == "" {
		extractedProto = sourceProto
	}

	// 解析端口和协议
	if target == "Anywhere" {
		// 纯 IP 规则，无端口
		info.Type = TypeRich
		info.PortStart = 1
		info.PortEnd = 65535
		if extractedProto != "" {
			info.Protocol = Protocol(extractedProto)
		} else {
			info.Protocol = ProtocolTCPUDP
		}
		return info
	}

	// 解析 port/proto 格式
	info.Type = TypeNormal
	if strings.Contains(target, "/") {
		parts := strings.SplitN(target, "/", 2)
		portPart := parts[0]
		protoPart := strings.ToLower(parts[1])

		switch protoPart {
		case "tcp":
			info.Protocol = ProtocolTCP
		case "udp":
			info.Protocol = ProtocolUDP
		default:
			info.Protocol = Protocol(protoPart)
		}

		r.parsePort(portPart, info)
	} else {
		// 仅端口，默认 tcp/udp
		info.Protocol = ProtocolTCPUDP
		r.parsePort(target, info)
	}

	// 有地址的规则标记为 rich
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

// parsePort 解析端口部分（支持范围如 80:443）
func (r *ufw) parsePort(portStr string, info *FireInfo) {
	// ufw 使用冒号分隔端口范围
	if strings.Contains(portStr, ":") {
		parts := strings.SplitN(portStr, ":", 2)
		info.PortStart = cast.ToUint(parts[0])
		info.PortEnd = cast.ToUint(parts[1])
	} else {
		port := cast.ToUint(portStr)
		info.PortStart = port
		info.PortEnd = port
	}
}

func (r *ufw) Port(rule FireInfo, operation Operation) error {
	if rule.PortEnd == 0 {
		rule.PortEnd = rule.PortStart
	}
	if rule.PortStart > rule.PortEnd {
		return fmt.Errorf("invalid port range: %d-%d", rule.PortStart, rule.PortEnd)
	}

	// 有地址或非默认策略，使用 RichRules
	if rule.Address != "" || rule.Type == TypeRich {
		return r.RichRules(rule, operation)
	}

	if rule.Protocol == "" {
		rule.Protocol = ProtocolTCPUDP
	}

	if operation == OperationRemove {
		return r.deletePort(rule)
	}

	// 添加规则：使用简单语法
	for _, protocol := range buildProtocols(rule.Protocol) {
		cmd := r.buildSimplePortCmd(rule, protocol)
		if _, err := shell.Exec(cmd); err != nil {
			return err
		}
	}

	return nil
}

// deletePort 删除端口规则
func (r *ufw) deletePort(rule FireInfo) error {
	// tcp/udp 时先尝试无协议删除（匹配 ufw allow 8888 这种原生规则）
	if rule.Protocol == ProtocolTCPUDP {
		cmd := fmt.Sprintf("ufw delete %s %s", r.strategyToUFW(rule.Strategy), r.formatPort(rule))
		_, _ = shell.Exec(cmd)
	}

	for _, protocol := range buildProtocols(rule.Protocol) {
		// 简单语法: ufw delete allow 443/tcp
		simple := fmt.Sprintf("ufw delete %s %s/%s", r.strategyToUFW(rule.Strategy), r.formatPort(rule), protocol)
		_, _ = shell.Exec(simple)
		// 扩展语法: ufw delete allow in proto tcp to any port 443
		extended := r.buildPortCmd(rule, protocol, OperationRemove)
		_, _ = shell.Exec(extended)
	}

	return nil
}

// formatPort 格式化端口（单端口或范围）
func (r *ufw) formatPort(rule FireInfo) string {
	if rule.PortStart == rule.PortEnd {
		return fmt.Sprintf("%d", rule.PortStart)
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

func (r *ufw) RichRules(rule FireInfo, operation Operation) error {
	// 出站规则下，必须指定具体的地址
	if rule.Direction == "out" && rule.Address == "" {
		return fmt.Errorf("outbound rules must specify an address")
	}

	if rule.Protocol == "" {
		rule.Protocol = ProtocolTCPUDP
	}

	// 删除时额外尝试无协议命令（匹配 ufw allow 8888 这种原生合并规则）
	if operation == OperationRemove && rule.Protocol == ProtocolTCPUDP {
		cmd := r.buildRichCmd(rule, "", operation)
		_, _ = shell.Exec(cmd)
	}

	for _, protocol := range buildProtocols(rule.Protocol) {
		cmd := r.buildRichCmd(rule, protocol, operation)
		_, err := shell.Exec(cmd)
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

func (r *ufw) ListForward() ([]FireForwardInfo, error) {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return nil, nil // before.rules 不存在时返回空
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

func (r *ufw) Forward(rule Forward, operation Operation) error {
	if operation == OperationAdd {
		return r.addForward(rule)
	}
	return r.removeForward(rule)
}

func (r *ufw) addForward(rule Forward) error {
	// 启用 IP 转发
	_, _ = shell.Execf("sysctl -w net.ipv4.ip_forward=1")
	_, _ = shell.Execf("sysctl -w net.ipv6.conf.all.forwarding=1")
	_ = os.WriteFile("/etc/sysctl.d/99-acepanel-forward.conf", []byte("net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n"), 0644)

	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", beforeRulesPath, err)
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

	for _, protocol := range buildProtocols(rule.Protocol) {
		if _, err = shell.Exec(r.routeAllowCommand(rule, protocol)); err != nil {
			return err
		}
	}

	_, err = shell.Execf("ufw reload")
	return err
}

func (r *ufw) removeForward(rule Forward) error {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return nil
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

	for _, protocol := range buildProtocols(rule.Protocol) {
		_, _ = shell.Exec(r.routeDeleteCommand(rule, protocol))
	}

	_, err = shell.Execf("ufw reload")
	return err
}

func (r *ufw) routeAllowCommand(rule Forward, protocol string) string {
	return fmt.Sprintf("ufw route allow proto %s to %s port %d", protocol, rule.TargetIP, rule.TargetPort)
}

func (r *ufw) routeDeleteCommand(rule Forward, protocol string) string {
	return fmt.Sprintf("ufw --force delete route allow proto %s to %s port %d", protocol, rule.TargetIP, rule.TargetPort)
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

func (r *ufw) PingStatus() (bool, error) {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return true, nil
	}

	// 检查 icmp echo-request 规则
	// 默认 before.rules 包含: -A ufw-before-input -p icmp --icmp-type echo-request -j ACCEPT
	text := string(content)
	if strings.Contains(text, "-p icmp --icmp-type echo-request -j DROP") {
		return false, nil
	}

	return true, nil
}

func (r *ufw) UpdatePingStatus(status bool) error {
	content, err := os.ReadFile(beforeRulesPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", beforeRulesPath, err)
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

	_, err = shell.Execf("ufw reload")
	return err
}
