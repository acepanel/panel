package tui

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gookit/color"
)

// Input 单行输入，secret 时以星号回显，内容超出宽度时只显示尾部，校验失败在下一行提示
func (p *prompt) Input(
	ctx context.Context,
	label, meta, placeholder string,
	secret bool,
	validate func(string) error,
) (string, error) {
	restore, err := p.raw()
	if err != nil {
		return "", err
	}
	defer restore()

	var buf []rune
	errMsg := ""
	for {
		p.drawInput(label, meta, placeholder, masked(buf, secret), errMsg)

		k, r, err := p.readKey(ctx)
		if err != nil {
			fmt.Print("\r\x1b[J")
			return "", err
		}
		errMsg = ""
		switch k {
		case keyRune:
			buf = append(buf, r)
		case keyBackspace:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
			}
		case keyClear:
			buf = nil
		case keyEnter:
			value := string(buf)
			if err = validate(value); err != nil {
				errMsg = err.Error()
				continue
			}
			fmt.Print("\r\x1b[J")
			answer(label, meta, answered(masked(buf, secret), placeholder))
			return value, nil
		case keyEsc:
			fmt.Print("\r\x1b[J")
			return "", errBack
		case keyAbort:
			fmt.Print("\r\x1b[J")
			return "", errAbort
		default:
		}
	}
}

// drawInput 从当前行起清屏并重绘，绘制完成后光标停在输入尾部所在的第一行，
// 因此下次重绘与收尾都只需从当前行清到屏幕末尾
func (p *prompt) drawInput(label, meta, placeholder, text, errMsg string) {
	cols := p.termCols()
	prefix := plainTitle(label, meta)
	// 只保留能放下的尾部，保证光标始终可见
	for text != "" && displayWidth(prefix)+displayWidth(text) >= cols {
		_, size := utf8.DecodeRuneInString(text)
		text = text[size:]
	}

	line := title(label, meta) + text
	if text == "" && placeholder != "" {
		line += color.Gray.Sprint(placeholder)
	}
	out := "\r\x1b[J" + fit(line, cols)
	if errMsg != "" {
		out += "\r\n" + fit(color.Red.Sprint("✘ "+errMsg), cols) + "\x1b[1A"
	}
	out += "\r"
	if col := displayWidth(prefix) + displayWidth(text); col > 0 {
		out += fmt.Sprintf("\x1b[%dC", col)
	}
	fmt.Print(out)
}

// masked 密码以星号回显
func masked(buf []rune, secret bool) string {
	if secret {
		return strings.Repeat("*", len(buf))
	}

	return string(buf)
}

// answered 回显输入值，留空时显示灰色的默认值或占位横线
func answered(text, placeholder string) string {
	if text != "" {
		return color.Cyan.Sprint(text)
	}
	if placeholder != "" {
		return color.Gray.Sprint(placeholder)
	}

	return color.Gray.Sprint("—")
}
