package apache

import (
	"fmt"
	"os"
	"strings"

	"github.com/acepanel/panel/v3/pkg/webserver/conf"
)

type parseOptions struct {
	// Tolerant 容错模式：遇到结构错误（孤立闭合标签、未闭合块）记录后尽力继续，不致命
	Tolerant bool
}

type parser struct {
	lines []string
	pos   int
	opts  parseOptions
}

// ParseString 从字符串解析配置或片段（容错模式）
func ParseString(content string) (*conf.Config, error) {
	return parse(content, parseOptions{Tolerant: true})
}

// ParseFile 从文件解析配置（容错模式）
func ParseFile(filename string) (*conf.Config, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return parse(string(content), parseOptions{Tolerant: true})
}

func parse(content string, opts parseOptions) (*conf.Config, error) {
	p := &parser{lines: scanLogicalLines(content), opts: opts}
	nodes, err := p.parseNodes("")
	if err != nil {
		return nil, err
	}
	return &conf.Config{Nodes: nodes}, nil
}

// parseNodes 解析一段节点，直到遇到 closeName 的闭合标签或输入耗尽
// closeName 为空表示顶层；返回时不消费闭合标签，交由调用者消费
func (p *parser) parseNodes(closeName string) ([]conf.Node, error) {
	var out []conf.Node

	for p.pos < len(p.lines) {
		t := p.lines[p.pos]

		switch {
		case strings.HasPrefix(t, "#"):
			out = append(out, &conf.Comment{Text: t[1:]})
			p.pos++

		case strings.HasPrefix(t, "</"):
			name := parseCloseTag(t)
			if closeName == "" {
				if p.opts.Tolerant {
					p.pos++ // 顶层孤立闭合标签：跳过
					continue
				}
				return nil, fmt.Errorf("unexpected </%s> at top level", name)
			}
			if !strings.EqualFold(name, closeName) && !p.opts.Tolerant {
				return nil, fmt.Errorf("mismatched closing tag: expected </%s>, got </%s>", closeName, name)
			}
			return out, nil // 交给调用者消费闭合标签

		case strings.HasPrefix(t, "<"):
			name, argStr := parseOpenTag(t)
			p.pos++ // 进入块体
			children, err := p.parseNodes(name)
			if err != nil {
				return nil, err
			}
			if p.pos < len(p.lines) {
				p.pos++ // 消费匹配的闭合标签
			} else if !p.opts.Tolerant {
				return nil, fmt.Errorf("unclosed <%s>", name)
			}
			out = append(out, &conf.Directive{Name: name, Args: tokenizeLine(argStr), Block: &conf.Block{Nodes: children}})

		default:
			toks := tokenizeLine(t)
			if len(toks) == 0 {
				p.pos++
				continue
			}
			out = append(out, &conf.Directive{
				Name: toks[0].Value,
				Args: toks[1:],
			})
			p.pos++
		}
	}

	if closeName != "" && !p.opts.Tolerant {
		return nil, fmt.Errorf("unclosed <%s>", closeName)
	}
	return out, nil
}

// parseOpenTag 解析起始标签 <Name args...>，返回块名与参数串
func parseOpenTag(t string) (name, argStr string) {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "<"), ">"))
	if idx := strings.IndexAny(inner, " \t"); idx >= 0 {
		return inner[:idx], strings.TrimSpace(inner[idx+1:])
	}
	return inner, ""
}

// parseCloseTag 解析闭合标签 </Name>，返回块名
func parseCloseTag(t string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "</"), ">"))
}
