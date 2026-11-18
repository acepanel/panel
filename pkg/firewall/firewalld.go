package firewall

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/spf13/cast"

	"github.com/acepanel/panel/v3/pkg/shell"
	"github.com/acepanel/panel/v3/pkg/systemctl"
)

// outPolicy 出站规则所在的策略：区域只过滤入站，本机出站要靠 HOST 到 ANY 的策略
const outPolicy = "acepanel-out"

type firewalld struct {
	forwardListRegex *regexp.Regexp
	richRuleRegex    *regexp.Regexp
}

func newFirewalld() *firewalld {
	return &firewalld{
		forwardListRegex: regexp.MustCompile(`^port=(\d{1,5}):proto=(.+?):toport=(\d{1,5}):toaddr=(.*)$`),
		richRuleRegex:    regexp.MustCompile(`^rule family="([^"]+)"(?: .*?(source|destination) address="([^"]+)")?(?: .*?port port="([^"]+)")?(?: .*?protocol(?: value)?="([^"]+)")?.*?(accept|drop|reject|mark).*?$`),
	}
}

func (r *firewalld) Status(ctx context.Context) (bool, error) {
	return systemctl.Status(ctx, "firewalld")
}

func (r *firewalld) Enable(ctx context.Context) error {
	if err := systemctl.Start(ctx, "firewalld"); err != nil {
		return err
	}
	return systemctl.Enable(ctx, "firewalld")
}

func (r *firewalld) Disable(ctx context.Context) error {
	if err := systemctl.Stop(ctx, "firewalld"); err != nil {
		return err
	}
	return systemctl.Disable(ctx, "firewalld")
}

func (r *firewalld) ListRule(ctx context.Context) ([]FireInfo, error) {
	var wg sync.WaitGroup
	var portRules []FireInfo
	var inRules []FireInfo
	var outRules []FireInfo
	wg.Add(3)

	go func() {
		defer wg.Done()
		out, err := shell.Execf(ctx, "firewall-cmd --zone=public --list-ports")
		if err != nil {
			return
		}
		ports := strings.SplitSeq(out, " ")
		for port := range ports {
			if len(port) == 0 {
				continue
			}
			var item FireInfo
			item.Type = TypeNormal
			if strings.Contains(port, "/") {
				ruleItem := strings.Split(port, "/")
				portItem := strings.Split(ruleItem[0], "-")
				if len(portItem) > 1 {
					item.PortStart = cast.ToUint(portItem[0])
					item.PortEnd = cast.ToUint(portItem[1])
				} else {
					item.PortStart = cast.ToUint(ruleItem[0])
					item.PortEnd = cast.ToUint(ruleItem[0])
				}
				item.Protocol = Protocol(ruleItem[1])
			}
			item.Family = "ipv4"
			item.Strategy = "accept"
			item.Direction = "in"
			portRules = append(portRules, item)
		}
	}()
	go func() {
		defer wg.Done()
		inRules, _ = r.listRichRule(ctx, DirectionIn)
	}()
	go func() {
		defer wg.Done()
		// 没加过出站规则时策略还不存在，命令报错即没有规则
		outRules, _ = r.listRichRule(ctx, DirectionOut)
	}()

	wg.Wait()

	return slices.Concat(portRules, inRules, outRules), nil
}

func (r *firewalld) Port(ctx context.Context, rule FireInfo, operation Operation) error {
	if rule.PortEnd == 0 {
		rule.PortEnd = rule.PortStart
	}
	if rule.PortStart > rule.PortEnd {
		return fmt.Errorf("invalid port range: %d-%d", rule.PortStart, rule.PortEnd)
	}
	// 不支持的切换使用rich rules
	if (rule.Family != "" && rule.Family != "ipv4") || rule.Direction != "in" || rule.Address != "" || rule.Strategy != "accept" || rule.Type == TypeRich {
		return r.RichRules(ctx, rule, operation)
	}

	// 未设置协议默认为tcp/udp
	if rule.Protocol == "" {
		rule.Protocol = ProtocolTCPUDP
	}

	// 删除时忽略错误（规则可能已不存在）
	for _, protocol := range buildProtocols(rule.Protocol) {
		_, err := shell.Execf(ctx, "firewall-cmd --zone=public --%s-port=%d-%d/%s --permanent", operation, rule.PortStart, rule.PortEnd, protocol)
		if err != nil && operation != OperationRemove {
			return err
		}
	}

	_, err := shell.Execf(ctx, "firewall-cmd --reload")
	return err
}

func (r *firewalld) RichRules(ctx context.Context, rule FireInfo, operation Operation) error {
	return r.RichRulesBatch(ctx, []FireInfo{rule}, operation)
}

func (r *firewalld) RichRulesBatch(ctx context.Context, rules []FireInfo, operation Operation) error {
	// richRuleChunk 单条 firewall-cmd 携带的规则数上限，避免命令行过长
	const richRuleChunk = 200

	options := make(map[string][]string) // 按作用域分组
	for _, rule := range rules {
		// 出站规则下，必须指定具体的地址
		if rule.Direction == DirectionOut && rule.Address == "" {
			return errors.New("outbound rules must specify an address")
		}
		scope := r.scope(rule.Direction)
		for _, protocol := range buildProtocols(rule.Protocol) {
			options[scope] = append(options[scope], fmt.Sprintf("--%s-rich-rule '%s'", operation, r.buildRichRuleStr(rule, protocol)))
		}
	}

	if operation == OperationAdd && len(options[r.scope(DirectionOut)]) > 0 {
		if err := r.ensureOutPolicy(ctx); err != nil {
			return err
		}
	}

	// 删除时忽略错误（规则可能已不存在）
	for _, scope := range []string{r.scope(DirectionIn), r.scope(DirectionOut)} {
		for chunk := range slices.Chunk(options[scope], richRuleChunk) {
			_, err := shell.Execf(ctx, "firewall-cmd %s --permanent %s", scope, strings.Join(chunk, " "))
			if err != nil && operation != OperationRemove {
				return err
			}
		}
	}

	_, err := shell.Execf(ctx, "firewall-cmd --reload")
	return err
}

func (r *firewalld) ListForward(ctx context.Context) ([]FireForwardInfo, error) {
	out, err := shell.Execf(ctx, "firewall-cmd --zone=public --list-forward-ports")
	if err != nil {
		return nil, err
	}

	var data []FireForwardInfo
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimFunc(line, func(r rune) bool {
			return r <= 32
		})
		if r.forwardListRegex.MatchString(line) {
			match := r.forwardListRegex.FindStringSubmatch(line)
			if len(match) < 4 {
				continue
			}
			if len(match[4]) == 0 {
				match[4] = "127.0.0.1"
			}
			data = append(data, FireForwardInfo{
				Port:       cast.ToUint(match[1]),
				Protocol:   Protocol(match[2]),
				TargetIP:   match[4],
				TargetPort: cast.ToUint(match[3]),
			})
		}
	}

	return data, nil
}

func (r *firewalld) Forward(ctx context.Context, rule Forward, operation Operation) error {
	if err := r.enableForward(ctx); err != nil {
		return err
	}

	enableIPForward(ctx)

	for _, protocol := range buildProtocols(rule.Protocol) {
		var toAddr string
		if rule.TargetIP != "" && !r.isLocalAddress(rule.TargetIP) {
			toAddr = ":toaddr=" + rule.TargetIP
		}
		_, err := shell.Execf(ctx, "firewall-cmd --zone=public --%s-forward-port=port=%d:proto=%s:toport=%d%s --permanent", operation, rule.Port, protocol, rule.TargetPort, toAddr)
		if err != nil && operation != OperationRemove {
			return err
		}
	}

	_, err := shell.Execf(ctx, "firewall-cmd --reload")
	return err
}

func (r *firewalld) PingStatus(ctx context.Context) (bool, error) {
	// 未运行时 firewall-cmd 要重试约 10 秒才失败，此时也没有规则拦截 Ping
	if running, err := r.Status(ctx); err != nil || !running {
		return true, err
	}

	out, err := shell.Execf(ctx, "firewall-cmd --zone=public --list-rich-rules")
	if err != nil {
		return true, nil //nolint:nilerr
	}

	if !strings.Contains(out, `rule protocol value="icmp" drop`) {
		return true, nil
	}

	return false, nil
}

func (r *firewalld) UpdatePingStatus(ctx context.Context, status bool) error {
	var err error
	if status {
		_, err = shell.Execf(ctx, `firewall-cmd --zone=public --permanent --remove-rich-rule='rule protocol value=icmp drop'`)
	} else {
		_, err = shell.Execf(ctx, `firewall-cmd --zone=public --permanent --add-rich-rule='rule protocol value=icmp drop'`)
	}
	if err != nil {
		return err
	}

	_, err = shell.Execf(ctx, "firewall-cmd --reload")
	return err
}

// scope 返回该方向的规则所在的 firewall-cmd 作用域
func (r *firewalld) scope(direction Direction) string {
	if direction == DirectionOut {
		return "--policy=" + outPolicy
	}
	return "--zone=public"
}

// ensureOutPolicy 确保出站策略存在
// 出口区域最后才设，查到它就说明策略已建完整
func (r *firewalld) ensureOutPolicy(ctx context.Context) error {
	if _, err := shell.Execf(ctx, "firewall-cmd --permanent --policy=%s --query-egress-zone=ANY", outPolicy); err == nil {
		return nil
	}

	// 上次建到一半时策略已在，新建会报重名
	_, _ = shell.Execf(ctx, "firewall-cmd --permanent --new-policy=%s", outPolicy)
	// firewall-cmd 不允许一条命令同时设入口和出口区域
	for _, zone := range []string{"--add-ingress-zone=HOST", "--add-egress-zone=ANY"} {
		if _, err := shell.Execf(ctx, "firewall-cmd --permanent --policy=%s %s", outPolicy, zone); err != nil {
			return err
		}
	}

	return nil
}

func (r *firewalld) listRichRule(ctx context.Context, direction Direction) ([]FireInfo, error) {
	out, err := shell.Execf(ctx, "firewall-cmd %s --list-rich-rules", r.scope(direction))
	if err != nil {
		return nil, err
	}

	var data []FireInfo
	rules := strings.SplitSeq(out, "\n")
	for rule := range rules {
		if len(rule) == 0 {
			continue
		}
		if richRules, err := r.parseRichRule(rule, direction); err == nil {
			data = append(data, richRules)
		}
	}

	return data, nil
}

// parseRichRule 解析 direction 方向作用域里的一条富规则
// 面板模型里入站的地址是来源、出站的地址是目标，反过来的规则无法表示
func (r *firewalld) parseRichRule(line string, direction Direction) (FireInfo, error) {
	if !r.richRuleRegex.MatchString(line) {
		return FireInfo{}, errors.New("invalid rich rule format")
	}

	match := r.richRuleRegex.FindStringSubmatch(line)
	if len(match) < 7 {
		return FireInfo{}, errors.New("invalid rich rule")
	}

	if match[2] != "" && (match[2] == "destination") != (direction == DirectionOut) {
		return FireInfo{}, errors.New("unsupported rich rule")
	}

	fireInfo := FireInfo{
		Type:      TypeRich,
		Family:    match[1],
		Address:   match[3],
		Protocol:  Protocol(match[5]),
		Strategy:  Strategy(match[6]),
		Direction: direction,
	}

	if fireInfo.Protocol == "" {
		fireInfo.Protocol = "tcp/udp"
	}

	ports := strings.Split(match[4], "-")
	switch {
	case len(ports) == 2: // 添加端口范围
		fireInfo.PortStart = cast.ToUint(ports[0])
		fireInfo.PortEnd = cast.ToUint(ports[1])
	case len(ports) == 1 && ports[0] != "": // 添加单个端口
		port := cast.ToUint(ports[0])
		fireInfo.PortStart = port
		fireInfo.PortEnd = port
	case len(ports) == 1 && ports[0] == "": // 未添加端口规则，表示所有端口
		fireInfo.PortStart = 1
		fireInfo.PortEnd = 65535
	}

	return fireInfo, nil
}

// buildRichRuleStr 构建 firewalld 富规则字符串
func (r *firewalld) buildRichRuleStr(rule FireInfo, protocol string) string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, `rule family="%s" `, rule.Family)

	if rule.Address != "" {
		switch rule.Direction {
		case DirectionOut:
			_, _ = fmt.Fprintf(&sb, `destination address="%s" `, rule.Address)
		default:
			_, _ = fmt.Fprintf(&sb, `source address="%s" `, rule.Address)
		}
	}
	if rule.PortStart != 0 && rule.PortEnd != 0 && (rule.PortStart != 1 || rule.PortEnd != 65535) {
		_, _ = fmt.Fprintf(&sb, `port port="%d-%d" `, rule.PortStart, rule.PortEnd)
	}
	if protocol != "" {
		sb.WriteString(`protocol`)
		if rule.PortStart == 0 && rule.PortEnd == 0 { // IP 规则下，必须添加 value
			sb.WriteString(` value`)
		}
		_, _ = fmt.Fprintf(&sb, `="%s" `, protocol)
	}

	sb.WriteString(string(rule.Strategy))
	return sb.String()
}

func (r *firewalld) enableForward(ctx context.Context) error {
	out, err := shell.Execf(ctx, "firewall-cmd --zone=public --query-masquerade")
	if err != nil {
		if out == "no" {
			out, err = shell.Execf(ctx, "firewall-cmd --zone=public --add-masquerade --permanent")
			if err != nil {
				return fmt.Errorf("%w: %s", err, out)
			}
			_, err = shell.Execf(ctx, "firewall-cmd --reload")
			return err
		}
		return fmt.Errorf("%w: %s", err, out)
	}

	return nil
}

// isLocalAddress 判断是否为本地地址
func (r *firewalld) isLocalAddress(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() {
		return true
	}
	if parsed.IsUnspecified() {
		return true
	}

	return false
}
