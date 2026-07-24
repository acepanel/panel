//go:build linux

package tamper

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/btf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"
	"golang.org/x/sys/unix"
)

const (
	oWrOnly = 1
	oRdWr   = 2
)

// 事件布局:inode(8)+dev(8)+pid(4)+op(4)+comm(16)+name(256),name 覆盖 NAME_MAX+NUL
const (
	eventStructSize = 296
	eventNameOff    = 40
	eventNameSize   = 256
)

// op 最高位携带 denied 标志,区分内核拦截与放行观察
const eventDeniedBit = uint32(1) << 31

// dirValue: flags(1)+pad(7)+extMask(8);flags 0=整树,1=按 extMask 匹配尾缀
const dirValueSize = 16

// protKey 加 dev 是因不同文件系统 inode 会冲突(如 tmpfs)
type protKey struct {
	Ino uint64
	Dev uint64
}

// 6.15+ 会把字段包进匿名 union(如 dentry.d_name)
type btfOffsets struct {
	fFlags, fInode                   uint32
	dInode, dName, qstrLen, qstrName uint32
	iIno, iMode, iSb, sDev           uint32
}

type hookSpec struct {
	hook     string
	param    string // 目标参数名
	param2   string // rename 覆盖目标 / 创建类的新名 dentry
	isFile   bool   // struct file(查 f_flags)还是 dentry
	isDir    bool   // 创建类:按父目录 inode 查目录集合
	optional bool   // 内核缺该符号时静默跳过
	op       Op
}

// 存量对象拦截:文件与目录 inode 均入 protected 集合,故这些 hook 同时护住目录本身
// rename 需查 new_dentry 防改名覆盖;setattr 覆盖 chmod/chown/truncate;
// setxattr/removexattr 独立于 set_acl/remove_acl(旧内核无 acl hook,标 optional)
var lsmHooks = []hookSpec{
	{hook: "file_open", param: "file", isFile: true, op: OpWrite},
	{hook: "inode_unlink", param: "dentry", op: OpUnlink},
	{hook: "inode_rmdir", param: "dentry", op: OpUnlink},
	{hook: "inode_rename", param: "old_dentry", param2: "new_dentry", op: OpRename},
	{hook: "inode_setattr", param: "dentry", op: OpSetattr},
	{hook: "inode_setxattr", param: "dentry", op: OpSetattr},
	{hook: "inode_removexattr", param: "dentry", op: OpSetattr},
	{hook: "inode_set_acl", param: "dentry", op: OpSetattr, optional: true},
	{hook: "inode_remove_acl", param: "dentry", op: OpSetattr, optional: true},
	{hook: "inode_file_setattr", param: "dentry", op: OpSetattr, optional: true}, // FS_IOC_FSSETXATTR 等
	{hook: "inode_link", param: "old_dentry", op: OpLink},
}

// 创建类:按父目录 inode 查目录集合;严格模式禁 mkdir 与目录移入以消除异步纳管竞态
var dirHooks = []hookSpec{
	{hook: "inode_create", param: "dir", param2: "dentry", isDir: true, op: OpCreate},
	{hook: "inode_mkdir", param: "dir", param2: "dentry", isDir: true, op: OpCreate},
	{hook: "inode_mknod", param: "dir", param2: "dentry", isDir: true, op: OpCreate},
	{hook: "inode_symlink", param: "dir", param2: "dentry", isDir: true, op: OpCreate},
	{hook: "inode_link", param: "dir", param2: "new_dentry", isDir: true, op: OpCreate},
	{hook: "inode_rename", param: "new_dir", param2: "new_dentry", isDir: true, op: OpCreate},
}

type ebpfEngine struct {
	log           *slog.Logger
	blockNew      bool
	protected     *ebpf.Map
	protectedDirs *ebpf.Map
	eventsMap     *ebpf.Map
	progs         []*ebpf.Program
	exts          []string
	extIndex      map[string]int
	reader        *ringbuf.Reader

	// mu 串行化 start/apply/remove/Close,防 use-after-close
	mu      sync.Mutex
	links   []link.Link
	started bool
	closed  bool

	out       chan Event
	done      chan struct{}
	wg        sync.WaitGroup
	once      sync.Once
	closeErr  error
	userDrops atomic.Uint64 // 观察模式下 drop=漏纳保,严格模式仅影响审计
}

func fieldOffset(spec *btf.Spec, structName, fieldName string) (uint32, error) {
	var s *btf.Struct
	if err := spec.TypeByName(structName, &s); err != nil {
		return 0, fmt.Errorf("btf struct %s: %w", structName, err)
	}
	if off, ok := findMember(s.Members, fieldName); ok {
		return off, nil
	}
	return 0, fmt.Errorf("btf struct %s has no member %s", structName, fieldName)
}

func findMember(members []btf.Member, name string) (uint32, bool) {
	for _, m := range members {
		if m.Name == name {
			return m.Offset.Bytes(), true
		}
		if m.Name != "" {
			continue
		}
		var inner []btf.Member
		switch t := btf.UnderlyingType(m.Type).(type) {
		case *btf.Struct:
			inner = t.Members
		case *btf.Union:
			inner = t.Members
		default:
			continue
		}
		if off, ok := findMember(inner, name); ok {
			return m.Offset.Bytes() + off, true
		}
	}
	return 0, false
}

// LSM BPF ctx 布局:[arg0..argN, prevRet]
func paramLayout(spec *btf.Spec, hook, param string) (paramIdx, nargs int, err error) {
	var fn *btf.Func
	if err = spec.TypeByName("bpf_lsm_"+hook, &fn); err != nil {
		return 0, 0, fmt.Errorf("btf func bpf_lsm_%s: %w", hook, err)
	}
	proto, ok := fn.Type.(*btf.FuncProto)
	if !ok {
		return 0, 0, fmt.Errorf("bpf_lsm_%s is not a func proto", hook)
	}
	idx := -1
	for i, p := range proto.Params {
		if p.Name == param {
			idx = i
		}
	}
	if idx < 0 {
		return 0, 0, fmt.Errorf("bpf_lsm_%s has no param %s", hook, param)
	}
	return idx, len(proto.Params), nil
}

// eventInsns 入口约定:R9=inode R8=dev,R6 复用为事件指针
// namePtrOff=0 无新名;verdictInR7 时按 R7 标 denied,否则恒 denied
func eventInsns(op Op, eventsFD int, namePtrOff int16, verdictInR7 bool) asm.Instructions {
	insns := asm.Instructions{
		asm.LoadMapPtr(asm.R1, eventsFD).WithSymbol("hit"),
		asm.Mov.Imm(asm.R2, eventStructSize),
		asm.Mov.Imm(asm.R3, 0),
		asm.FnRingbufReserve.Call(),
		asm.JEq.Imm(asm.R0, 0, "deny"),
		asm.Mov.Reg(asm.R6, asm.R0),

		asm.StoreMem(asm.R6, 0, asm.R9, asm.DWord),
		asm.StoreMem(asm.R6, 8, asm.R8, asm.DWord),
		asm.FnGetCurrentPidTgid.Call(),
		asm.RSh.Imm(asm.R0, 32), // 高 32 位 = tgid(用户所见 PID)
		asm.StoreMem(asm.R6, 16, asm.R0, asm.Word),
	}
	if verdictInR7 {
		insns = append(insns,
			asm.Mov.Imm(asm.R2, int32(uint32(op)|eventDeniedBit)),
			asm.JNE.Imm(asm.R7, 0, "op_store"),
			asm.Mov.Imm(asm.R2, int32(op)),
			asm.StoreMem(asm.R6, 20, asm.R2, asm.Word).WithSymbol("op_store"),
		)
	} else {
		insns = append(insns, asm.StoreImm(asm.R6, 20, int64(uint32(op)|eventDeniedBit), asm.Word))
	}
	insns = append(insns,
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Add.Imm(asm.R1, 24),
		asm.Mov.Imm(asm.R2, 16),
		asm.FnGetCurrentComm.Call(),
	)
	// ringbuf 复用旧数据
	insns = append(insns, asm.StoreImm(asm.R6, eventNameOff, 0, asm.Byte))
	if namePtrOff == 0 {
		return insns
	}
	return append(insns,
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Add.Imm(asm.R1, eventNameOff),
		asm.Mov.Imm(asm.R2, eventNameSize),
		asm.LoadMem(asm.R3, asm.RFP, namePtrOff, asm.DWord),
		asm.FnProbeReadKernelStr.Call(),
	)
}

// R6=ctx/事件指针 R7=prev_ret R8=dev R9=ino(R6-R9 跨 helper 保留)
func buildProg(h hookSpec, offs btfOffsets, paramIdx, paramIdx2, nargs, protectedFD, eventsFD int) asm.Instructions {
	retOff := int16(nargs * 8)

	lookup := asm.Instructions{
		asm.LoadMem(asm.R9, asm.R2, int16(offs.iIno), asm.DWord),
		asm.LoadMem(asm.R8, asm.R2, int16(offs.iSb), asm.DWord),
		asm.LoadMem(asm.R8, asm.R8, int16(offs.sDev), asm.Word),
		asm.StoreMem(asm.RFP, -16, asm.R9, asm.DWord),
		asm.StoreMem(asm.RFP, -8, asm.R8, asm.DWord),
		asm.LoadMapPtr(asm.R1, protectedFD),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -16),
		asm.FnMapLookupElem.Call(),
	}

	insns := asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),
		asm.LoadMem(asm.R7, asm.R6, retOff, asm.DWord),
		asm.JNE.Imm(asm.R7, 0, "deny_prev"),
	}

	if h.isFile {
		insns = append(insns,
			asm.LoadMem(asm.R2, asm.R6, int16(paramIdx*8), asm.DWord),
			asm.LoadMem(asm.R3, asm.R2, int16(offs.fFlags), asm.Word),
			asm.And.Imm(asm.R3, oWrOnly|oRdWr),
			asm.JEq.Imm(asm.R3, 0, "allow"),
			asm.LoadMem(asm.R2, asm.R2, int16(offs.fInode), asm.DWord),
		)
	} else {
		// 6.13+ dentry.d_inode 标 trusted_or_null,解引用前须判空
		insns = append(insns,
			asm.LoadMem(asm.R2, asm.R6, int16(paramIdx*8), asm.DWord),
			asm.LoadMem(asm.R2, asm.R2, int16(offs.dInode), asm.DWord),
			asm.JEq.Imm(asm.R2, 0, "allow"),
		)
	}

	insns = append(insns, lookup...)

	if paramIdx2 >= 0 {
		// rename 双目标:目标 1 未命中再查目标 2(覆盖既有文件),负 dentry 放行
		insns = append(insns, asm.JNE.Imm(asm.R0, 0, "hit"))
		insns = append(insns,
			asm.LoadMem(asm.R2, asm.R6, int16(paramIdx2*8), asm.DWord),
			asm.JEq.Imm(asm.R2, 0, "allow"),
			asm.LoadMem(asm.R2, asm.R2, int16(offs.dInode), asm.DWord),
			asm.JEq.Imm(asm.R2, 0, "allow"),
		)
		insns = append(insns, lookup...)
	}

	insns = append(insns, asm.JEq.Imm(asm.R0, 0, "allow"))
	insns = append(insns, eventInsns(h.op, eventsFD, 0, false)...)
	insns = append(insns,
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, 0),
		asm.FnRingbufSubmit.Call(),

		asm.Mov.Imm(asm.R0, -1).WithSymbol("deny"),
		asm.Return(),
		asm.Mov.Imm(asm.R0, 0).WithSymbol("allow"),
		asm.Return(),
		asm.Mov.Reg(asm.R0, asm.R7).WithSymbol("deny_prev"),
		asm.Return(),
	)

	return insns
}

// R7=prev_ret,透传后复用为本程序返回值(软命中路径改 0 放行)
// 栈:FP-16 复合键 / FP-24 dir value 指针 / FP-32 新名 char* / FP-36 新名长度 / FP-64 起 15B 尾部缓冲
func buildDirProg(h hookSpec, offs btfOffsets, paramIdx, nameIdx, oldIdx, nargs int, exts []string, dirsFD, eventsFD int, deny bool) asm.Instructions {
	retOff := int16(nargs * 8)
	ret := int32(-1)
	if !deny {
		ret = 0
	}
	slot := func(i int) string {
		if i >= len(exts) {
			return "allow"
		}
		return fmt.Sprintf("ext%d", i)
	}

	insns := asm.Instructions{
		asm.Mov.Reg(asm.R6, asm.R1),
		asm.LoadMem(asm.R7, asm.R6, retOff, asm.DWord),
		asm.JNE.Imm(asm.R7, 0, "deny"),
		asm.Mov.Imm(asm.R7, ret),

		asm.LoadMem(asm.R2, asm.R6, int16(paramIdx*8), asm.DWord),
		asm.LoadMem(asm.R9, asm.R2, int16(offs.iIno), asm.DWord),
		asm.LoadMem(asm.R8, asm.R2, int16(offs.iSb), asm.DWord),
		asm.LoadMem(asm.R8, asm.R8, int16(offs.sDev), asm.Word),
		asm.StoreMem(asm.RFP, -16, asm.R9, asm.DWord),
		asm.StoreMem(asm.RFP, -8, asm.R8, asm.DWord),
		asm.LoadMapPtr(asm.R1, dirsFD),
		asm.Mov.Reg(asm.R2, asm.RFP),
		asm.Add.Imm(asm.R2, -16),
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "allow"),
		asm.StoreMem(asm.RFP, -24, asm.R0, asm.DWord),

		asm.LoadMem(asm.R4, asm.R6, int16(nameIdx*8), asm.DWord),
		asm.LoadMem(asm.R5, asm.R4, int16(offs.dName+offs.qstrLen), asm.Word),
		asm.StoreMem(asm.RFP, -36, asm.R5, asm.Word),
		asm.LoadMem(asm.R4, asm.R4, int16(offs.dName+offs.qstrName), asm.DWord),
		asm.StoreMem(asm.RFP, -32, asm.R4, asm.DWord),

		asm.LoadMem(asm.R3, asm.R0, 0, asm.Byte),
		asm.JEq.Imm(asm.R3, 0, "hit"),
	}

	switch {
	case h.hook == "inode_mkdir":
		if !deny {
			insns = append(insns, asm.Mov.Imm(asm.R7, 0))
		}
		insns = append(insns, asm.Ja.Label("hit"))
		exts = nil // 后续扩展名槽会成为死代码
	case oldIdx >= 0:
		// 源为目录:严格拒/观察软放行;源为文件按扩展名匹配
		insns = append(insns,
			asm.LoadMem(asm.R4, asm.R6, int16(oldIdx*8), asm.DWord),
			asm.LoadMem(asm.R4, asm.R4, int16(offs.dInode), asm.DWord),
			asm.JEq.Imm(asm.R4, 0, slot(0)),
			asm.LoadMem(asm.R5, asm.R4, int16(offs.iMode), asm.Half),
			asm.And.Imm(asm.R5, 0xf000),
			asm.JNE.Imm(asm.R5, 0x4000, slot(0)), // S_IFDIR
		)
		if !deny {
			insns = append(insns, asm.Mov.Imm(asm.R7, 0))
		}
		insns = append(insns, asm.Ja.Label("hit"))
	case len(exts) == 0:
		insns = append(insns, asm.Ja.Label("allow"))
	}

	for i, ext := range exts {
		next := slot(i + 1)
		tail := "." + ext
		l := len(tail)
		insns = append(insns,
			asm.LoadMem(asm.R5, asm.RFP, -24, asm.DWord).WithSymbol(slot(i)),
			asm.LoadMem(asm.R3, asm.R5, 8, asm.DWord),
			asm.RSh.Imm(asm.R3, int32(i)),
			asm.And.Imm(asm.R3, 1),
			asm.JEq.Imm(asm.R3, 0, next),
			asm.LoadMem(asm.R4, asm.RFP, -36, asm.Word),
			asm.JLT.Imm(asm.R4, int32(l), next),
			asm.Sub.Imm(asm.R4, int32(l)),
			asm.LoadMem(asm.R5, asm.RFP, -32, asm.DWord),
			asm.Add.Reg(asm.R5, asm.R4),
			asm.Mov.Reg(asm.R1, asm.RFP),
			asm.Add.Imm(asm.R1, -64),
			asm.Mov.Imm(asm.R2, int32(l)),
			asm.Mov.Reg(asm.R3, asm.R5),
			asm.FnProbeReadKernel.Call(),
			asm.JNE.Imm(asm.R0, 0, next),
		)
		for j := range l {
			c := int32(tail[j])
			load := asm.LoadMem(asm.R2, asm.RFP, int16(-64+j), asm.Byte)
			if j > 0 {
				load = load.WithSymbol(fmt.Sprintf("e%db%d", i, j))
			}
			insns = append(insns, load)
			if c >= 'a' && c <= 'z' {
				insns = append(insns,
					asm.JEq.Imm(asm.R2, c, fmt.Sprintf("e%db%d", i, j+1)),
					asm.JNE.Imm(asm.R2, c-32, next),
				)
			} else {
				insns = append(insns, asm.JNE.Imm(asm.R2, c, next))
			}
		}
		insns = append(insns, asm.Ja.Label("hit").WithSymbol(fmt.Sprintf("e%db%d", i, l)))
	}

	insns = append(insns, eventInsns(h.op, eventsFD, -32, true)...)
	insns = append(insns,
		asm.Mov.Reg(asm.R1, asm.R6),
		asm.Mov.Imm(asm.R2, 0),
		asm.FnRingbufSubmit.Call(),

		asm.Mov.Reg(asm.R0, asm.R7).WithSymbol("deny"),
		asm.Return(),
		asm.Mov.Imm(asm.R0, 0).WithSymbol("allow"),
		asm.Return(),
	)

	return insns
}

// normExts 拒绝无法进内核匹配的扩展名:超 14 字节 / 非 ASCII / 超 64 条位图
func normExts(ruleExts []string) (accepted, rejected []string) {
	for _, x := range ruleExts {
		n := NormExt(x)
		if n == "" || slices.Contains(accepted, n) {
			continue
		}
		if len(n) > 14 || !isASCIIExt(n) || len(accepted) == 64 {
			if !slices.Contains(rejected, n) {
				rejected = append(rejected, n)
			}
			continue
		}
		accepted = append(accepted, n)
	}
	return accepted, rejected
}

func isASCIIExt(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// newEBPFEngine 只加载不 attach,attach 由 start 触发;先 apply 后 attach 消除激活空窗
func newEBPFEngine(log *slog.Logger, blockNew bool, ruleExts []string) (*ebpfEngine, error) {
	if log == nil {
		log = slog.Default()
	}
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock limit: %w", err)
	}

	spec, err := btf.LoadKernelSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to load kernel BTF: %w", err)
	}

	var offs btfOffsets
	for _, f := range []struct {
		dst             *uint32
		structN, fieldN string
	}{
		{&offs.fFlags, "file", "f_flags"},
		{&offs.fInode, "file", "f_inode"},
		{&offs.dInode, "dentry", "d_inode"},
		{&offs.dName, "dentry", "d_name"},
		{&offs.qstrLen, "qstr", "len"},
		{&offs.qstrName, "qstr", "name"},
		{&offs.iIno, "inode", "i_ino"},
		{&offs.iMode, "inode", "i_mode"},
		{&offs.iSb, "inode", "i_sb"},
		{&offs.sDev, "super_block", "s_dev"},
	} {
		if *f.dst, err = fieldOffset(spec, f.structN, f.fieldN); err != nil {
			return nil, err
		}
	}

	// NO_PREALLOC 避免百万级 MaxEntries 预占约 80MB 内核内存
	protected, err := ebpf.NewMap(&ebpf.MapSpec{
		Type: ebpf.Hash, KeySize: 16, ValueSize: 1, MaxEntries: 1 << 20,
		Flags: unix.BPF_F_NO_PREALLOC,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create protected map: %w", err)
	}
	dirs, err := ebpf.NewMap(&ebpf.MapSpec{
		Type: ebpf.Hash, KeySize: 16, ValueSize: dirValueSize, MaxEntries: 1 << 18,
		Flags: unix.BPF_F_NO_PREALLOC,
	})
	if err != nil {
		_ = protected.Close()
		return nil, fmt.Errorf("failed to create protected dirs map: %w", err)
	}
	events, err := ebpf.NewMap(&ebpf.MapSpec{Type: ebpf.RingBuf, MaxEntries: 1 << 20})
	if err != nil {
		_ = dirs.Close()
		_ = protected.Close()
		return nil, fmt.Errorf("failed to create events ringbuf: %w", err)
	}

	exts, rejected := normExts(ruleExts)
	if len(rejected) > 0 {
		log.Warn("tamper extensions cannot be matched in kernel, ignored", slog.Any("ignored", rejected))
	}
	extIndex := make(map[string]int, len(exts))
	for i, x := range exts {
		extIndex[x] = i
	}

	e := &ebpfEngine{
		log:           log,
		blockNew:      blockNew,
		protected:     protected,
		protectedDirs: dirs,
		eventsMap:     events,
		exts:          exts,
		extIndex:      extIndex,
		out:           make(chan Event, 256),
		done:          make(chan struct{}),
	}

	hooks := slices.Clone(lsmHooks)
	hooks = append(hooks, dirHooks...)
	for _, h := range hooks {
		paramIdx, nargs, err := paramLayout(spec, h.hook, h.param)
		if err != nil {
			if h.optional && errors.Is(err, btf.ErrNotFound) {
				log.Debug("skip unavailable optional LSM hook", slog.String("hook", h.hook))
				continue
			}
			_ = e.Close()
			return nil, err
		}
		var insns asm.Instructions
		name := "tamper_" + h.hook
		if h.isDir {
			nameIdx, _, err := paramLayout(spec, h.hook, h.param2)
			if err != nil {
				_ = e.Close()
				return nil, err
			}
			oldIdx := -1
			if h.hook == "inode_rename" {
				if oldIdx, _, err = paramLayout(spec, h.hook, "old_dentry"); err != nil {
					_ = e.Close()
					return nil, err
				}
			}
			name = "tamper_new_" + strings.TrimPrefix(h.hook, "inode_")
			insns = buildDirProg(h, offs, paramIdx, nameIdx, oldIdx, nargs, e.exts, dirs.FD(), events.FD(), blockNew)
		} else {
			paramIdx2 := -1
			if h.param2 != "" {
				if paramIdx2, _, err = paramLayout(spec, h.hook, h.param2); err != nil {
					_ = e.Close()
					return nil, err
				}
			}
			insns = buildProg(h, offs, paramIdx, paramIdx2, nargs, protected.FD(), events.FD())
		}
		// inode_link 挂两个程序(源检查 + 目标目录策略),name 区分
		if h.hook == "inode_link" && !h.isDir {
			name = "tamper_linksrc"
		}
		prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
			Name:         name,
			Type:         ebpf.LSM,
			AttachType:   ebpf.AttachLSMMac,
			AttachTo:     h.hook,
			License:      "GPL",
			Instructions: insns,
		})
		if err != nil {
			_ = e.Close()
			if ve, ok := errors.AsType[*ebpf.VerifierError](err); ok {
				return nil, fmt.Errorf("failed to load %s LSM program: %+v", h.hook, ve)
			}
			return nil, fmt.Errorf("failed to load %s LSM program: %w", h.hook, err)
		}
		e.progs = append(e.progs, prog)
	}

	e.reader, err = ringbuf.NewReader(events)
	if err != nil {
		_ = e.Close()
		return nil, fmt.Errorf("failed to create ringbuf reader: %w", err)
	}

	e.wg.Add(1)
	go e.readLoop()
	return e, nil
}

// start 幂等,失败回滚已 attach 的
func (e *ebpfEngine) start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("tamper engine closed")
	}
	if e.started {
		return nil
	}
	attached := make([]link.Link, 0, len(e.progs))
	for _, prog := range e.progs {
		l, err := link.AttachLSM(link.LSMOptions{Program: prog})
		if err != nil {
			var errs []error
			for i := len(attached) - 1; i >= 0; i-- {
				errs = append(errs, attached[i].Close())
			}
			return errors.Join(fmt.Errorf("failed to attach %s: %w", prog, err), errors.Join(errs...))
		}
		attached = append(attached, l)
	}
	e.links = attached
	e.started = true
	return nil
}

// readLoop 唯一发送者,退出时关 out
func (e *ebpfEngine) readLoop() {
	defer e.wg.Done()
	defer close(e.out)
	var rec ringbuf.Record
	for {
		if err := e.reader.ReadInto(&rec); err != nil {
			if !errors.Is(err, ringbuf.ErrClosed) {
				e.log.Error("tamper ringbuf reader stopped, event reporting disabled", slog.Any("err", err))
			}
			return
		}
		b := rec.RawSample
		if len(b) < eventStructSize {
			continue
		}
		raw := binary.NativeEndian.Uint32(b[20:24])
		op := Op(raw &^ eventDeniedBit)
		ev := Event{
			Inode:  binary.NativeEndian.Uint64(b[0:8]),
			Dev:    binary.NativeEndian.Uint64(b[8:16]),
			PID:    binary.NativeEndian.Uint32(b[16:20]),
			Op:     op,
			OpStr:  op.String(),
			Denied: raw&eventDeniedBit != 0,
			Comm:   cString(b[24:40]),
			Name:   cString(b[eventNameOff:eventStructSize]),
		}
		select {
		case e.out <- ev:
		case <-e.done:
			return
		default:
			// 观察模式 drop=漏纳保
			if n := e.userDrops.Add(1); !e.blockNew && (n == 1 || n%1000 == 0) {
				e.log.Warn("tamper event dropped in observe mode, new object may miss protection",
					slog.Uint64("drops", n))
			}
		}
	}
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// 目录 inode 一起进 protected,让存量 hook 顺带护住目录本身
func keysOf(entries []fileEntry) []protKey {
	keys := make([]protKey, 0, len(entries))
	for _, en := range entries {
		if en.inode != 0 {
			keys = append(keys, protKey{Ino: en.inode, Dev: en.dev})
		}
	}
	return keys
}

// dirValue 任一扩展名无法进内核表即升级整树,fail-closed
func (e *ebpfEngine) dirValue(exts []string) [dirValueSize]byte {
	var v [dirValueSize]byte
	if len(exts) == 0 {
		return v
	}
	var mask uint64
	var missing []string
	for _, x := range exts {
		if i, ok := e.extIndex[NormExt(x)]; ok {
			mask |= uint64(1) << i
			continue
		}
		missing = append(missing, x)
	}
	if len(missing) != 0 || mask == 0 {
		e.log.Warn("tamper dir exts cannot be represented, upgrading to whole-tree",
			slog.Any("exts", exts), slog.Any("unmatched", missing))
		return v
	}
	v[0] = 1
	binary.NativeEndian.PutUint64(v[8:], mask)
	return v
}

// batchUpdateAll 部分成功视为错误,避免静默半生效
func batchUpdateAll[K, V any](m *ebpf.Map, keys []K, values []V) error {
	if len(keys) != len(values) {
		return fmt.Errorf("batch update key/value count mismatch: %d/%d", len(keys), len(values))
	}
	if len(keys) == 0 {
		return nil
	}
	n, err := m.BatchUpdate(keys, values, nil)
	if err != nil {
		return fmt.Errorf("batch update %d/%d: %w", n, len(keys), err)
	}
	if n != len(keys) {
		return fmt.Errorf("short batch update %d/%d", n, len(keys))
	}
	return nil
}

// apply 先目录创建策略后对象保护,失败时偏 fail-closed
func (e *ebpfEngine) apply(entries []fileEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("tamper engine closed")
	}
	keys := make([]protKey, 0, len(entries))
	values := make([][dirValueSize]byte, 0, len(entries))
	for _, en := range entries {
		if en.isDir && en.inode != 0 {
			keys = append(keys, protKey{Ino: en.inode, Dev: en.dev})
			values = append(values, e.dirValue(en.exts))
		}
	}
	if err := batchUpdateAll(e.protectedDirs, keys, values); err != nil {
		return err
	}
	return e.addLocked(keysOf(entries))
}

func (e *ebpfEngine) remove(entries []fileEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return errors.New("tamper engine closed")
	}
	errs := []error{e.removeLocked(keysOf(entries))}
	for _, en := range entries {
		if en.isDir && en.inode != 0 {
			if err := e.protectedDirs.Delete(protKey{Ino: en.inode, Dev: en.dev}); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (e *ebpfEngine) events() <-chan Event { return e.out }

func (e *ebpfEngine) UserDrops() uint64 { return e.userDrops.Load() }

func (e *ebpfEngine) close() error { return e.Close() }

func (e *ebpfEngine) addLocked(keys []protKey) error {
	values := make([]uint8, len(keys))
	for i := range values {
		values[i] = 1
	}
	return batchUpdateAll(e.protected, keys, values)
}

func (e *ebpfEngine) removeLocked(keys []protKey) error {
	var errs []error
	for _, k := range keys {
		if err := e.protected.Delete(k); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Close 顺序:标记 → detach 停生产 → 关 reader → 等 readLoop → 释放 map/prog
func (e *ebpfEngine) Close() error {
	e.once.Do(func() {
		e.mu.Lock()
		e.closed = true
		links := e.links
		e.links = nil
		e.mu.Unlock()
		close(e.done)

		var errs []error
		for _, l := range links {
			errs = append(errs, l.Close())
		}
		if e.reader != nil {
			errs = append(errs, e.reader.Close())
		}
		e.wg.Wait()
		for _, p := range e.progs {
			errs = append(errs, p.Close())
		}
		if e.eventsMap != nil {
			errs = append(errs, e.eventsMap.Close())
		}
		if e.protectedDirs != nil {
			errs = append(errs, e.protectedDirs.Close())
		}
		if e.protected != nil {
			errs = append(errs, e.protected.Close())
		}
		e.closeErr = errors.Join(errs...)
	})
	return e.closeErr
}
