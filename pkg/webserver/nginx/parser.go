package nginx

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/dumper"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

// Parser Nginx vhost 配置解析器
type Parser struct {
	cfg     *config.Config
	cfgPath string // 配置文件路径
}

// NewParser 使用网站名创建解析器，将默认配置中的 default 替换为实际网站名
func NewParser(siteName string) (*Parser, error) {
	str := strings.ReplaceAll(DefaultConf, "/opt/ace/sites/default", fmt.Sprintf("/opt/ace/sites/%s", siteName))

	p := parser.NewStringParser(str, parser.WithSkipIncludeParsingErr(), parser.WithSkipValidDirectivesErr())
	cfg, err := p.Parse()
	if err != nil {
		return nil, err
	}

	return &Parser{cfg: cfg, cfgPath: ""}, nil
}

// NewParserFromFile 从指定文件路径创建解析器
func NewParserFromFile(filePath string) (*Parser, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	p := parser.NewStringParser(string(content), parser.WithSkipIncludeParsingErr(), parser.WithSkipValidDirectivesErr())
	cfg, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &Parser{cfg: cfg, cfgPath: filePath}, nil
}

// NewParserFromString 从字符串创建解析器
func NewParserFromString(content string) (*Parser, error) {
	p := parser.NewStringParser(content, parser.WithSkipIncludeParsingErr(), parser.WithSkipValidDirectivesErr())
	cfg, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &Parser{cfg: cfg, cfgPath: ""}, nil
}

func (p *Parser) Config() *config.Config {
	return p.cfg
}

// Find 查找指令，如: Find("server.listen")
func (p *Parser) Find(key string) ([]config.IDirective, error) {
	parts := strings.Split(key, ".")
	var block *config.Block
	var ok bool
	block = p.cfg.Block
	for i := 0; i < len(parts)-1; i++ {
		key = parts[i]
		directives := block.FindDirectives(key)
		if len(directives) == 0 {
			return nil, fmt.Errorf("given key %s not found", key)
		}
		if len(directives) > 1 {
			return nil, errors.New("multiple directives found")
		}
		block, ok = directives[0].GetBlock().(*config.Block)
		if !ok {
			return nil, errors.New("block is not *config.Block")
		}
	}

	var result []config.IDirective
	for _, dir := range block.GetDirectives() {
		if dir.GetName() == parts[len(parts)-1] {
			result = append(result, dir)
		}
	}

	return result, nil
}

// FindOne 查找单个指令，如: FindOne("server.server_name")
func (p *Parser) FindOne(key string) (config.IDirective, error) {
	directives, err := p.Find(key)
	if err != nil {
		return nil, err
	}
	if len(directives) == 0 {
		return nil, fmt.Errorf("given key %s not found", key)
	}
	if len(directives) > 1 {
		return nil, fmt.Errorf("multiple directives found for %s", key)
	}

	return directives[0], nil
}

// Clear 移除指令，如: Clear("server.server_name")
func (p *Parser) Clear(key string) error {
	parts := strings.Split(key, ".")
	last := parts[len(parts)-1]
	parts = parts[:len(parts)-1]

	var block *config.Block
	var ok bool
	block = p.cfg.Block
	for i := 0; i < len(parts); i++ {
		directives := block.FindDirectives(parts[i])
		if len(directives) == 0 {
			return fmt.Errorf("given key %s not found", parts[i])
		}
		if len(directives) > 1 {
			return fmt.Errorf("multiple directives found for %s", parts[i])
		}
		block, ok = directives[0].GetBlock().(*config.Block)
		if !ok {
			return errors.New("block is not *config.Block")
		}
	}

	var newDirectives []config.IDirective
	for _, directive := range block.GetDirectives() {
		if directive.GetName() != last {
			newDirectives = append(newDirectives, directive)
		}
	}
	block.Directives = newDirectives

	return nil
}

// Set 设置指令，如: Set("server.index", []Directive{...})
func (p *Parser) Set(key string, directives []*config.Directive, after ...string) error {
	parts := strings.Split(key, ".")

	var block *config.Block
	var blockDirective config.IDirective
	var ok bool
	block = p.cfg.Block
	for i := 0; i < len(parts); i++ {
		sub := block.FindDirectives(parts[i])
		if len(sub) == 0 {
			return fmt.Errorf("given key %s not found", parts[i])
		}
		if len(sub) > 1 {
			return fmt.Errorf("multiple directives found for %s", parts[i])
		}
		block, ok = sub[0].GetBlock().(*config.Block)
		if !ok {
			return errors.New("block is not *config.Block")
		}
		blockDirective = sub[0]
	}

	iDirectives := make([]config.IDirective, 0, len(directives))
	for _, directive := range directives {
		directive.SetParent(blockDirective)
		iDirectives = append(iDirectives, directive)
	}

	if len(after) == 0 {
		block.Directives = append(block.Directives, iDirectives...)
	} else {
		insertIndex := -1
		for i, d := range block.Directives {
			if d.GetName() == after[0] {
				insertIndex = i + 1
				break
			}
		}
		if insertIndex == -1 {
			return fmt.Errorf("after directive %s not found", after[0])
		}

		block.Directives = append(
			block.Directives[:insertIndex],
			append(iDirectives, block.Directives[insertIndex:]...)...,
		)
	}

	return nil
}

// SetOne 设置单个指令，如: SetOne("server.listen", []string{"80"})
func (p *Parser) SetOne(key string, params []string) error {
	parts := strings.Split(key, ".")
	if len(parts) < 2 {
		return fmt.Errorf("key must have at least 2 parts: %s", key)
	}

	directiveName := parts[len(parts)-1]
	blockKey := strings.Join(parts[:len(parts)-1], ".")

	return p.Set(blockKey, []*config.Directive{
		{
			Name:       directiveName,
			Parameters: p.slices2Parameters(params),
		},
	})
}

// Dump 将指令结构导出为配置内容
func (p *Parser) Dump() string {
	return dumper.DumpConfig(p.cfg, dumper.IndentedStyle)
}

// Save 保存配置到文件
func (p *Parser) Save() error {
	p.sortDirectives(p.cfg.Directives, order)
	content := p.Dump() + "\n"
	if err := os.WriteFile(p.cfgPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

// SetConfigPath 设置配置文件路径
func (p *Parser) SetConfigPath(path string) {
	p.cfgPath = path
}

func (p *Parser) sortDirectives(directives []config.IDirective, orderIndex map[string]int) {
	slices.SortStableFunc(directives, func(a config.IDirective, b config.IDirective) int {
		// 块指令（如 server、location）应该排在普通指令（如 include）后面
		aIsBlock := a.GetBlock() != nil && len(a.GetBlock().GetDirectives()) > 0
		bIsBlock := b.GetBlock() != nil && len(b.GetBlock().GetDirectives()) > 0

		if aIsBlock != bIsBlock {
			if aIsBlock {
				return 1 // a 是块指令，排在后面
			}
			return -1 // b 是块指令，排在前面
		}

		// 同类指令，按 order 排序；相同名称的指令保持原有顺序
		return orderIndex[a.GetName()] - orderIndex[b.GetName()]
	})

	for _, directive := range directives {
		if block, ok := directive.GetBlock().(*config.Block); ok {
			p.sortDirectives(block.Directives, orderIndex)
		}
	}
}

func (p *Parser) slices2Parameters(slices []string) []config.Parameter {
	var parameters []config.Parameter
	for _, slice := range slices {
		parameters = append(parameters, config.Parameter{Value: slice})
	}
	return parameters
}

func (p *Parser) parameters2Slices(parameters []config.Parameter) []string {
	var s []string
	for _, parameter := range parameters {
		s = append(s, parameter.Value)
	}
	return s
}
