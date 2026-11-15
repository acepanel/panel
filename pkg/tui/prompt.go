package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gookit/color"
	"github.com/leonelquinteros/gotext"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"golang.org/x/text/width"
)

var (
	// errBack 用户按下 Esc 要求返回上一级
	errBack = errors.New("back")
	// errAbort 用户按下 Ctrl+C 或上下文已取消，要求退出
	errAbort = errors.New("abort")
)

type key int

const (
	keyRune key = iota
	keyEnter
	keyUp
	keyDown
	keyBackspace
	keyClear
	keyEsc
	keyAbort
	keyOther
)

// 等待按键时每隔 pollStep 毫秒检查一次上下文；转义序列的后续字节最多等 escapeWait 毫秒
const (
	pollStep   = 200
	escapeWait = 50
)

// prompt 封装原始模式下的按键读取与行绘制。stdin 逐字节读取、不做预读，
// 以免抢走随后执行的命令（例如交互输入密码）要读的内容
type prompt struct {
	t  *gotext.Locale
	in *os.File
	fd int
}

func newPrompt(t *gotext.Locale) *prompt {
	return &prompt{t: t, in: os.Stdin, fd: int(os.Stdin.Fd())}
}

// raw 进入原始模式并返回恢复函数，期间回显与信号均由程序自行处理
func (p *prompt) raw() (func(), error) {
	state, err := term.MakeRaw(p.fd)
	if err != nil {
		return nil, err
	}

	return func() { _ = term.Restore(p.fd, state) }, nil
}

// readKey 读取一次按键：先等到 stdin 可读，期间上下文被取消即中止
func (p *prompt) readKey(ctx context.Context) (key, rune, error) {
	for {
		if ctx.Err() != nil {
			return keyAbort, 0, errAbort
		}
		ready, err := p.ready(pollStep)
		if err != nil {
			return keyAbort, 0, err
		}
		if ready {
			break
		}
	}

	r, err := p.readRune()
	if err != nil {
		return keyAbort, 0, err
	}
	switch r {
	case '\r', '\n':
		return keyEnter, 0, nil
	case 0x03:
		return keyAbort, 0, nil
	case 0x04:
		return keyEsc, 0, nil
	case 0x7f, 0x08:
		return keyBackspace, 0, nil
	case 0x15:
		return keyClear, 0, nil
	case 0x1b:
		return p.readEscape()
	}
	if unicode.IsPrint(r) {
		return keyRune, r, nil
	}

	return keyOther, 0, nil
}

// ready 等待至多 timeout 毫秒看 stdin 是否可读，被信号打断按未就绪处理
func (p *prompt) ready(timeout int) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(p.fd), Events: unix.POLLIN}} //nolint:gosec // 标准输入描述符不会溢出
	n, err := unix.Poll(fds, timeout)
	if errors.Is(err, unix.EINTR) {
		return false, nil
	}

	return n > 0, err
}

// readRune 逐字节拼出一个 UTF-8 字符
func (p *prompt) readRune() (rune, error) {
	var buf [utf8.UTFMax]byte
	for n := 1; n <= len(buf); n++ {
		if _, err := p.in.Read(buf[n-1 : n]); err != nil {
			return 0, err
		}
		if utf8.FullRune(buf[:n]) {
			r, _ := utf8.DecodeRune(buf[:n])
			return r, nil
		}
	}

	return utf8.RuneError, nil
}

// readEscape 区分单独的 Esc 与转义序列：终端把整段序列一次写入，短暂等待后没有后续字节即视为 Esc
func (p *prompt) readEscape() (key, rune, error) {
	b, ok, err := p.next()
	if err != nil {
		return keyAbort, 0, err
	}
	if !ok || (b != '[' && b != 'O') {
		return keyEsc, 0, nil
	}

	// CSI 序列以 0x40-0x7e 范围内的字节结束
	for {
		if b, ok, err = p.next(); err != nil {
			return keyAbort, 0, err
		}
		if !ok {
			return keyOther, 0, nil
		}
		if b >= 0x40 && b <= 0x7e {
			break
		}
	}
	switch b {
	case 'A':
		return keyUp, 0, nil
	case 'B':
		return keyDown, 0, nil
	}

	return keyOther, 0, nil
}

// next 在 escapeWait 毫秒内读取转义序列的下一个字节
func (p *prompt) next() (byte, bool, error) {
	ready, err := p.ready(escapeWait)
	if err != nil || !ready {
		return 0, false, err
	}
	var b [1]byte
	if _, err = p.in.Read(b[:]); err != nil {
		return 0, false, err
	}

	return b[0], true, nil
}

// termCols 终端列数，取不到时按 80 列处理
func (p *prompt) termCols() int {
	w, _, err := term.GetSize(p.fd)
	if err != nil || w <= 0 {
		return 80
	}

	return w
}

// termRows 终端行数，取不到时按 24 行处理
func (p *prompt) termRows() int {
	_, h, err := term.GetSize(p.fd)
	if err != nil || h <= 0 {
		return 24
	}

	return h
}

// redraw 清掉上次绘制的 prev 行后重新输出，原始模式下换行必须带回车
func redraw(prev int, lines []string) {
	var b strings.Builder
	if prev > 0 {
		_, _ = fmt.Fprintf(&b, "\x1b[%dA", prev)
	}
	b.WriteString("\r\x1b[J")
	for _, line := range lines {
		b.WriteString(line + "\r\n")
	}
	fmt.Print(b.String())
}

// answer 以固定样式回显已确认的答案，后续输出接在其下方
func answer(label, meta, value string) {
	fmt.Print(title(label, meta) + value + "\r\n")
}

// title 拼接问题行前缀：绿色问号、加粗标签、灰色补充说明
func title(label, meta string) string {
	s := color.Green.Sprint("? ") + color.Bold.Sprint(label)
	if meta != "" {
		s += " " + color.Gray.Sprint(meta)
	}

	return s + " › "
}

// plainTitle 与 title 内容一致的无样式文本，用于计算显示宽度
func plainTitle(label, meta string) string {
	s := "? " + label
	if meta != "" {
		s += " " + meta
	}

	return s + " › "
}

// runeWidth 按东亚宽度表估算显示列数，组合字符与格式字符不占列
func runeWidth(r rune) int {
	if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) {
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

// displayWidth 无样式文本的显示列数
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}

	return w
}

// fit 把带颜色的行截断到 cols 列：ANSI 序列不计宽，截断处补复位码避免样式外溢，
// 保证每个逻辑行只占一个物理行，重绘时的行数计算才可靠
func fit(s string, cols int) string {
	var b strings.Builder
	w := 0
	for s != "" {
		if strings.HasPrefix(s, "\x1b[") {
			end := strings.IndexByte(s, 'm')
			if end < 0 {
				break
			}
			b.WriteString(s[:end+1])
			s = s[end+1:]
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		w += runeWidth(r)
		if w > cols {
			return b.String() + "\x1b[0m"
		}
		b.WriteString(s[:size])
		s = s[size:]
	}

	return b.String()
}
