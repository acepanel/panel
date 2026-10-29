package conf

import "strings"

// 名称比较一律大小写不敏感：apache 与 OLS 本身不区分，nginx 与 Caddy 的指令名全小写

func (b *Block) Append(nodes ...Node) {
	b.Nodes = append(b.Nodes, nodes...)
}

func (b *Block) Len() int {
	if b == nil {
		return 0
	}
	return len(b.Nodes)
}

func (b *Block) All() []*Directive {
	if b == nil {
		return nil
	}
	var out []*Directive
	for _, n := range b.Nodes {
		if d, ok := n.(*Directive); ok {
			out = append(out, d)
		}
	}
	return out
}

func (b *Block) Get(name string) *Directive {
	if b == nil {
		return nil
	}
	for _, n := range b.Nodes {
		if d, ok := n.(*Directive); ok && strings.EqualFold(d.Name, name) {
			return d
		}
	}
	return nil
}

func (b *Block) GetAll(name string) []*Directive {
	if b == nil {
		return nil
	}
	var out []*Directive
	for _, n := range b.Nodes {
		if d, ok := n.(*Directive); ok && strings.EqualFold(d.Name, name) {
			out = append(out, d)
		}
	}
	return out
}

func (b *Block) Has(name string) bool {
	return b.Get(name) != nil
}

func (b *Block) Value(name string) string {
	return b.Get(name).Arg(0)
}

func (b *Block) Add(name string, args ...string) *Directive {
	d := Dir(name, args...)
	b.Nodes = append(b.Nodes, d)
	return d
}

func (b *Block) AddBlock(name string, args ...string) *Directive {
	d := Blk(name, args...)
	b.Nodes = append(b.Nodes, d)
	return d
}

// Set 存在则更新首个同名指令的参数，否则追加
func (b *Block) Set(name string, args ...string) *Directive {
	if d := b.Get(name); d != nil {
		d.SetArgs(args...)
		return d
	}
	return b.Add(name, args...)
}

// Remove 删除全部同名指令，返回删除数量
func (b *Block) Remove(name string) int {
	return b.RemoveFunc(name, func(*Directive) bool { return true })
}

// RemoveFunc 删除满足条件的同名指令，返回删除数量
func (b *Block) RemoveFunc(name string, pred func(*Directive) bool) int {
	if b == nil {
		return 0
	}
	kept := b.Nodes[:0]
	count := 0
	for _, n := range b.Nodes {
		if d, ok := n.(*Directive); ok && strings.EqualFold(d.Name, name) && pred(d) {
			count++
			continue
		}
		kept = append(kept, n)
	}
	b.Nodes = kept
	return count
}

func (b *Block) Filter(keep func(Node) bool) {
	if b == nil {
		return
	}
	kept := b.Nodes[:0]
	for _, n := range b.Nodes {
		if keep(n) {
			kept = append(kept, n)
		}
	}
	b.Nodes = kept
}

// GetBlock 首个带子块的同名指令，给定参数时还要求参数逐项相等
func (b *Block) GetBlock(name string, args ...string) *Directive {
	for _, d := range b.Blocks(name) {
		if len(args) == 0 || argsEqual(d.Args, args) {
			return d
		}
	}
	return nil
}

func (b *Block) Blocks(name string) []*Directive {
	var out []*Directive
	for _, d := range b.GetAll(name) {
		if d.Block != nil {
			out = append(out, d)
		}
	}
	return out
}

// Find 按点路径跨块查找指令，如 Find("server.listen")
func (b *Block) Find(path string) []*Directive {
	if b == nil {
		return nil
	}
	parts := strings.Split(path, ".")
	blocks := []*Block{b}
	for _, part := range parts[:len(parts)-1] {
		var next []*Block
		for _, blk := range blocks {
			for _, d := range blk.Blocks(part) {
				next = append(next, d.Block)
			}
		}
		blocks = next
	}
	var out []*Directive
	for _, blk := range blocks {
		out = append(out, blk.GetAll(parts[len(parts)-1])...)
	}
	return out
}

func (b *Block) FindOne(path string) *Directive {
	if ds := b.Find(path); len(ds) > 0 {
		return ds[0]
	}
	return nil
}

func (b *Block) FindBlocks(path string) []*Directive {
	var out []*Directive
	for _, d := range b.Find(path) {
		if d.Block != nil {
			out = append(out, d)
		}
	}
	return out
}

// Comments 全部注释文本，去掉首尾空白
func (b *Block) Comments() []string {
	if b == nil {
		return nil
	}
	var out []string
	for _, n := range b.Nodes {
		if c, ok := n.(*Comment); ok {
			out = append(out, strings.TrimSpace(c.Text))
		}
	}
	return out
}

// Meta 读取形如 `# ace:key value` 的元数据注释
func (b *Block) Meta(key string) string {
	for _, c := range b.Comments() {
		if rest, ok := strings.CutPrefix(c, "ace:"+key+" "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func (b *Block) AddMeta(key, value string) {
	b.Nodes = append(b.Nodes, &Comment{Text: " ace:" + key + " " + value})
}

// customMarker 子块内分隔面板生成的指令与用户追加的原生指令
const customMarker = "ace:custom"

// AppendCustom 在子块末尾写分隔注释与用户原生指令，没有指令时不写注释
func (d *Directive) AppendCustom(nodes []Node) {
	if len(nodes) > 0 {
		d.Append(&Comment{Text: " " + customMarker})
		d.Append(nodes...)
	}
}

// SplitCustom 拆出分隔注释之后的用户原生指令，返回只含面板生成部分的浅拷贝，原指令不变
func (d *Directive) SplitCustom() (*Directive, []Node) {
	if d == nil || d.Block == nil {
		return d, nil
	}
	for i, n := range d.Nodes {
		if c, ok := n.(*Comment); ok && strings.TrimSpace(c.Text) == customMarker {
			return &Directive{Name: d.Name, Args: d.Args, Block: &Block{Nodes: d.Nodes[:i:i]}, Trailing: d.Trailing}, d.Nodes[i+1:]
		}
	}
	return d, nil
}

func argsEqual(args []Arg, want []string) bool {
	if len(args) != len(want) {
		return false
	}
	for i, a := range args {
		if a.Value != want[i] {
			return false
		}
	}
	return true
}

// Walk 深度优先遍历全部指令，含嵌套子块
func (b *Block) Walk(fn func(*Directive)) {
	for _, d := range b.All() {
		fn(d)
		d.Walk(fn)
	}
}
