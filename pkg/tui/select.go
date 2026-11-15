package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/gookit/color"
)

// item 列表项，Desc 以灰色显示在标签之后
type item struct {
	Label string
	Desc  string
}

// selector 列表选择过程中的状态
type selector struct {
	p       *prompt
	label   string
	meta    string
	items   []item
	matched []int
	query   []rune
	cursor  int
	top     int
	pad     int
}

// Select 方向键选择列表项，输入字符即时过滤，返回所选项在 items 中的下标
func (p *prompt) Select(ctx context.Context, label, meta string, items []item, def int) (int, error) {
	restore, err := p.raw()
	if err != nil {
		return 0, err
	}
	defer restore()
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	s := &selector{p: p, label: label, meta: meta, items: items}
	for _, item := range items {
		s.pad = max(s.pad, displayWidth(item.Label))
	}
	s.filter()
	s.cursor = def

	prev := 0
	for {
		lines := s.render()
		redraw(prev, lines)
		prev = len(lines)

		k, r, err := p.readKey(ctx)
		if err != nil {
			redraw(prev, nil)
			return 0, err
		}
		switch k {
		case keyUp:
			if s.cursor > 0 {
				s.cursor--
			}
		case keyDown:
			if s.cursor < len(s.matched)-1 {
				s.cursor++
			}
		case keyRune:
			s.query = append(s.query, r)
			s.filter()
		case keyBackspace:
			if len(s.query) > 0 {
				s.query = s.query[:len(s.query)-1]
				s.filter()
			}
		case keyClear:
			s.query = nil
			s.filter()
		case keyEnter:
			if len(s.matched) == 0 {
				continue
			}
			redraw(prev, nil)
			answer(label, meta, color.Cyan.Sprint(items[s.matched[s.cursor]].Label))
			return s.matched[s.cursor], nil
		case keyEsc:
			redraw(prev, nil)
			return 0, errBack
		case keyAbort:
			redraw(prev, nil)
			return 0, errAbort
		default:
		}
	}
}

// filter 按标签或说明包含查询串筛选，忽略大小写，筛选后光标回到顶部
func (s *selector) filter() {
	query := strings.ToLower(string(s.query))
	s.matched = s.matched[:0]
	for i, item := range s.items {
		if strings.Contains(strings.ToLower(item.Label+" "+item.Desc), query) {
			s.matched = append(s.matched, i)
		}
	}
	s.cursor, s.top = 0, 0
}

// render 输出标题、可视窗口内的列表项与按键提示，窗口随光标滚动，每行截断到终端宽度
func (s *selector) render() []string {
	cols := s.p.termCols()
	rows := max(s.p.termRows()-4, 1)
	if s.cursor < s.top {
		s.top = s.cursor
	}
	if s.cursor >= s.top+rows {
		s.top = s.cursor - rows + 1
	}

	lines := []string{fit(title(s.label, s.meta)+color.Cyan.Sprint(string(s.query)), cols)}
	if len(s.matched) == 0 {
		lines = append(lines, fit(color.Gray.Sprint("  "+s.p.t.Get("No matches")), cols))
	}
	for i := s.top; i < len(s.matched) && i < s.top+rows; i++ {
		item := s.items[s.matched[i]]
		name := item.Label + strings.Repeat(" ", s.pad-displayWidth(item.Label))
		if i == s.cursor {
			lines = append(lines, fit(color.Cyan.Sprint("❯ "+name+"  "+item.Desc), cols))
		} else {
			lines = append(lines, fit("  "+name+"  "+color.Gray.Sprint(item.Desc), cols))
		}
	}
	lines = append(lines, fit(
		color.Gray.Sprint(s.p.t.Get("↑/↓ select · Enter confirm · type to filter · Esc back · Ctrl+C exit")),
		cols,
	))

	return lines
}
