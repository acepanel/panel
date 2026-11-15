package caddy

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
)

// Caddyfile 文本模型：每行一条指令，首个 token 为名称、其余为参数；行尾单独的 `{` 开块，`}` 独占一行收块；
// 顶层带块的行是站点块（Name 与 Args 为地址列表）、片段 `(name)` 或无名的全局选项块；
// `#` 开头的 token 至行尾为注释；参数支持双引号、反引号与 heredoc，行尾反斜杠续行

var heredocMarker = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type token struct {
	text    string
	line    int
	quoted  bool // 引号、反引号或 heredoc 包裹，不参与块结构判断
	comment bool
}

func (t token) quoteStyle() conf.Quote {
	if t.quoted {
		return conf.QuoteDouble
	}
	return conf.QuoteNone
}

func Parse(content string) (*conf.Config, error) {
	tokens, err := lex(content)
	if err != nil {
		return nil, err
	}

	cfg := &conf.Config{}
	stack := []*conf.Block{&cfg.Block}
	for i := 0; i < len(tokens); {
		var toks []token
		var trailing string
		line := tokens[i].line
		// 顶层地址列表以逗号结尾时跨行继续
		for i < len(tokens) {
			line = tokens[i].line
			for ; i < len(tokens) && tokens[i].line == line; i++ {
				if tokens[i].comment {
					trailing = tokens[i].text
					continue
				}
				toks = append(toks, tokens[i])
			}
			if len(stack) > 1 || len(toks) == 0 || !strings.HasSuffix(toks[len(toks)-1].text, ",") {
				break
			}
		}
		cur := stack[len(stack)-1]

		// 没有指令 token 的行只可能是纯注释行
		if len(toks) == 0 {
			cur.Append(&conf.Comment{Text: trailing})
			continue
		}
		if !toks[0].quoted && toks[0].text == "}" {
			if len(toks) > 1 {
				return nil, fmt.Errorf("line %d: unexpected token after '}'", line)
			}
			if len(stack) == 1 {
				return nil, fmt.Errorf("line %d: unexpected '}'", line)
			}
			stack = stack[:len(stack)-1]
			if trailing != "" {
				stack[len(stack)-1].Append(&conf.Comment{Text: trailing})
			}
			continue
		}

		d := &conf.Directive{Trailing: trailing}
		if last := toks[len(toks)-1]; !last.quoted && last.text == "{" {
			d.Block = &conf.Block{}
			toks = toks[:len(toks)-1]
		}
		// 顶层带块且不是片段定义的行是站点块，地址以逗号分隔
		if len(stack) == 1 && d.Block != nil && len(toks) > 0 && !strings.HasPrefix(toks[0].text, "(") {
			addrs := splitAddresses(toks)
			d.Name, d.Args = addrs[0], conf.Args(addrs[1:]...)
		} else if len(toks) > 0 {
			d.Name = toks[0].text
			for _, t := range toks[1:] {
				d.Args = append(d.Args, conf.Arg{Value: t.text, Quote: t.quoteStyle()})
			}
		}
		cur.Append(d)
		if d.Block != nil {
			stack = append(stack, d.Block)
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

// lex 切分 token，逻辑行号用于分组，续行、引号与 heredoc 内的换行不增加行号
func lex(content string) ([]token, error) {
	s := strings.ReplaceAll(content, "\r\n", "\n")
	var tokens []token
	line := 1
	for i := 0; i < len(s); {
		ch := s[i]
		switch {
		case ch == '\n':
			line++
			i++
		case ch == ' ' || ch == '\t' || ch == '\r':
			i++
		case escapedNewline(s, i):
			i += 2
		case ch == '#':
			end := strings.IndexByte(s[i:], '\n')
			if end < 0 {
				end = len(s) - i
			}
			tokens = append(tokens, token{text: s[i+1 : i+end], line: line, comment: true})
			i += end
		case ch == '"':
			var val strings.Builder
			j := i + 1
			for ; j < len(s) && s[j] != '"'; j++ {
				if s[j] == '\\' && j+1 < len(s) && s[j+1] == '"' {
					j++
				}
				val.WriteByte(s[j])
			}
			if j >= len(s) {
				return nil, fmt.Errorf("line %d: unterminated quoted string", line)
			}
			tokens = append(tokens, token{text: val.String(), line: line, quoted: true})
			i = j + 1
		case ch == '`':
			end := strings.IndexByte(s[i+1:], '`')
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated backtick string", line)
			}
			tokens = append(tokens, token{text: s[i+1 : i+1+end], line: line, quoted: true})
			i += end + 2
		case ch == '<' && strings.HasPrefix(s[i:], "<<") && isHeredoc(s[i+2:]):
			nl := strings.IndexByte(s[i:], '\n')
			marker := s[i+2 : i+nl]
			body, next, err := readHeredoc(s[i+nl+1:], marker)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			tokens = append(tokens, token{text: body, line: line, quoted: true})
			i += nl + 1 + next
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t\r\n", rune(s[j])) && !escapedNewline(s, j) {
				j++
			}
			tokens = append(tokens, token{text: s[i:j], line: line})
			i = j
		}
	}
	return tokens, nil
}

func escapedNewline(s string, i int) bool {
	return s[i] == '\\' && i+1 < len(s) && s[i+1] == '\n'
}

func isHeredoc(rest string) bool {
	nl := strings.IndexByte(rest, '\n')
	return nl > 0 && heredocMarker.MatchString(rest[:nl])
}

// readHeredoc 按结束标记的缩进去掉每行前导空白，返回正文与消费的字节数
func readHeredoc(s, marker string) (string, int, error) {
	var consumed int
	var lines []string
	for consumed <= len(s) {
		end := strings.IndexByte(s[consumed:], '\n')
		var physical string
		if end < 0 {
			physical = s[consumed:]
		} else {
			physical = s[consumed : consumed+end]
		}
		if trimmed := strings.TrimLeft(physical, " \t"); strings.HasPrefix(trimmed, marker) &&
			(len(trimmed) == len(marker) || strings.ContainsRune(" \t\r", rune(trimmed[len(marker)]))) {
			indent := physical[:len(physical)-len(trimmed)]
			for i, l := range lines {
				lines[i] = strings.TrimPrefix(l, indent)
			}
			return strings.Join(lines, "\n"), consumed + len(indent) + len(marker), nil
		}
		if end < 0 {
			break
		}
		lines = append(lines, physical)
		consumed += end + 1
	}
	return "", 0, fmt.Errorf("unterminated heredoc %s", marker)
}

func splitAddresses(tokens []token) []string {
	var out []string
	for _, t := range tokens {
		for part := range strings.SplitSeq(t.text, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// Export 渲染为配置文本，顶层块之间空一行
func Export(c *conf.Config) string {
	var b strings.Builder
	for i, n := range c.Nodes {
		if d, ok := n.(*conf.Directive); ok && d.Block != nil && i > 0 {
			if _, blank := c.Nodes[i-1].(*conf.Blank); !blank {
				b.WriteByte('\n')
			}
		}
		render(&b, n, 0)
	}
	return b.String()
}

func render(b *strings.Builder, n conf.Node, depth int) {
	indent := strings.Repeat("\t", depth)
	switch v := n.(type) {
	case *conf.Comment:
		b.WriteString(indent + "#" + v.Text + "\n")
	case *conf.Blank:
		b.WriteByte('\n')
	case *conf.Directive:
		b.WriteString(indent)
		if depth == 0 && isSite(v) {
			b.WriteString(strings.Join(addresses(v), ",\n"+indent))
		} else if v.Name != "" {
			b.WriteString(v.Name)
			for _, t := range v.Values() {
				b.WriteString(" " + quote(t, indent))
			}
		}
		if v.Block != nil {
			if v.Name != "" {
				b.WriteByte(' ')
			}
			b.WriteByte('{')
		}
		if v.Trailing != "" {
			b.WriteString(" #" + v.Trailing)
		}
		b.WriteByte('\n')
		if v.Block != nil {
			for _, child := range v.Nodes {
				render(b, child, depth+1)
			}
			b.WriteString(indent + "}\n")
		}
	}
}

// quote 含换行用 heredoc，含双引号用反引号，含空白或结构字符用双引号
func quote(t, indent string) string {
	switch {
	case t == "":
		return `""`
	case strings.Contains(t, "\n"):
		lines := strings.Split(t, "\n")
		for i, l := range lines {
			lines[i] = indent + "\t" + l
		}
		return "<<EOT\n" + strings.Join(lines, "\n") + "\n" + indent + "\tEOT"
	case strings.Contains(t, `"`) && !strings.Contains(t, "`"):
		return "`" + t + "`"
	case strings.ContainsAny(t, " \t\""), t == "{", t == "}", strings.HasPrefix(t, "#"):
		return `"` + strings.ReplaceAll(t, `"`, `\"`) + `"`
	}
	return t
}

// isSite 带块且不是片段或全局选项的顶层指令，Name 与 Args 为地址列表
func isSite(d *conf.Directive) bool {
	return d.Block != nil && d.Name != "" && !strings.HasPrefix(d.Name, "(")
}

func sites(cfg *conf.Config) []*conf.Directive {
	var out []*conf.Directive
	for _, d := range cfg.All() {
		if isSite(d) {
			out = append(out, d)
		}
	}
	return out
}

func addresses(d *conf.Directive) []string {
	return append([]string{d.Name}, d.Values()...)
}

func addSite(cfg *conf.Config, addrs ...string) *conf.Directive {
	return cfg.AddBlock(addrs[0], addrs[1:]...)
}
