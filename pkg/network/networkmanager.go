package network

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type networkManagerBackend struct{}

func (b *networkManagerBackend) Name() string { return "networkmanager" }

func (b *networkManagerBackend) available(ctx context.Context) bool {
	if !hasCommand("nmcli") {
		return false
	}
	output, err := run(ctx, "nmcli", "-t", "-f", "RUNNING", "general", "status")
	return err == nil && strings.EqualFold(output, "running")
}

func (b *networkManagerBackend) Load(ctx context.Context, items []Interface) error {
	// profileFields 一次取回的全部属性，逐个查询每张网卡要启动十几次 nmcli
	const profileFields = "ipv4.method,ipv4.addresses,ipv4.gateway,ipv4.dns,ipv4.ignore-auto-dns," +
		"ipv6.method,ipv6.addresses,ipv6.gateway,ipv6.dns,ipv6.ignore-auto-dns,"

	for i := range items {
		uuid, err := b.uuid(ctx, items[i].Name)
		if err != nil {
			items[i].Reason = "no active NetworkManager connection profile"
			continue
		}
		output, err := run(ctx, "nmcli", "-t", "-f", profileFields+b.mtuProperty(items[i].Type), "connection", "show", "uuid", uuid)
		if err != nil {
			items[i].Reason = "failed to read NetworkManager connection profile"
			continue
		}
		items[i].IPv4, items[i].IPv6, items[i].ConfiguredMTU = b.parse(output, items[i].Type)
		items[i].Editable = true
	}
	return nil
}

func (b *networkManagerBackend) Apply(
	ctx context.Context,
	item Interface,
	config Config,
) (func(context.Context) error, error) {
	uuid, err := b.uuid(ctx, config.Name)
	if err != nil {
		return nil, fmt.Errorf("%w: no active NetworkManager connection profile", ErrValidation)
	}

	// 连接配置由 nmcli 直接改写，改回读取到的旧值即可撤销
	previous := Config{Name: item.Name, MTU: item.ConfiguredMTU, IPv4: item.IPv4, IPv6: item.IPv6}
	rollback := func(ctx context.Context) error { return b.modify(ctx, uuid, item.Type, previous) }
	if err = b.modify(ctx, uuid, item.Type, config); err != nil {
		return nil, err
	}
	return rollback, nil
}

func (b *networkManagerBackend) uuid(ctx context.Context, name string) (string, error) {
	uuid, err := run(ctx, "nmcli", "-g", "GENERAL.CON-UUID", "device", "show", name)
	if err != nil {
		return "", err
	}
	if uuid == "" {
		return "", fmt.Errorf("interface %s has no connection profile", name)
	}
	return uuid, nil
}

// parse 解析 nmcli -t -f 的输出，每行一个「属性:值」，值内的冒号不转义。
// 配置里没有的设置段整行不输出（如 dummy、loopback 没有 802-3-ethernet），所以按属性名而不是行序取值
func (b *networkManagerBackend) parse(output, kind string) (FamilyConfig, FamilyConfig, int) {
	values := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		if key, value, ok := strings.Cut(line, ":"); ok {
			values[key] = value
		}
	}
	// 未设置 MTU 时返回 auto，解析失败即视为未配置
	mtu, _ := strconv.Atoi(values[b.mtuProperty(kind)])
	return b.family(values, "ipv4"), b.family(values, "ipv6"), mtu
}

func (b *networkManagerBackend) family(values map[string]string, name string) FamilyConfig {
	mode := modeDisabled
	switch values[name+".method"] {
	case "auto", "dhcp":
		mode = modeAuto
	case "manual":
		mode = modeManual
	}
	return FamilyConfig{
		Mode:      mode,
		Addresses: unique(strings.Split(values[name+".addresses"], ",")),
		Gateway:   values[name+".gateway"],
		AutoDNS:   values[name+".ignore-auto-dns"] != "yes",
		DNS:       unique(strings.Split(values[name+".dns"], ",")),
	}
}

func (b *networkManagerBackend) modify(ctx context.Context, uuid, kind string, config Config) error {
	mtu := "auto"
	if config.MTU > 0 {
		mtu = strconv.Itoa(config.MTU)
	}
	args := []string{"connection", "modify", "uuid", uuid}
	args = append(args, b.familyArgs("ipv4", config.IPv4)...)
	args = append(args, b.familyArgs("ipv6", config.IPv6)...)
	args = append(args, b.mtuProperty(kind), mtu)
	if _, err := run(ctx, "nmcli", args...); err != nil {
		return err
	}

	// reapply 可在不断链的情况下生效，不支持时回退到重新激活连接
	if _, err := run(ctx, "nmcli", "device", "reapply", config.Name); err == nil {
		return nil
	}
	_, err := run(ctx, "nmcli", "connection", "up", "uuid", uuid, "ifname", config.Name)
	return err
}

func (b *networkManagerBackend) familyArgs(name string, config FamilyConfig) []string {
	ignoreAutoDNS := "yes"
	if config.AutoDNS {
		ignoreAutoDNS = "no"
	}
	return []string{
		name + ".method", config.Mode,
		name + ".addresses", strings.Join(config.Addresses, ","),
		name + ".gateway", config.Gateway,
		name + ".dns", strings.Join(config.DNS, ","),
		name + ".ignore-auto-dns", ignoreAutoDNS,
	}
}

// mtuProperty MTU 属于具体链路层设置，nmcli 只接受带设置名的完整属性
func (b *networkManagerBackend) mtuProperty(kind string) string {
	if kind == "wifi" {
		return "802-11-wireless.mtu"
	}
	return "802-3-ethernet.mtu"
}
