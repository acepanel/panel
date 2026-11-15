// Package conf 是各 Web 服务器配置共用的语法树：指令由名称、参数和可选子块组成，
// 各方言只负责自己的分词、解析与渲染，查询与修改在这里统一实现
package conf

import "slices"

// Quote 参数的引号风格，解析时记录原样，生成时由方言按 QuoteAuto 决定是否加引号
type Quote uint8

const (
	QuoteAuto    Quote = iota // 生成的参数，渲染时按方言规则判断
	QuoteNone                 // 裸值
	QuoteDouble               // 双引号
	QuoteSingle               // 单引号
	QuoteHeredoc              // 多行值
	QuoteRaw                  // 原样输出的块体，如 nginx 的 lua 块
)

// Arg 指令参数：解引号后的纯值与引号风格
type Arg struct {
	Value string
	Quote Quote
}

// Node 配置节点：指令、注释或空行
type Node interface {
	node()
}

// Comment 整行注释，不含注释符
type Comment struct {
	Text string
}

// Blank 空行，只在生成时用于分组
type Blank struct{}

// Directive 指令，Block 非 nil 时带子块，Trailing 为行尾注释
type Directive struct {
	Name string
	Args []Arg
	*Block
	Trailing string
}

// Block 有序节点列表，Config 与带子块的指令共用查询与修改方法。查询与删除对 nil 接收者安全；
// 往里加节点的不行，指令可能不带子块时用 Directive.Append，它会先把子块建出来
type Block struct {
	Nodes []Node
}

type Config struct {
	Block
}

func (*Comment) node()   {}
func (*Blank) node()     {}
func (*Directive) node() {}

func Dir(name string, args ...string) *Directive {
	return &Directive{Name: name, Args: Args(args...)}
}

func Blk(name string, args ...string) *Directive {
	return &Directive{Name: name, Args: Args(args...), Block: &Block{}}
}

// Cmt 构造一条注释，文本前补一个空格
func Cmt(text string) *Comment {
	return &Comment{Text: " " + text}
}

func Args(values ...string) []Arg {
	if len(values) == 0 {
		return nil
	}
	out := make([]Arg, len(values))
	for i, v := range values {
		out[i] = Arg{Value: v}
	}
	return out
}

func Values(args []Arg) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a.Value
	}
	return out
}

// Arg 第 i 个参数的值，nil 或越界返回空串
func (d *Directive) Arg(i int) string {
	if d == nil || i < 0 || i >= len(d.Args) {
		return ""
	}
	return d.Args[i].Value
}

func (d *Directive) Values() []string {
	if d == nil {
		return nil
	}
	return Values(d.Args)
}

// ArgsFrom 第 i 个及之后参数的值，越界返回 nil
func (d *Directive) ArgsFrom(i int) []string {
	if d == nil || i >= len(d.Args) {
		return nil
	}
	return Values(d.Args[i:])
}

func (d *Directive) SetArgs(values ...string) {
	d.Args = Args(values...)
}

func (d *Directive) AppendArg(value string) {
	d.Args = append(d.Args, Arg{Value: value})
}

// Append 向子块追加节点，无子块时先创建，返回自身便于链式调用
func (d *Directive) Append(nodes ...Node) *Directive {
	if d.Block == nil {
		d.Block = &Block{}
	}
	d.Nodes = append(d.Nodes, nodes...)
	return d
}

func (d *Directive) Clone() *Directive {
	if d == nil {
		return nil
	}
	out := &Directive{Name: d.Name, Trailing: d.Trailing, Block: d.Block.Clone()}
	if d.Args != nil {
		out.Args = slices.Clone(d.Args)
	}
	return out
}

// Clone 深拷贝节点列表，副本与原树不共享任何可变结构
func (b *Block) Clone() *Block {
	if b == nil {
		return nil
	}
	out := &Block{Nodes: make([]Node, 0, len(b.Nodes))}
	for _, n := range b.Nodes {
		out.Nodes = append(out.Nodes, cloneNode(n))
	}
	return out
}

func cloneNode(n Node) Node {
	switch v := n.(type) {
	case *Directive:
		return v.Clone()
	case *Comment:
		return &Comment{Text: v.Text}
	default:
		return &Blank{}
	}
}
