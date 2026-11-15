package openlitespeed

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
)

// OpenLiteSpeed 纯文本配置：`key value` 行、以 `{` 结尾的块、`}` 结束块、`key <<<SIGN` 多行值、`#` 注释
// 指令只有一个参数，即键之后的整行；多行值以 QuoteHeredoc 标记

const indentUnit = "  "

func Parse(content string) (*conf.Config, error) {
	cfg := &conf.Config{}
	stack := []*conf.Block{&cfg.Block}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		// 行尾反斜杠续行
		for strings.HasSuffix(line, `\`) && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(line, `\`) + strings.TrimSpace(lines[i])
		}
		if line == "" {
			continue
		}
		cur := stack[len(stack)-1]

		if after, ok := strings.CutPrefix(line, "#"); ok {
			cur.Append(&conf.Comment{Text: after})
			continue
		}
		if strings.HasPrefix(line, "}") {
			if len(stack) == 1 {
				return nil, fmt.Errorf("line %d: unexpected '}'", i+1)
			}
			stack = stack[:len(stack)-1]
			// } 之后可能紧跟新内容
			if rest := strings.TrimSpace(line[1:]); rest != "" {
				lines[i] = rest
				i--
			}
			continue
		}
		if before, ok := strings.CutSuffix(line, "{"); ok {
			name, value := splitKeyValue(strings.TrimSpace(before))
			if name == "" {
				return nil, fmt.Errorf("line %d: block without name", i+1)
			}
			block := cur.AddBlock(name)
			if value != "" {
				block.Args = []conf.Arg{{Value: value, Quote: conf.QuoteNone}}
			}
			stack = append(stack, block.Block)
			continue
		}

		name, value := splitKeyValue(line)
		if sign, ok := strings.CutPrefix(value, "<<<"); ok {
			sign = strings.TrimSpace(sign)
			var body []string
			closed := false
			for i+1 < len(lines) {
				i++
				if strings.TrimSpace(lines[i]) == sign {
					closed = true
					break
				}
				body = append(body, lines[i])
			}
			if !closed {
				return nil, fmt.Errorf("unterminated multiline value for %s", name)
			}
			cur.Append(&conf.Directive{Name: name, Args: []conf.Arg{{Value: strings.Join(body, "\n"), Quote: conf.QuoteHeredoc}}})
			continue
		}
		d := cur.Add(name)
		if value != "" {
			d.Args = []conf.Arg{{Value: value, Quote: conf.QuoteNone}}
		}
	}

	if len(stack) != 1 {
		return nil, errors.New("unclosed block")
	}

	return cfg, nil
}

func ParseFile(path string) (*conf.Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(content))
}

func splitKeyValue(line string) (string, string) {
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return line, ""
	}
	return line[:idx], strings.TrimSpace(line[idx+1:])
}

// Export 渲染为配置文本，顶层块之间空一行
func Export(c *conf.Config) string {
	var b strings.Builder
	for i, n := range c.Nodes {
		if d, ok := n.(*conf.Directive); ok && d.Block != nil && i > 0 {
			b.WriteByte('\n')
		}
		render(&b, n, 0)
	}
	return b.String()
}

func render(b *strings.Builder, n conf.Node, depth int) {
	indent := strings.Repeat(indentUnit, depth)
	switch v := n.(type) {
	case *conf.Comment:
		b.WriteString(indent + "#" + v.Text + "\n")
	case *conf.Blank:
		b.WriteByte('\n')
	case *conf.Directive:
		value := directiveValue(v)
		switch {
		case v.Block != nil:
			b.WriteString(indent + v.Name)
			if value != "" {
				b.WriteString(" " + value)
			}
			b.WriteString(" {\n")
			for _, child := range v.Nodes {
				render(b, child, depth+1)
			}
			b.WriteString(indent + "}\n")
		case len(v.Args) > 0 && v.Args[0].Quote == conf.QuoteHeredoc:
			sign := "END_" + v.Name
			_, _ = fmt.Fprintf(b, "%s%s <<<%s\n%s\n%s\n", indent, v.Name, sign, v.Args[0].Value, sign)
		case value == "":
			b.WriteString(indent + v.Name + "\n")
		default:
			_, _ = fmt.Fprintf(b, "%s%-24s %s\n", indent, v.Name, value)
		}
	}
}

// directiveValue 参数按空格拼成一行，跳过空参数
func directiveValue(d *conf.Directive) string {
	var parts []string
	for _, a := range d.Args {
		if a.Value != "" {
			parts = append(parts, a.Value)
		}
	}
	return strings.Join(parts, " ")
}
