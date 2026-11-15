package network

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

type netplanBackend struct{}

type netplanMatch struct {
	group      string
	id         string
	definition map[string]any
}

func (b *netplanBackend) Name() string { return "netplan" }

func (b *netplanBackend) available(ctx context.Context) bool {
	if !hasCommand("netplan") {
		return false
	}
	yamls, _ := filepath.Glob("/etc/netplan/*.yaml")
	ymls, _ := filepath.Glob("/etc/netplan/*.yml")
	if len(yamls)+len(ymls) == 0 {
		return false
	}
	_, err := run(ctx, "netplan", "get")
	return err == nil
}

func (b *netplanBackend) Load(ctx context.Context, items []Interface) error {
	config, err := b.get(ctx)
	if err != nil {
		return err
	}
	for i := range items {
		matches := b.find(config, items[i])
		switch {
		case len(matches) == 0:
			items[i].Reason = "no matching netplan interface definition"
		case len(matches) > 1:
			items[i].Reason = "multiple netplan interface definitions match this interface"
		default:
			items[i].IPv4 = b.family(matches[0].definition, true, items[i].CurrentIPv4.Addresses)
			items[i].IPv6 = b.family(matches[0].definition, false, items[i].CurrentIPv6.Addresses)
			items[i].ConfiguredMTU = b.valueInt(matches[0].definition["mtu"])
			items[i].Editable = true
		}
	}
	return nil
}

func (b *netplanBackend) Apply(
	ctx context.Context,
	item Interface,
	config Config,
) (func(context.Context) error, error) {
	// netplanOverride 面板写入的配置片段，序号最大以覆盖系统默认配置
	const netplanOverride = "/etc/netplan/99-acepanel.yaml"

	document, err := b.get(ctx)
	if err != nil {
		return nil, err
	}
	matches := b.find(document, item)
	if len(matches) != 1 {
		return nil, fmt.Errorf("%w: netplan interface definition no longer exists", ErrValidation)
	}

	// 面板只写 99-acepanel.yaml，恢复该文件即可撤销本次变更
	backup, readErr := os.ReadFile(netplanOverride)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return nil, readErr
	}
	rollback := func(ctx context.Context) error {
		if readErr != nil {
			// 变更前该文件不存在，删除即可还原
			if err := os.Remove(netplanOverride); err != nil && !os.IsNotExist(err) {
				return err
			}
		} else if err := os.WriteFile(netplanOverride, backup, 0600); err != nil {
			return err
		}
		return b.reload(ctx)
	}

	match := matches[0]
	b.merge(match.definition, config)
	// netplan set 以点号分隔层级，而接口名本身可能含点（如 VLAN 的 eth0.100），
	// 因此键只到组名，接口名放进值里
	data, err := yaml.Marshal(map[string]any{match.id: match.definition})
	if err != nil {
		return nil, err
	}
	if _, err = run(
		ctx,
		"netplan",
		"set",
		"--origin-hint=99-acepanel",
		match.group+"="+strings.TrimSpace(string(data)),
	); err != nil {
		return nil, err
	}
	_ = os.Chmod(netplanOverride, 0600)
	if err = b.reload(ctx); err != nil {
		return nil, errors.Join(err, rollback(context.WithoutCancel(ctx)))
	}
	return rollback, nil
}

func (b *netplanBackend) reload(ctx context.Context) error {
	if _, err := run(ctx, "netplan", "generate"); err != nil {
		return err
	}
	_, err := run(ctx, "netplan", "apply")
	return err
}

// merge 把面板配置合并进 netplan 的接口定义，未建模的字段原样保留
func (b *netplanBackend) merge(definition map[string]any, config Config) {
	definition["dhcp4"] = config.IPv4.Mode == modeAuto
	definition["dhcp6"] = config.IPv6.Mode == modeAuto
	definition["accept-ra"] = config.IPv6.Mode == modeAuto
	definition["addresses"] = append(append([]string{}, config.IPv4.Addresses...), config.IPv6.Addresses...)
	definition["routes"] = b.routes(definition["routes"], config)
	// 旧式网关字段与 routes 冲突，netplan 已弃用
	delete(definition, "gateway4")
	delete(definition, "gateway6")

	nameservers := b.valueMap(definition["nameservers"])
	nameservers["addresses"] = append(append([]string{}, config.IPv4.DNS...), config.IPv6.DNS...)
	definition["nameservers"] = nameservers
	for suffix, family := range map[string]FamilyConfig{"4": config.IPv4, "6": config.IPv6} {
		overrides := b.valueMap(definition["dhcp"+suffix+"-overrides"])
		overrides["use-dns"] = family.AutoDNS
		definition["dhcp"+suffix+"-overrides"] = overrides
	}
	if config.MTU == 0 {
		delete(definition, "mtu")
	} else {
		definition["mtu"] = config.MTU
	}
}

func (b *netplanBackend) get(ctx context.Context) (map[string]any, error) {
	output, err := run(ctx, "netplan", "get")
	if err != nil {
		return nil, err
	}
	config := make(map[string]any)
	if err = yaml.Unmarshal([]byte(output), &config); err != nil {
		return nil, err
	}
	return config, nil
}

// find 按接口名、set-name、MAC 或匹配模式定位接口定义
func (b *netplanBackend) find(config map[string]any, item Interface) []netplanMatch {
	groups := []string{"ethernets", "wifis", "bonds", "bridges", "vlans"}

	network := b.valueMap(config["network"])
	var result []netplanMatch
	for _, group := range groups {
		for id, raw := range b.valueMap(network[group]) {
			definition := b.valueMap(raw)
			match := b.valueMap(definition["match"])
			mac := b.valueString(match["macaddress"])
			matched := id == item.Name || b.valueString(definition["set-name"]) == item.Name ||
				(mac != "" && strings.EqualFold(mac, item.MAC))
			if pattern := b.valueString(match["name"]); !matched && pattern != "" {
				matched, _ = filepath.Match(pattern, item.Name)
			}
			if matched {
				result = append(result, netplanMatch{group: group, id: id, definition: definition})
			}
		}
	}
	return result
}

func (b *netplanBackend) family(definition map[string]any, ipv4 bool, current []string) FamilyConfig {
	suffix := "6"
	if ipv4 {
		suffix = "4"
	}
	addresses := b.addresses(definition["addresses"], ipv4)
	automatic := b.valueBool(definition["dhcp"+suffix])
	if !ipv4 {
		automatic = automatic || b.valueBool(definition["accept-ra"])
		// accept-ra 未显式配置时默认开启，此时有地址说明来自路由器通告
		if _, configured := definition["accept-ra"]; !configured && len(addresses) == 0 && len(current) > 0 {
			automatic = true
		}
	}
	mode := modeDisabled
	switch {
	case automatic:
		mode = modeAuto
	case len(addresses) > 0:
		mode = modeManual
	}
	autoDNS := true
	if value, ok := b.valueMap(definition["dhcp"+suffix+"-overrides"])["use-dns"]; ok {
		autoDNS = b.valueBool(value)
	}
	return FamilyConfig{
		Mode:      mode,
		Addresses: addresses,
		Gateway:   b.gateway(definition, ipv4),
		AutoDNS:   autoDNS,
		DNS:       b.addresses(b.valueMap(definition["nameservers"])["addresses"], ipv4),
	}
}

// routes 保留非默认路由，用配置中的网关重建默认路由
func (b *netplanBackend) routes(raw any, config Config) []any {
	result := make([]any, 0)
	for _, current := range b.valueSlice(raw) {
		if !b.isDefaultRoute(b.valueString(b.valueMap(current)["to"])) {
			result = append(result, current)
		}
	}
	for _, gateway := range []string{config.IPv4.Gateway, config.IPv6.Gateway} {
		if gateway != "" {
			result = append(result, map[string]any{"to": "default", "via": gateway})
		}
	}
	return result
}

func (b *netplanBackend) gateway(definition map[string]any, ipv4 bool) string {
	key := "gateway6"
	if ipv4 {
		key = "gateway4"
	}
	if gateway := b.valueString(definition[key]); gateway != "" {
		return gateway
	}
	for _, current := range b.valueSlice(definition["routes"]) {
		route := b.valueMap(current)
		gateway := b.valueString(route["via"])
		address, err := netip.ParseAddr(gateway)
		if b.isDefaultRoute(b.valueString(route["to"])) && err == nil && address.Is4() == ipv4 {
			return gateway
		}
	}
	return ""
}

func (b *netplanBackend) addresses(raw any, ipv4 bool) []string {
	result := make([]string, 0)
	for _, current := range b.valueSlice(raw) {
		value := fmt.Sprint(current)
		if prefix, err := netip.ParsePrefix(value); err == nil && prefix.Addr().Is4() == ipv4 {
			result = append(result, value)
		} else if address, err := netip.ParseAddr(value); err == nil && address.Is4() == ipv4 {
			result = append(result, value)
		}
	}
	return unique(result)
}

func (b *netplanBackend) isDefaultRoute(to string) bool {
	return to == "default" || to == "0.0.0.0/0" || to == "::/0"
}

func (b *netplanBackend) valueMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	if result == nil {
		result = make(map[string]any)
	}
	return result
}

func (b *netplanBackend) valueSlice(value any) []any {
	switch current := value.(type) {
	case []any:
		return current
	case []string:
		result := make([]any, len(current))
		for i := range current {
			result[i] = current[i]
		}
		return result
	default:
		return nil
	}
}

func (b *netplanBackend) valueString(value any) string {
	result, _ := value.(string)
	return result
}

func (b *netplanBackend) valueBool(value any) bool {
	result, _ := strconv.ParseBool(fmt.Sprint(value))
	return result
}

func (b *netplanBackend) valueInt(value any) int {
	result, _ := strconv.Atoi(fmt.Sprint(value))
	return result
}
