package firewall

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/samber/lo"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// Firewall 防火墙统一接口
type Firewall interface {
	backend
	// Name 当前使用的防火墙软件名，未检测到支持的防火墙时为空
	Name() string
}

// backend 各防火墙软件需实现的操作
// 变更方法由 NewFirewall 返回的实现统一加锁并断开取消链，列表也由它统一排序、合并协议，各后端实现内无需再处理
type backend interface {
	// Status 获取防火墙运行状态
	Status(ctx context.Context) (bool, error)
	// Enable 启用防火墙
	Enable(ctx context.Context) error
	// Disable 禁用防火墙
	Disable(ctx context.Context) error

	// ListRule 列出所有规则
	ListRule(ctx context.Context) ([]FireInfo, error)
	// Port 添加/删除端口规则
	Port(ctx context.Context, rule FireInfo, operation Operation) error
	// RichRules 添加/删除富规则（IP/高级规则）
	RichRules(ctx context.Context, rule FireInfo, operation Operation) error
	// RichRulesBatch 批量添加/删除富规则，整批只重载一次
	// 返回错误时可能已有部分规则生效，调用方可用相反的操作撤回
	RichRulesBatch(ctx context.Context, rules []FireInfo, operation Operation) error

	// ListForward 列出所有转发规则
	ListForward(ctx context.Context) ([]FireForwardInfo, error)
	// Forward 添加/删除转发规则
	Forward(ctx context.Context, rule Forward, operation Operation) error

	// PingStatus 获取 Ping 状态（true 为允许）
	PingStatus(ctx context.Context) (bool, error)
	// UpdatePingStatus 更新 Ping 状态
	UpdatePingStatus(ctx context.Context, status bool) error
}

// mu 串行化所有防火墙变更操作
// ufw 在加锁前就已将 user.rules 读入内存，并发执行时后写入的进程会覆盖先写入的规则且都返回成功
var mu sync.Mutex

var (
	firewalldBackend = newFirewalld()
	ufwBackend       = newUFW()
)

// detected 探测到的防火墙后端及其软件名
type detected struct {
	backend
	name string
}

// lockedFirewall 变更操作加锁并断开取消链，列表统一排序，其余读操作直接透传
// 每个变更都是「写永久配置 + reload」的组合，中途取消会让两者脱节：
// firewalld 下规则进了永久配置却没进 runtime，ufw 下面板读的规则文件已改而内核里的规则还在跑
type lockedFirewall struct{}

// NewFirewall 返回当前系统的防火墙实现
func NewFirewall() Firewall {
	return lockedFirewall{}
}

func (f lockedFirewall) Status(ctx context.Context) (bool, error) {
	return f.detectFirewall().Status(ctx)
}

func (f lockedFirewall) Enable(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().Enable(context.WithoutCancel(ctx))
}

func (f lockedFirewall) Disable(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().Disable(context.WithoutCancel(ctx))
}

func (f lockedFirewall) ListRule(ctx context.Context) ([]FireInfo, error) {
	rules, err := f.detectFirewall().ListRule(ctx)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(rules, func(a, b FireInfo) int {
		return cmp.Or(
			cmp.Compare(a.PortStart, b.PortStart),
			cmp.Compare(a.PortEnd, b.PortEnd),
			cmp.Compare(a.Protocol, b.Protocol),
			cmp.Compare(a.Family, b.Family),
			cmp.Compare(a.Strategy, b.Strategy),
			cmp.Compare(a.Direction, b.Direction),
			cmp.Compare(a.Type, b.Type),
		)
	})

	return f.mergeRules(rules), nil
}

func (f lockedFirewall) Port(ctx context.Context, rule FireInfo, operation Operation) error {
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().Port(context.WithoutCancel(ctx), rule, operation)
}

func (f lockedFirewall) RichRules(ctx context.Context, rule FireInfo, operation Operation) error {
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().RichRules(context.WithoutCancel(ctx), rule, operation)
}

func (f lockedFirewall) RichRulesBatch(ctx context.Context, rules []FireInfo, operation Operation) error {
	if len(rules) == 0 {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().RichRulesBatch(context.WithoutCancel(ctx), rules, operation)
}

func (f lockedFirewall) ListForward(ctx context.Context) ([]FireForwardInfo, error) {
	forwards, err := f.detectFirewall().ListForward(ctx)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(forwards, func(a, b FireForwardInfo) int {
		return cmp.Or(
			cmp.Compare(a.Port, b.Port),
			cmp.Compare(a.TargetPort, b.TargetPort),
			cmp.Compare(a.Protocol, b.Protocol),
			cmp.Compare(a.TargetIP, b.TargetIP),
		)
	})

	return forwards, nil
}

func (f lockedFirewall) Forward(ctx context.Context, rule Forward, operation Operation) error {
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().Forward(context.WithoutCancel(ctx), rule, operation)
}

func (f lockedFirewall) PingStatus(ctx context.Context) (bool, error) {
	return f.detectFirewall().PingStatus(ctx)
}

func (f lockedFirewall) UpdatePingStatus(ctx context.Context, status bool) error {
	mu.Lock()
	defer mu.Unlock()
	return f.detectFirewall().UpdatePingStatus(context.WithoutCancel(ctx), status)
}

func (f lockedFirewall) Name() string {
	return f.detectFirewall().name
}

// detectFirewall 只查命令是否存在，不执行
// firewall-cmd 连 --version 都要先连 D-Bus，firewalld 未运行时重试约 10 秒才失败，执行探测会卡住请求并误判为未安装
// 查 PATH 足够便宜，每次都重新探测，装卸防火墙后无需重启面板
func (lockedFirewall) detectFirewall() detected {
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		return detected{firewalldBackend, "firewalld"}
	}
	if _, err := exec.LookPath("ufw"); err == nil {
		return detected{ufwBackend, "ufw"}
	}
	// 都没装时按 firewalld 处理，名字留空供上层提示未安装
	return detected{firewalldBackend, ""}
}

// mergeRules 将同端口/同地址/同策略/同方向、仅协议不同（tcp vs udp）的规则合并为 tcp/udp
func (lockedFirewall) mergeRules(rules []FireInfo) []FireInfo {
	grouped := make(map[string]*FireInfo)
	var order []string

	for i := range rules {
		r := rules[i]
		key := fmt.Sprintf("%s|%s|%s|%d|%d|%s|%s",
			r.Type, r.Family, r.Address, r.PortStart, r.PortEnd, r.Strategy, r.Direction)

		if existing, ok := grouped[key]; ok {
			// 合并协议：tcp + udp → tcp/udp
			if existing.Protocol != r.Protocol {
				existing.Protocol = ProtocolTCPUDP
			}
		} else {
			grouped[key] = new(r)
			order = append(order, key)
		}
	}

	return lo.Map(order, func(key string, _ int) FireInfo {
		return *grouped[key]
	})
}

// enableIPForward 打开内核转发并持久化
func enableIPForward(ctx context.Context) {
	_, _ = shell.Execf(ctx, "sysctl -w net.ipv4.ip_forward=1")
	_, _ = shell.Execf(ctx, "sysctl -w net.ipv6.conf.all.forwarding=1")
	_ = os.WriteFile("/etc/sysctl.d/99-acepanel-forward.conf", []byte("net.ipv4.ip_forward=1\nnet.ipv6.conf.all.forwarding=1\n"), 0644)
}

// buildProtocols 拆分协议字符串为列表
func buildProtocols(protocol Protocol) []string {
	return strings.Split(string(protocol), "/")
}
