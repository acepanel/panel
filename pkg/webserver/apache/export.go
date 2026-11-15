package apache

import (
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
)

const defaultIndent = "    "

// orderLower 是 order 表的小写键副本，用于大小写不敏感查找
var orderLower = func() map[string]int {
	m := make(map[string]int, len(order))
	for k, v := range order {
		m[strings.ToLower(k)] = v
	}
	return m
}()

// Export 保序导出（用于片段）：按节点原有顺序输出，规范缩进
func Export(c *conf.Config) string {
	var b strings.Builder
	writeNodes(&b, c.Nodes, 0, false)
	return strings.TrimRight(b.String(), "\n")
}

// Render 规范化导出（用于主文件 Save）：按 order 表稳定排序后输出
func Render(c *conf.Config) string {
	var b strings.Builder
	writeNodes(&b, c.Nodes, 0, true)
	return strings.TrimRight(b.String(), "\n")
}

func writeNodes(b *strings.Builder, nodes []conf.Node, depth int, sorted bool) {
	if sorted {
		nodes = conf.Sort(nodes, orderOf)
	}
	indent := strings.Repeat(defaultIndent, depth)
	for _, n := range nodes {
		switch v := n.(type) {
		case *conf.Comment:
			b.WriteString(indent + "#" + v.Text + "\n")
		case *conf.Blank:
			b.WriteByte('\n')
		case *conf.Directive:
			b.WriteString(indent)
			if v.Block != nil {
				b.WriteByte('<')
			}
			b.WriteString(v.Name)
			for _, a := range v.Args {
				b.WriteString(" " + quote(a))
			}
			if v.Block == nil {
				b.WriteByte('\n')
				continue
			}
			b.WriteString(">\n")
			writeNodes(b, v.Nodes, depth+1, sorted)
			b.WriteString(indent + "</" + v.Name + ">\n")
		}
	}
}

// orderOf 返回排序键，未知指令排在已知功能指令之后、容器之前
func orderOf(d *conf.Directive) int {
	if k, ok := orderLower[strings.ToLower(d.Name)]; ok {
		return k
	}
	return orderDefault
}
