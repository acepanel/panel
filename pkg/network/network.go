package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/acepanel/panel/v3/pkg/shell"
)

const (
	ModeAuto     = "auto"
	ModeManual   = "manual"
	ModeDisabled = "disabled"

	// ConfirmTimeout 变更后等待确认的时长，超时未确认自动回滚，
	// 避免配错导致远程失联后无法恢复
	ConfirmTimeout = 30 * time.Second

	// verifyTimeout 等待配置生效的上限，IPv6 重复地址检测默认约需 1～2 秒
	verifyTimeout = 10 * time.Second
)

// ErrValidation 配置不合法，服务层据此返回 422
var ErrValidation = errors.New("invalid network configuration")

type FamilyConfig struct {
	Mode      string   `json:"mode"`
	Addresses []string `json:"addresses"`
	Gateway   string   `json:"gateway"`
	AutoDNS   bool     `json:"auto_dns"`
	DNS       []string `json:"dns"`
}

type Config struct {
	Name string       `json:"name"`
	MTU  int          `json:"mtu"`
	IPv4 FamilyConfig `json:"ipv4"`
	IPv6 FamilyConfig `json:"ipv6"`
}

// FamilyState 网卡当前生效的网络状态，自动获取时配置里没有这些值，用于回填表单
type FamilyState struct {
	Addresses []string `json:"addresses"`
	Gateway   string   `json:"gateway"`
	DNS       []string `json:"dns"`
}

type Interface struct {
	Name          string       `json:"name"`
	Type          string       `json:"type"`
	State         string       `json:"state"`
	MAC           string       `json:"mac"`
	CurrentMTU    int          `json:"current_mtu"`
	ConfiguredMTU int          `json:"configured_mtu"`
	CurrentIPv4   FamilyState  `json:"current_ipv4"`
	CurrentIPv6   FamilyState  `json:"current_ipv6"`
	Editable      bool         `json:"editable"`
	Reason        string       `json:"reason"`
	IPv4          FamilyConfig `json:"ipv4"`
	IPv6          FamilyConfig `json:"ipv6"`
}

type Result struct {
	Manager string      `json:"manager"`
	Items   []Interface `json:"items"`
	// Pending 为真表示有变更等待确认
	Pending bool `json:"pending"`
}

// backend 封装不同网络管理器的配置读写
type backend interface {
	Name() string
	available(ctx context.Context) bool
	// Load 填充网卡的持久化配置，不可编辑时写入 Reason
	Load(ctx context.Context, items []Interface) error
	// Apply 应用配置，返回撤销本次变更的回滚函数
	Apply(ctx context.Context, item Interface, config Config) (func(context.Context) error, error)
}

// Service 网卡配置管理，同一时间只允许一个变更处于待确认状态
type Service struct {
	apply    sync.Mutex
	mu       sync.RWMutex
	backend  backend
	name     string
	rollback func(context.Context) error
	timer    *time.Timer
	onExpire func(name string, err error)
}

// New 创建网卡配置管理服务，onExpire 在超时自动回滚后调用，err 为回滚错误
func New(onExpire func(name string, err error)) *Service {
	return &Service{onExpire: onExpire}
}

func (s *Service) Interfaces(ctx context.Context) (*Result, error) {
	items, err := runtimeInterfaces()
	if err != nil {
		return nil, err
	}
	ipv4Gateways, ipv6Gateways := defaultGateways(ctx, true), defaultGateways(ctx, false)
	for i := range items {
		ipv4, ipv6 := currentDNS(ctx, items[i].Name)
		items[i].CurrentIPv4.Gateway, items[i].CurrentIPv4.DNS = ipv4Gateways[items[i].Name], ipv4
		items[i].CurrentIPv6.Gateway, items[i].CurrentIPv6.DNS = ipv6Gateways[items[i].Name], ipv6
	}

	current := s.detect(ctx)
	if current == nil {
		for i := range items {
			items[i].Reason = "unsupported network manager"
		}
		return &Result{Manager: "unsupported", Items: items}, nil
	}
	if err = current.Load(ctx, items); err != nil {
		return nil, err
	}
	return &Result{Manager: current.Name(), Items: items, Pending: s.pending()}, nil
}

// Update 应用网卡配置，成功后进入待确认状态，超时未确认自动回滚
func (s *Service) Update(ctx context.Context, config Config) error {
	if err := Validate(config); err != nil {
		return err
	}

	s.apply.Lock()
	defer s.apply.Unlock()
	if s.pending() {
		return fmt.Errorf("%w: a previous change is still waiting for confirmation", ErrValidation)
	}
	current := s.detect(ctx)
	if current == nil {
		return fmt.Errorf("%w: unsupported network manager", ErrValidation)
	}

	items, err := runtimeInterfaces()
	if err != nil {
		return err
	}
	index := slices.IndexFunc(items, func(item Interface) bool { return item.Name == config.Name })
	if index < 0 {
		return fmt.Errorf("%w: network interface does not exist", ErrValidation)
	}
	// 只加载目标网卡，避免为改一张网卡读遍全部配置
	target := items[index : index+1]
	if err = current.Load(ctx, target); err != nil {
		return err
	}
	if !target[0].Editable {
		return fmt.Errorf("%w: %s", ErrValidation, target[0].Reason)
	}

	rollback, err := current.Apply(ctx, target[0], config)
	if err != nil {
		return err
	}
	if err = verify(ctx, config); err != nil {
		return errors.Join(err, rollback(context.WithoutCancel(ctx)))
	}

	s.mu.Lock()
	s.name = config.Name
	s.rollback = rollback
	s.timer = time.AfterFunc(ConfirmTimeout, s.expire)
	s.mu.Unlock()
	return nil
}

func (s *Service) Confirm() error {
	if s.take() == nil {
		return fmt.Errorf("%w: no change is waiting for confirmation", ErrValidation)
	}
	return nil
}

func (s *Service) Rollback(ctx context.Context) error {
	s.apply.Lock()
	defer s.apply.Unlock()
	rollback := s.take()
	if rollback == nil {
		return fmt.Errorf("%w: no change is waiting for confirmation", ErrValidation)
	}
	return rollback(ctx)
}

// expire 等待确认超时，自动回滚
func (s *Service) expire() {
	s.apply.Lock()
	defer s.apply.Unlock()
	rollback := s.take()
	if rollback == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s.onExpire(s.name, rollback(ctx))
}

// pending 是否有变更等待确认
func (s *Service) pending() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rollback != nil
}

// take 取出待确认的回滚函数并清空状态，返回 nil 表示没有待确认变更
func (s *Service) take() func(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rollback := s.rollback
	if rollback != nil {
		s.timer.Stop()
		s.timer, s.rollback = nil, nil
	}
	return rollback
}

// detect 探测网络管理器，结果在进程生命周期内缓存
func (s *Service) detect(ctx context.Context) backend {
	s.mu.RLock()
	cached := s.backend
	s.mu.RUnlock()
	if cached != nil {
		return cached
	}

	var found backend
	for _, candidate := range []backend{&netplanBackend{}, &networkManagerBackend{}, &ifupdownBackend{}} {
		if candidate.available(ctx) {
			found = candidate
			break
		}
	}
	if found == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.backend == nil {
		s.backend = found
	}
	return s.backend
}

// verify 确认配置已在网卡上生效，挡住应用命令返回成功但实际未生效的情况。
// 网络管理器可能异步提交地址，IPv6 地址还要等重复地址检测结束，检测失败的地址挂在网卡上却无法使用，
// 因此轮询到全部就绪、确认冲突或超时为止
func verify(ctx context.Context, config Config) error {
	deadline := time.Now().Add(verifyTimeout)
	for {
		output, err := run(ctx, "ip", "-j", "address", "show", "dev", config.Name)
		if err == nil {
			err = checkApplied(config, output)
		}
		if err == nil || errors.Is(err, ErrValidation) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// checkApplied 根据 ip -j address 的输出检查网卡已启用且手动地址都已通过重复地址检测
func checkApplied(config Config, output string) error {
	var links []struct {
		Flags    []string `json:"flags"`
		AddrInfo []struct {
			Local     string `json:"local"`
			Prefixlen int    `json:"prefixlen"`
			Tentative bool   `json:"tentative"`
			DADFailed bool   `json:"dadfailed"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal([]byte(output), &links); err != nil || len(links) != 1 {
		return fmt.Errorf("failed to read addresses of %s: %s", config.Name, output)
	}
	if !slices.Contains(links[0].Flags, "UP") {
		return fmt.Errorf("network interface %s is down after applying", config.Name)
	}

	type state struct{ tentative, failed bool }
	applied := make(map[netip.Prefix]state, len(links[0].AddrInfo))
	for _, info := range links[0].AddrInfo {
		if address, err := netip.ParseAddr(info.Local); err == nil {
			applied[netip.PrefixFrom(address, info.Prefixlen)] = state{info.Tentative, info.DADFailed}
		}
	}
	for _, family := range []FamilyConfig{config.IPv4, config.IPv6} {
		if family.Mode != ModeManual {
			continue
		}
		for _, value := range family.Addresses {
			// 按解析结果比较，用户输入的大写或非压缩写法与内核的规范写法视为同一地址
			prefix, _ := netip.ParsePrefix(value)
			current, ok := applied[prefix]
			switch {
			case !ok:
				return fmt.Errorf("address %s was not applied to %s", value, config.Name)
			case current.failed:
				return fmt.Errorf("%w: address %s is already in use by another device on the network (duplicate address detection failed)", ErrValidation, value)
			case current.tentative:
				return fmt.Errorf("address %s did not pass duplicate address detection on %s, it may be in use by another device on the network", value, config.Name)
			}
		}
	}
	return nil
}

func Validate(config Config) error {
	if config.Name == "" || filepath.Base(config.Name) != config.Name || strings.ContainsAny(config.Name, " \t\r\n/") {
		return fmt.Errorf("%w: invalid network interface name", ErrValidation)
	}
	if config.MTU != 0 && (config.MTU < 68 || config.MTU > 65535) {
		return fmt.Errorf("%w: MTU must be 0 or between 68 and 65535", ErrValidation)
	}
	if config.IPv4.Mode == ModeDisabled && config.IPv6.Mode == ModeDisabled {
		return fmt.Errorf("%w: IPv4 and IPv6 cannot both be disabled", ErrValidation)
	}
	if config.IPv6.Mode != ModeDisabled && config.MTU != 0 && config.MTU < 1280 {
		return fmt.Errorf("%w: MTU must not be lower than 1280 when IPv6 is enabled", ErrValidation)
	}
	if err := validateFamily("IPv4", config.IPv4, true); err != nil {
		return err
	}
	return validateFamily("IPv6", config.IPv6, false)
}

func validateFamily(name string, config FamilyConfig, ipv4 bool) error {
	if !slices.Contains([]string{ModeAuto, ModeManual, ModeDisabled}, config.Mode) {
		return fmt.Errorf("%w: invalid %s mode", ErrValidation, name)
	}
	if config.Mode == ModeDisabled {
		if len(config.Addresses) > 0 || config.Gateway != "" || len(config.DNS) > 0 {
			return fmt.Errorf("%w: disabled %s configuration must be empty", ErrValidation, name)
		}
		return nil
	}
	if config.Mode == ModeManual && len(config.Addresses) == 0 {
		return fmt.Errorf("%w: manual %s configuration requires an address", ErrValidation, name)
	}

	addresses := make(map[netip.Prefix]struct{}, len(config.Addresses))
	for _, value := range config.Addresses {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix.Addr().Is4() != ipv4 {
			return fmt.Errorf("%w: invalid %s CIDR: %s", ErrValidation, name, value)
		}
		// 主机位全零的是网络地址，IPv6 下即子网路由器任播地址，由上游路由器占用，
		// 配到本机会因重复地址检测失败而不可用。/31、/127 点对点链路与单地址前缀不受此限
		if prefix.Bits() < prefix.Addr().BitLen()-1 && prefix.Masked().Addr() == prefix.Addr() {
			return fmt.Errorf("%w: %s address %s is a subnet network address and cannot be assigned to a host", ErrValidation, name, value)
		}
		if _, exists := addresses[prefix]; exists {
			return fmt.Errorf("%w: duplicate %s address: %s", ErrValidation, name, value)
		}
		addresses[prefix] = struct{}{}
	}
	if config.Gateway != "" {
		gateway, err := netip.ParseAddr(config.Gateway)
		if err != nil || gateway.Is4() != ipv4 {
			return fmt.Errorf("%w: invalid %s gateway", ErrValidation, name)
		}
	}
	dns := make(map[netip.Addr]struct{}, len(config.DNS))
	for _, value := range config.DNS {
		server, err := netip.ParseAddr(value)
		if err != nil || server.Is4() != ipv4 {
			return fmt.Errorf("%w: invalid %s DNS: %s", ErrValidation, name, value)
		}
		if _, exists := dns[server]; exists {
			return fmt.Errorf("%w: duplicate %s DNS: %s", ErrValidation, name, value)
		}
		dns[server] = struct{}{}
	}
	return nil
}

// runtimeInterfaces 列出内核中可管理的物理网卡
func runtimeInterfaces() ([]Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	items := make([]Interface, 0, len(interfaces))
	for _, current := range interfaces {
		kind := interfaceType(current.Name)
		if kind == "" {
			continue
		}
		state := "down"
		if current.Flags&net.FlagUp != 0 {
			state = "up"
		}
		ipv4, ipv6 := currentAddresses(current)
		items = append(items, Interface{
			Name: current.Name, Type: kind, State: state, MAC: current.HardwareAddr.String(),
			CurrentMTU:  current.MTU,
			CurrentIPv4: FamilyState{Addresses: ipv4},
			CurrentIPv6: FamilyState{Addresses: ipv6},
			IPv4:        emptyFamily(), IPv6: emptyFamily(),
		})
	}
	slices.SortFunc(items, func(a, b Interface) int { return strings.Compare(a.Name, b.Name) })
	return items, nil
}

// interfaceType 识别网卡类型，容器与隧道等虚拟设备不纳入管理
func interfaceType(name string) string {
	base := filepath.Join("/sys/class/net", name)
	switch {
	case name == "lo":
		return ""
	case exists(filepath.Join(base, "bonding")):
		return "bond"
	case exists(filepath.Join(base, "bridge")):
		return "bridge"
	case hasVLANParent(base):
		return "vlan"
	case !exists(filepath.Join(base, "device")):
		// 无对应硬件设备即为虚拟网卡（docker0、veth、tun 等）
		return ""
	case exists(filepath.Join(base, "wireless")), exists(filepath.Join(base, "phy80211")):
		return "wifi"
	default:
		return "ethernet"
	}
}

func currentAddresses(current net.Interface) ([]string, []string) {
	addresses, _ := current.Addrs()
	ipv4, ipv6 := make([]string, 0), make([]string, 0)
	for _, address := range addresses {
		prefix, err := netip.ParsePrefix(address.String())
		if err != nil || prefix.Addr().IsUnspecified() {
			continue
		}
		if prefix.Addr().Is4() {
			ipv4 = append(ipv4, address.String())
		} else if !prefix.Addr().IsLinkLocalUnicast() {
			ipv6 = append(ipv6, address.String())
		}
	}
	return unique(ipv4), unique(ipv6)
}

func emptyFamily() FamilyConfig {
	return FamilyConfig{Mode: ModeDisabled, Addresses: []string{}, AutoDNS: true, DNS: []string{}}
}

func unique(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	slices.Sort(result)
	return result
}

// currentDNS 读取网卡当前生效的 DNS，按地址族分开返回。
// systemd-resolved 按网卡维护 DNS，此时 resolv.conf 内只有本机 stub 地址
func currentDNS(ctx context.Context, name string) ([]string, []string) {
	var servers []string
	if output, err := run(ctx, "resolvectl", "dns", name); err == nil {
		// 输出形如 Link 2 (ens5): 183.60.83.19 183.60.82.98
		if _, values, ok := strings.Cut(output, ":"); ok {
			servers = strings.Fields(values)
		}
	} else if content, readErr := os.ReadFile("/etc/resolv.conf"); readErr == nil {
		for line := range strings.SplitSeq(string(content), "\n") {
			if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "nameserver" {
				servers = append(servers, fields[1])
			}
		}
	}

	ipv4, ipv6 := make([]string, 0), make([]string, 0)
	for _, server := range servers {
		address, err := netip.ParseAddr(server)
		// 本机 stub 解析器不是真实上游，回填无意义
		if err != nil || address.IsLoopback() {
			continue
		}
		if address.Is4() {
			ipv4 = append(ipv4, server)
		} else {
			ipv6 = append(ipv6, server)
		}
	}
	return ipv4, ipv6
}

// defaultGateways 读取各网卡当前生效的默认网关
func defaultGateways(ctx context.Context, ipv4 bool) map[string]string {
	family := "-6"
	if ipv4 {
		family = "-4"
	}
	output, err := run(ctx, "ip", family, "route", "show", "default")
	if err != nil {
		return nil
	}

	gateways := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		var gateway, device string
		for i := 0; i+1 < len(fields); i++ {
			switch fields[i] {
			case "via":
				gateway = fields[i+1]
			case "dev":
				device = fields[i+1]
			}
		}
		// 同一网卡可能有多条默认路由，取指标最优的第一条
		if gateway != "" && device != "" && gateways[device] == "" {
			gateways[device] = gateway
		}
	}
	return gateways
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := shell.Command(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hasVLANParent VLAN 在 sysfs 下有指向父设备的 lower_* 链接，
// 而 bond 与 bridge 已在前面按各自目录识别
func hasVLANParent(base string) bool {
	matches, _ := filepath.Glob(filepath.Join(base, "lower_*"))
	return len(matches) > 0
}
