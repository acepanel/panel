//go:build linux

package scan

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

const (
	ringBufSize = 256 * 1024 // 256KB
	eventSize   = 20         // src_ip(16) + dst_port(2) + protocol(1) + version(1)
)

// __sk_buff 字段偏移量
const (
	skbData    = 76 // __sk_buff.data
	skbDataEnd = 80 // __sk_buff.data_end
)

// 协议头长度
const (
	ethLen  = 14
	ipLen   = 20 // IPv4 最小头长度
	ipv6Len = 40 // IPv6 固定头长度
	tcpLen  = 20 // TCP 最小头长度
	udpLen  = 8  // UDP 头长度
)

// EtherType（标准数值，配合 HostTo(BE) 使用）
const (
	ethIPv4   = 0x0800 // ETH_P_IP
	ethIPv6   = 0x86DD // ETH_P_IPV6
	ethVLAN   = 0x8100 // ETH_P_8021Q
	ethQinQ   = 0x88A8 // ETH_P_8021AD
	ethVLANQ  = 0x9100 // 兼容部分设备上的外层 VLAN
	maxV6Exts = 8
)

// 协议常量
const (
	ipTCP   = 6
	ipUDP   = 17
	synOnly = 0x02 // 仅 SYN
)

// IPv6 扩展头协议号
const (
	ipv6HopByHop = 0
	ipv6Routing  = 43
	ipv6Fragment = 44
	ipv6AH       = 51
	ipv6DestOpts = 60
)

// IPv6 字段偏移量（从 IPv6 头起始）
const (
	offV6NextHdr = 6 // Next Header
	offV6SrcIP   = 8 // 源地址，16 字节
)

// ifaceHandle 单个网卡的 eBPF 挂载句柄
type ifaceHandle struct {
	link link.Link
}

// knownServicePorts 已知服务端口列表
// 当 UDP 包的源端口匹配这些端口时，视为合法回包而非扫描
// 例如 DNS 响应（源端口 53）、NTP 响应（源端口 123）等
var knownServicePorts = []uint16{
	53,  // DNS
	123, // NTP
	67,  // DHCP 服务端
	68,  // DHCP 客户端
}

// Scanner eBPF 扫描检测器
type Scanner struct {
	prog         *ebpf.Program
	events       *ebpf.Map
	ports        *ebpf.Map // 监听端口白名单（hash map，命中则跳过）
	skipSrcPorts *ebpf.Map // 已知服务源端口白名单，用于 UDP 回包过滤
	handles      map[string]*ifaceHandle
	reader       *ringbuf.Reader
	eventsCh     chan Event
	stopCh       chan struct{}
	mu           sync.Mutex
	log          *slog.Logger
	lastPorts    map[uint16]bool
}

// newPortsMap 创建监听端口白名单 BPF map
func newPortsMap() (*ebpf.Map, error) {
	return ebpf.NewMap(&ebpf.MapSpec{
		Type:       ebpf.Hash,
		KeySize:    4, // uint32
		ValueSize:  1, // uint8
		MaxEntries: 65536,
	})
}

// buildDetector eBPF TC ingress 扫描检测程序
// 只捕获目标端口不在 ports 白名单中的 SYN/UDP 包
// 对于 UDP，还会过滤源端口在 skipSrcPorts 中的回包（如 DNS/NTP 响应）
func buildDetector(events, ports, skipSrcPorts *ebpf.Map) (*ebpf.Program, error) {
	emitEvent := func(proto int64) asm.Instructions {
		return asm.Instructions{
			// 查询端口白名单
			asm.StoreMem(asm.RFP, -24, asm.R9, asm.Word),
			asm.LoadMapPtr(asm.R1, ports.FD()),
			asm.Mov.Reg(asm.R2, asm.RFP),
			asm.Add.Imm(asm.R2, -24),
			asm.FnMapLookupElem.Call(),
			asm.JNE.Imm(asm.R0, 0, "exit"),

			// 写入事件
			asm.StoreMem(asm.RFP, -4, asm.R9, asm.Half),
			asm.StoreImm(asm.RFP, -2, proto, asm.Byte),

			asm.LoadMapPtr(asm.R1, events.FD()),
			asm.Mov.Reg(asm.R2, asm.RFP),
			asm.Add.Imm(asm.R2, -eventSize),
			asm.Mov.Imm(asm.R3, eventSize),
			asm.Mov.Imm(asm.R4, 0),
			asm.FnRingbufOutput.Call(),
			asm.Ja.Label("exit"),
		}
	}

	// TCP 处理器：边界检查 → SYN 过滤 → 端口白名单 → 输出事件
	tcpHandler := func(sym string) asm.Instructions {
		insns := asm.Instructions{
			asm.Mov.Reg(asm.R2, asm.R4).WithSymbol(sym),
			asm.Add.Imm(asm.R2, tcpLen),
			asm.JGT.Reg(asm.R2, asm.R7, "exit"),

			// 仅 SYN（忽略 ECE/CWR，禁止 ACK/RST/FIN 等控制位）
			asm.LoadMem(asm.R2, asm.R4, 13, asm.Byte),
			asm.And.Imm(asm.R2, 0x3f),
			asm.JNE.Imm(asm.R2, synOnly, "exit"),

			// 目标端口
			asm.LoadMem(asm.R2, asm.R4, 2, asm.Half),
			asm.HostTo(asm.BE, asm.R2, asm.Half),
			asm.Mov.Reg(asm.R9, asm.R2),
		}

		insns = append(insns, emitEvent(ipTCP)...)
		return insns
	}

	// UDP 处理器：边界检查 → 源端口过滤 → 端口白名单 → 输出事件
	udpHandler := func(sym string) asm.Instructions {
		insns := asm.Instructions{
			asm.Mov.Reg(asm.R2, asm.R4).WithSymbol(sym),
			asm.Add.Imm(asm.R2, udpLen),
			asm.JGT.Reg(asm.R2, asm.R7, "exit"),

			// 保存 L4 头指针到 R9（R6=skb.data R7=skb.data_end R8=协议号 均已占用）
			asm.Mov.Reg(asm.R9, asm.R4),

			// 读取源端口（UDP 头偏移 0），检查是否为已知服务回包
			asm.LoadMem(asm.R2, asm.R4, 0, asm.Half),
			asm.HostTo(asm.BE, asm.R2, asm.Half),
			asm.StoreMem(asm.RFP, -24, asm.R2, asm.Word),
			asm.LoadMapPtr(asm.R1, skipSrcPorts.FD()),
			asm.Mov.Reg(asm.R2, asm.RFP),
			asm.Add.Imm(asm.R2, -24),
			asm.FnMapLookupElem.Call(),
			asm.JNE.Imm(asm.R0, 0, "exit"), // 命中已知服务端口 → 跳过

			// 恢复 L4 头指针，读取目标端口
			asm.Mov.Reg(asm.R4, asm.R9),
			asm.LoadMem(asm.R2, asm.R4, 2, asm.Half),
			asm.HostTo(asm.BE, asm.R2, asm.Half),
			asm.Mov.Reg(asm.R9, asm.R2),
		}

		insns = append(insns, emitEvent(ipUDP)...)
		return insns
	}

	// IPv6 扩展头解析步骤（R4=当前头指针，R8=NextHeader）
	ipv6ExtStep := func(sym, next, dispatch string) asm.Instructions {
		generic := fmt.Sprintf("%s_generic", sym)
		fragment := fmt.Sprintf("%s_fragment", sym)
		ah := fmt.Sprintf("%s_ah", sym)

		return asm.Instructions{
			asm.Mov.Reg(asm.R2, asm.R8).WithSymbol(sym),
			asm.JEq.Imm(asm.R2, ipv6HopByHop, generic),
			asm.JEq.Imm(asm.R2, ipv6Routing, generic),
			asm.JEq.Imm(asm.R2, ipv6DestOpts, generic),
			asm.JEq.Imm(asm.R2, ipv6Fragment, fragment),
			asm.JEq.Imm(asm.R2, ipv6AH, ah),
			asm.Ja.Label(dispatch),

			// Hop-by-Hop / Routing / Destination Options
			asm.Mov.Reg(asm.R9, asm.R4).WithSymbol(generic),
			asm.Add.Imm(asm.R9, 2),
			asm.JGT.Reg(asm.R9, asm.R7, "exit"),
			asm.LoadMem(asm.R2, asm.R4, 0, asm.Byte), // Next Header
			asm.LoadMem(asm.R3, asm.R4, 1, asm.Byte), // Hdr Ext Len
			asm.LSh.Imm(asm.R3, 3),
			asm.Add.Imm(asm.R3, 8),
			asm.Mov.Reg(asm.R9, asm.R4),
			asm.Add.Reg(asm.R9, asm.R3),
			asm.JGT.Reg(asm.R9, asm.R7, "exit"),
			asm.Mov.Reg(asm.R8, asm.R2),
			asm.Mov.Reg(asm.R4, asm.R9),
			asm.Ja.Label(next),

			// Fragment header：仅接受首片
			asm.Mov.Reg(asm.R9, asm.R4).WithSymbol(fragment),
			asm.Add.Imm(asm.R9, 8),
			asm.JGT.Reg(asm.R9, asm.R7, "exit"),
			asm.LoadMem(asm.R2, asm.R4, 0, asm.Byte), // Next Header
			asm.LoadMem(asm.R3, asm.R4, 2, asm.Half),
			asm.HostTo(asm.BE, asm.R3, asm.Half),
			asm.And.Imm(asm.R3, 0xfff8), // Fragment Offset 非 0 直接跳过
			asm.JNE.Imm(asm.R3, 0, "exit"),
			asm.Mov.Reg(asm.R8, asm.R2),
			asm.Mov.Reg(asm.R4, asm.R9),
			asm.Ja.Label(next),

			// Authentication Header
			asm.Mov.Reg(asm.R9, asm.R4).WithSymbol(ah),
			asm.Add.Imm(asm.R9, 2),
			asm.JGT.Reg(asm.R9, asm.R7, "exit"),
			asm.LoadMem(asm.R2, asm.R4, 0, asm.Byte), // Next Header
			asm.LoadMem(asm.R3, asm.R4, 1, asm.Byte), // Payload Len
			asm.Add.Imm(asm.R3, 2),
			asm.LSh.Imm(asm.R3, 2),
			asm.Mov.Reg(asm.R9, asm.R4),
			asm.Add.Reg(asm.R9, asm.R3),
			asm.JGT.Reg(asm.R9, asm.R7, "exit"),
			asm.Mov.Reg(asm.R8, asm.R2),
			asm.Mov.Reg(asm.R4, asm.R9),
			asm.Ja.Label(next),
		}
	}

	var insns asm.Instructions

	// 加载 skb->data / skb->data_end
	insns = append(insns,
		asm.LoadMem(asm.R6, asm.R1, skbData, asm.Word),
		asm.LoadMem(asm.R7, asm.R1, skbDataEnd, asm.Word),
	)

	// 以太网头边界检查
	insns = append(insns,
		asm.Mov.Reg(asm.R2, asm.R6),
		asm.Add.Imm(asm.R2, ethLen),
		asm.JGT.Reg(asm.R2, asm.R7, "exit"),
	)

	// 读取 EtherType（R0）并处理 VLAN
	insns = append(insns,
		asm.LoadMem(asm.R0, asm.R6, 12, asm.Half),
		asm.HostTo(asm.BE, asm.R0, asm.Half),

		asm.JEq.Imm(asm.R0, ethIPv4, "no_vlan"),
		asm.JEq.Imm(asm.R0, ethIPv6, "no_vlan"),
		asm.JEq.Imm(asm.R0, ethVLAN, "vlan1"),
		asm.JEq.Imm(asm.R0, ethQinQ, "vlan1"),
		asm.JEq.Imm(asm.R0, ethVLANQ, "vlan1"),
		asm.Ja.Label("exit"),

		asm.Mov.Reg(asm.R5, asm.R6).WithSymbol("no_vlan"),
		asm.Add.Imm(asm.R5, ethLen),
		asm.Ja.Label("l3dispatch"),

		// 第一层 VLAN
		asm.Mov.Reg(asm.R2, asm.R6).WithSymbol("vlan1"),
		asm.Add.Imm(asm.R2, ethLen+4),
		asm.JGT.Reg(asm.R2, asm.R7, "exit"),
		asm.LoadMem(asm.R0, asm.R6, 16, asm.Half),
		asm.HostTo(asm.BE, asm.R0, asm.Half),
		asm.JEq.Imm(asm.R0, ethVLAN, "vlan2"),
		asm.JEq.Imm(asm.R0, ethQinQ, "vlan2"),
		asm.JEq.Imm(asm.R0, ethVLANQ, "vlan2"),
		asm.Mov.Reg(asm.R5, asm.R6),
		asm.Add.Imm(asm.R5, ethLen+4),
		asm.Ja.Label("l3dispatch"),

		// 第二层 VLAN（QinQ）
		asm.Mov.Reg(asm.R2, asm.R6).WithSymbol("vlan2"),
		asm.Add.Imm(asm.R2, ethLen+8),
		asm.JGT.Reg(asm.R2, asm.R7, "exit"),
		asm.LoadMem(asm.R0, asm.R6, 20, asm.Half),
		asm.HostTo(asm.BE, asm.R0, asm.Half),
		asm.Mov.Reg(asm.R5, asm.R6),
		asm.Add.Imm(asm.R5, ethLen+8),
	)

	// L3 分发（R0=EtherType，R5=L3 起始）
	insns = append(insns,
		asm.JEq.Imm(asm.R0, ethIPv4, "ipv4").WithSymbol("l3dispatch"),
		asm.JEq.Imm(asm.R0, ethIPv6, "ipv6"),
		asm.Ja.Label("exit"),
	)

	// ========== IPv4 ==========
	insns = append(insns,
		// 基础头
		asm.Mov.Reg(asm.R2, asm.R5).WithSymbol("ipv4"),
		asm.Add.Imm(asm.R2, ipLen),
		asm.JGT.Reg(asm.R2, asm.R7, "exit"),

		// Version + IHL
		asm.LoadMem(asm.R2, asm.R5, 0, asm.Byte),
		asm.Mov.Reg(asm.R3, asm.R2),
		asm.RSh.Imm(asm.R3, 4),
		asm.JNE.Imm(asm.R3, 4, "exit"),
		asm.And.Imm(asm.R2, 0x0f),
		asm.LSh.Imm(asm.R2, 2), // IHL * 4
		asm.JLT.Imm(asm.R2, ipLen, "exit"),
		asm.Mov.Reg(asm.R3, asm.R5),
		asm.Add.Reg(asm.R3, asm.R2),
		asm.JGT.Reg(asm.R3, asm.R7, "exit"),

		// 分片：只处理 offset=0
		asm.LoadMem(asm.R3, asm.R5, 6, asm.Half),
		asm.HostTo(asm.BE, asm.R3, asm.Half),
		asm.And.Imm(asm.R3, 0x1fff),
		asm.JNE.Imm(asm.R3, 0, "exit"),

		// 协议 + 源 IP
		asm.LoadMem(asm.R8, asm.R5, 9, asm.Byte),
		asm.LoadMem(asm.R9, asm.R5, 12, asm.Word),

		// 栈布局：[src_ip(16) | port(2) | proto(1) | ver(1)]
		asm.StoreMem(asm.RFP, -20, asm.R9, asm.Word),
		asm.StoreImm(asm.RFP, -16, 0, asm.Word),
		asm.StoreImm(asm.RFP, -12, 0, asm.Word),
		asm.StoreImm(asm.RFP, -8, 0, asm.Word),
		asm.StoreImm(asm.RFP, -1, 4, asm.Byte),

		// R4 指向 L4 头
		asm.Mov.Reg(asm.R4, asm.R5),
		asm.Add.Reg(asm.R4, asm.R2),

		asm.JEq.Imm(asm.R8, ipTCP, "tcp"),
		asm.JEq.Imm(asm.R8, ipUDP, "udp"),
		asm.Ja.Label("exit"),
	)

	// ========== IPv6 ==========
	insns = append(insns,
		asm.Mov.Reg(asm.R2, asm.R5).WithSymbol("ipv6"),
		asm.Add.Imm(asm.R2, ipv6Len),
		asm.JGT.Reg(asm.R2, asm.R7, "exit"),

		// 版本检查
		asm.LoadMem(asm.R2, asm.R5, 0, asm.Byte),
		asm.RSh.Imm(asm.R2, 4),
		asm.JNE.Imm(asm.R2, 6, "exit"),

		asm.LoadMem(asm.R8, asm.R5, offV6NextHdr, asm.Byte),

		// 源地址 16 字节
		asm.LoadMem(asm.R2, asm.R5, offV6SrcIP, asm.Word),
		asm.StoreMem(asm.RFP, -20, asm.R2, asm.Word),
		asm.LoadMem(asm.R2, asm.R5, offV6SrcIP+4, asm.Word),
		asm.StoreMem(asm.RFP, -16, asm.R2, asm.Word),
		asm.LoadMem(asm.R2, asm.R5, offV6SrcIP+8, asm.Word),
		asm.StoreMem(asm.RFP, -12, asm.R2, asm.Word),
		asm.LoadMem(asm.R2, asm.R5, offV6SrcIP+12, asm.Word),
		asm.StoreMem(asm.RFP, -8, asm.R2, asm.Word),
		asm.StoreImm(asm.RFP, -1, 6, asm.Byte),

		// R4 指向 IPv6 负载起始
		asm.Mov.Reg(asm.R4, asm.R5),
		asm.Add.Imm(asm.R4, ipv6Len),
		asm.Ja.Label("v6ext_0"),
	)

	// 最多解析 maxV6Exts 层扩展头，避免 verifier 复杂循环
	for i := range maxV6Exts {
		start := fmt.Sprintf("v6ext_%d", i)
		next := "v6dispatch"
		if i < maxV6Exts-1 {
			next = fmt.Sprintf("v6ext_%d", i+1)
		}
		insns = append(insns, ipv6ExtStep(start, next, "v6dispatch")...)
	}

	insns = append(insns,
		asm.Mov.Reg(asm.R2, asm.R8).WithSymbol("v6dispatch"),
		asm.JEq.Imm(asm.R2, ipTCP, "tcp"),
		asm.JEq.Imm(asm.R2, ipUDP, "udp"),
		asm.Ja.Label("exit"),
	)

	insns = append(insns, tcpHandler("tcp")...)
	insns = append(insns, udpHandler("udp")...)

	// ========== 退出 ==========
	insns = append(insns,
		asm.Mov.Imm(asm.R0, 0).WithSymbol("exit"),
		asm.Return(),
	)

	return ebpf.NewProgram(&ebpf.ProgramSpec{
		Name:         "scan_detector",
		Type:         ebpf.SchedCLS,
		Instructions: insns,
	})
}

// Supported 检测当前系统是否支持 eBPF
func Supported() bool {
	events, err := ebpf.NewMap(&ebpf.MapSpec{
		Type:       ebpf.RingBuf,
		MaxEntries: ringBufSize,
	})
	if err != nil {
		return false
	}

	ports, err := newPortsMap()
	if err != nil {
		_ = events.Close()
		return false
	}

	skipSrcPorts, err := newPortsMap()
	if err != nil {
		_ = events.Close()
		_ = ports.Close()
		return false
	}

	prog, err := buildDetector(events, ports, skipSrcPorts)
	if err != nil {
		_ = events.Close()
		_ = ports.Close()
		_ = skipSrcPorts.Close()
		return false
	}

	_ = prog.Close()
	_ = events.Close()
	_ = ports.Close()
	_ = skipSrcPorts.Close()
	return true
}

// New 创建 Scanner，加载 eBPF 程序并挂载到指定网卡
func New(ifaces []string, log *slog.Logger) (*Scanner, error) {
	if len(ifaces) == 0 {
		defaultIface := DefaultInterface()
		if defaultIface == "" {
			return nil, errors.New("no available network interface found")
		}
		ifaces = []string{defaultIface}
	}

	events, err := ebpf.NewMap(&ebpf.MapSpec{
		Type:       ebpf.RingBuf,
		MaxEntries: ringBufSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ring buffer: %w", err)
	}

	ports, err := newPortsMap()
	if err != nil {
		_ = events.Close()
		return nil, fmt.Errorf("failed to create ports whitelist map: %w", err)
	}

	skipSrcPorts, err := newPortsMap()
	if err != nil {
		_ = events.Close()
		_ = ports.Close()
		return nil, fmt.Errorf("failed to create skip source ports map: %w", err)
	}

	// 填充已知服务端口，这些端口作为 UDP 源端口时视为回包
	for _, port := range knownServicePorts {
		if err = skipSrcPorts.Put(uint32(port), uint8(1)); err != nil {
			_ = events.Close()
			_ = ports.Close()
			_ = skipSrcPorts.Close()
			return nil, fmt.Errorf("failed to populate skip source ports map: %w", err)
		}
	}

	prog, err := buildDetector(events, ports, skipSrcPorts)
	if err != nil {
		_ = events.Close()
		_ = ports.Close()
		_ = skipSrcPorts.Close()
		return nil, fmt.Errorf("failed to load eBPF program: %w", err)
	}

	s := &Scanner{
		prog:         prog,
		events:       events,
		ports:        ports,
		skipSrcPorts: skipSrcPorts,
		handles:      make(map[string]*ifaceHandle),
		eventsCh:     make(chan Event, 4096),
		stopCh:       make(chan struct{}),
		log:          log,
		lastPorts:    make(map[uint16]bool),
	}

	for _, ifaceName := range ifaces {
		if err := s.attach(ifaceName); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("failed to attach to interface %s: %w", ifaceName, err)
		}
	}

	reader, err := ringbuf.NewReader(events)
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("failed to create ring buffer reader: %w", err)
	}
	s.reader = reader

	// 初始同步监听端口白名单
	s.syncPorts()

	go s.readLoop()
	go s.portsLoop()

	return s, nil
}

// Events 返回事件通道
func (s *Scanner) Events() <-chan Event {
	return s.eventsCh
}

// Close 卸载 eBPF 程序并清理资源
func (s *Scanner) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}

	if s.reader != nil {
		_ = s.reader.Close()
	}

	for name, h := range s.handles {
		if err := h.link.Close(); err != nil {
			s.log.Warn("failed to detach eBPF program", slog.String("iface", name), slog.Any("err", err))
		}
		s.log.Info("eBPF scan detector detached", slog.String("iface", name))
	}

	if s.prog != nil {
		_ = s.prog.Close()
	}
	if s.events != nil {
		_ = s.events.Close()
	}
	if s.ports != nil {
		_ = s.ports.Close()
	}
	if s.skipSrcPorts != nil {
		_ = s.skipSrcPorts.Close()
	}

	return nil
}

// attach 挂载 eBPF 程序到指定网卡的 TC ingress
func (s *Scanner) attach(ifaceName string) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", ifaceName, err)
	}

	l, err := link.AttachTCX(link.TCXOptions{
		Interface: iface.Index,
		Program:   s.prog,
		Attach:    ebpf.AttachTCXIngress,
	})
	if err != nil {
		return fmt.Errorf("failed to attach TCX to %s: %w", ifaceName, err)
	}

	s.handles[ifaceName] = &ifaceHandle{link: l}
	s.log.Info("eBPF scan detector attached", slog.String("iface", ifaceName))

	return nil
}

// readLoop 持续读取 Ring Buffer 事件
func (s *Scanner) readLoop() {
	defer close(s.eventsCh)

	for {
		record, err := s.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			s.log.Warn("failed to read ring buffer", slog.Any("err", err))
			continue
		}

		if len(record.RawSample) < eventSize {
			continue
		}

		select {
		case s.eventsCh <- parseEvent(record.RawSample):
		default:
		}
	}
}

// portsLoop 定时同步监听端口白名单
func (s *Scanner) portsLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.syncPorts()
		}
	}
}

// syncPorts 读取系统当前监听端口并同步到 BPF map
func (s *Scanner) syncPorts() {
	newPorts := readListeningPorts()

	// 删除不再监听的端口
	for port := range s.lastPorts {
		if !newPorts[port] {
			_ = s.ports.Delete(uint32(port))
		}
	}

	// 添加新监听的端口
	for port := range newPorts {
		if !s.lastPorts[port] {
			_ = s.ports.Put(uint32(port), uint8(1))
		}
	}

	s.lastPorts = newPorts
}

// readListeningPorts 从 /proc 读取当前所有监听端口
func readListeningPorts() map[uint16]bool {
	ports := make(map[uint16]bool)
	// TCP LISTEN (state 0A)
	parseProcNet("/proc/net/tcp", "0A", ports)
	parseProcNet("/proc/net/tcp6", "0A", ports)
	// UDP 绑定端口 (state 07 = unconnected)
	parseProcNet("/proc/net/udp", "07", ports)
	parseProcNet("/proc/net/udp6", "07", ports)
	return ports
}

// parseProcNet 解析 /proc/net/{tcp,tcp6,udp,udp6} 提取指定状态的本地端口
func parseProcNet(path, state string, ports map[uint16]bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] != state {
			continue
		}
		// local_address 格式: "00000000:0050" 或 "00000000000000000000000000000000:0050"
		idx := strings.LastIndex(fields[1], ":")
		if idx < 0 {
			continue
		}
		port, err := strconv.ParseUint(fields[1][idx+1:], 16, 16)
		if err != nil {
			continue
		}
		ports[uint16(port)] = true
	}
}

// parseEvent 解析原始事件数据
// 栈布局：[src_ip(16) | dst_port(2) | protocol(1) | version(1)]
func parseEvent(data []byte) Event {
	version := data[19]

	var ip net.IP
	if version == 6 {
		ip = make(net.IP, 16)
		copy(ip, data[0:16])
	} else {
		ip = net.IPv4(data[0], data[1], data[2], data[3])
	}

	dstPort := binary.NativeEndian.Uint16(data[16:18])
	proto := data[18]

	protoStr := "tcp"
	if proto == ipUDP {
		protoStr = "udp"
	}

	return Event{
		SourceIP:  ip.String(),
		Port:      dstPort,
		Protocol:  protoStr,
		Timestamp: time.Now(),
	}
}
