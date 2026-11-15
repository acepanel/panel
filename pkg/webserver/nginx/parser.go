package nginx

import (
	"fmt"
	"os"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
)

// nginx 配置：分号结尾的指令、花括号块、单双引号（反斜杠转义）、# 注释，*_by_lua_block 的块体按 Lua 词法原样保留

type parser struct {
	s    string
	i    int
	line int
}

func Parse(content string) (*conf.Config, error) {
	p := &parser{s: strings.ReplaceAll(content, "\r\n", "\n"), line: 1}
	nodes, err := p.parseNodes(true)
	if err != nil {
		return nil, err
	}
	return &conf.Config{Nodes: nodes}, nil
}

func ParseFile(path string) (*conf.Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(content))
}

func (p *parser) parseNodes(top bool) ([]conf.Node, error) {
	var nodes []conf.Node
	for {
		p.skipSpace()
		if p.i >= len(p.s) {
			if !top {
				return nil, p.errorf("unclosed block")
			}
			return nodes, nil
		}
		switch p.s[p.i] {
		case '#':
			nodes = append(nodes, &conf.Comment{Text: p.readLine()[1:]})
		case '}':
			if top {
				return nil, p.errorf("unexpected '}'")
			}
			p.i++
			return nodes, nil
		case ';', '{':
			return nil, p.errorf("unexpected '%c'", p.s[p.i])
		default:
			d, err := p.parseDirective()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, d)
		}
	}
}

func (p *parser) parseDirective() (*conf.Directive, error) {
	d := &conf.Directive{}
	// map 等块里的键可能带引号
	if ch := p.s[p.i]; ch == '"' || ch == '\'' {
		name, err := p.readQuoted(ch)
		if err != nil {
			return nil, err
		}
		d.Name = name
	} else {
		d.Name = p.readWord()
	}
	for {
		p.skipSpace()
		if p.i >= len(p.s) {
			return nil, p.errorf("unexpected end of file in directive %q", d.Name)
		}
		switch ch := p.s[p.i]; ch {
		case ';':
			p.i++
			d.Trailing = p.trailingComment()
			return d, nil
		case '{':
			p.i++
			if isLuaBlock(d.Name) {
				raw, err := p.readLuaBlock()
				if err != nil {
					return nil, err
				}
				d.Args = append(d.Args, conf.Arg{Value: raw, Quote: conf.QuoteRaw})
				d.Trailing = p.trailingComment()
				return d, nil
			}
			d.Trailing = p.trailingComment()
			children, err := p.parseNodes(false)
			if err != nil {
				return nil, err
			}
			d.Block = &conf.Block{Nodes: children}
			return d, nil
		case '}':
			return nil, p.errorf("unexpected '}' in directive %q", d.Name)
		case '#':
			p.readLine()
		case '"', '\'':
			val, err := p.readQuoted(ch)
			if err != nil {
				return nil, err
			}
			quote := conf.QuoteDouble
			if ch == '\'' {
				quote = conf.QuoteSingle
			}
			d.Args = append(d.Args, conf.Arg{Value: val, Quote: quote})
		default:
			d.Args = append(d.Args, conf.Arg{Value: p.readWord(), Quote: conf.QuoteNone})
		}
	}
}

// readWord 读取裸词，直到空白或结构字符；反斜杠连同其后一个字符原样保留
func (p *parser) readWord() string {
	start := p.i
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\r', '\n', ';':
			return p.s[start:p.i]
		case '{':
			if p.i == start || p.s[p.i-1] != '$' {
				return p.s[start:p.i]
			}
		case '}':
			if p.i == start {
				return p.s[start:p.i]
			}
		case '\\':
			p.i++
		}
		p.i++
	}
	return p.s[start:]
}

// readQuoted 读取引号串并解转义：\" \' \\ 取其后字符，\t \r \n 取控制符，其余反斜杠序列原样保留
func (p *parser) readQuoted(quote byte) (string, error) {
	var b strings.Builder
	p.i++
	for p.i < len(p.s) {
		ch := p.s[p.i]
		switch {
		case ch == quote:
			p.i++
			return b.String(), nil
		case ch == '\\' && p.i+1 < len(p.s):
			next := p.s[p.i+1]
			switch next {
			case '"', '\'', '\\':
				b.WriteByte(next)
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'n':
				b.WriteByte('\n')
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
			}
			if next == '\n' {
				p.line++
			}
			p.i += 2
		default:
			if ch == '\n' {
				p.line++
			}
			b.WriteByte(ch)
			p.i++
		}
	}
	return "", p.errorf("unterminated quoted string")
}

func (p *parser) trailingComment() string {
	j := p.i
	for j < len(p.s) && (p.s[j] == ' ' || p.s[j] == '\t') {
		j++
	}
	if j >= len(p.s) || p.s[j] != '#' {
		return ""
	}
	p.i = j
	return p.readLine()[1:]
}

// readLuaBlock 读取 Lua 块体直到配对的 }，跳过 Lua 字符串、长括号与注释里的花括号
func (p *parser) readLuaBlock() (string, error) {
	start, depth := p.i, 1
	for p.i < len(p.s) {
		ch := p.s[p.i]
		switch {
		case ch == '\n':
			p.line++
			p.i++
		case ch == '{':
			depth++
			p.i++
		case ch == '}':
			depth--
			if depth == 0 {
				raw := p.s[start:p.i]
				p.i++
				return raw, nil
			}
			p.i++
		case ch == '-' && strings.HasPrefix(p.s[p.i:], "--"):
			if level, ok := longBracket(p.s[p.i+2:]); ok {
				p.skipLongBracket(level)
			} else {
				p.readLine()
			}
		case ch == '[':
			if level, ok := longBracket(p.s[p.i:]); ok {
				p.skipLongBracket(level)
			} else {
				p.i++
			}
		case ch == '"' || ch == '\'':
			p.skipLuaString(ch)
		default:
			p.i++
		}
	}
	return "", p.errorf("unclosed lua block")
}

// skipLongBracket 跳过整个长括号，含注释形式
func (p *parser) skipLongBracket(level int) {
	if strings.HasPrefix(p.s[p.i:], "--") {
		p.i += 2
	}
	closing := "]" + strings.Repeat("=", level) + "]"
	p.i += level + 2
	end := strings.Index(p.s[p.i:], closing)
	if end < 0 {
		p.line += strings.Count(p.s[p.i:], "\n")
		p.i = len(p.s)
		return
	}
	p.line += strings.Count(p.s[p.i:p.i+end], "\n")
	p.i += end + len(closing)
}

func (p *parser) skipLuaString(quote byte) {
	p.i++
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case '\\':
			if p.i+1 < len(p.s) && p.s[p.i+1] == '\n' {
				p.line++
			}
			p.i += 2
			continue
		case '\n':
			p.line++
		case quote:
			p.i++
			return
		}
		p.i++
	}
}

func (p *parser) skipSpace() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case '\n':
			p.line++
		case ' ', '\t', '\r':
		default:
			return
		}
		p.i++
	}
}

// readLine 读到行尾，不消费换行符
func (p *parser) readLine() string {
	end := strings.IndexByte(p.s[p.i:], '\n')
	if end < 0 {
		end = len(p.s) - p.i
	}
	text := p.s[p.i : p.i+end]
	p.i += end
	return text
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.line, fmt.Sprintf(format, args...))
}

func isLuaBlock(name string) bool {
	return strings.HasSuffix(name, "_by_lua_block")
}

// longBracket 判断是否为 Lua 长括号 [=*[，返回等号数量
func longBracket(s string) (int, bool) {
	if !strings.HasPrefix(s, "[") {
		return 0, false
	}
	var level int
	for level+1 < len(s) && s[level+1] == '=' {
		level++
	}
	if level+1 < len(s) && s[level+1] == '[' {
		return level, true
	}
	return 0, false
}
