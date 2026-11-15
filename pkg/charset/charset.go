package charset

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	UTF8    = "utf-8"
	Auto    = "auto"
	GB18030 = "gb18030"
)

// UTF-16 与带 BOM 的 UTF-8 走 unicode 包，htmlindex 的同名编码不处理 BOM
var encodings = map[string]encoding.Encoding{
	"utf-8-bom":    unicode.UTF8BOM,
	"utf-16le":     unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	"utf-16be":     unicode.UTF16(unicode.BigEndian, unicode.UseBOM),
	GB18030:        simplifiedchinese.GB18030,
	"gbk":          simplifiedchinese.GBK,
	"big5":         traditionalchinese.Big5,
	"shift_jis":    japanese.ShiftJIS,
	"euc-jp":       japanese.EUCJP,
	"iso-2022-jp":  japanese.ISO2022JP,
	"euc-kr":       korean.EUCKR,
	"windows-1252": charmap.Windows1252,
	"windows-1251": charmap.Windows1251,
}

// UnsupportedRuneError 目标编码无法表示某个字符
type UnsupportedRuneError struct {
	Rune rune
	Line int
}

func (e *UnsupportedRuneError) Error() string {
	return fmt.Sprintf("rune %q at line %d is not supported", e.Rune, e.Line)
}

// Decode 将指定编码的字节转换为 UTF-8，编码名为空或 UTF-8 时原样返回
func Decode(data []byte, name string) ([]byte, error) {
	if name == "" || name == UTF8 {
		return data, nil
	}
	enc, ok := encodings[name]
	if !ok {
		return nil, fmt.Errorf("unsupported encoding: %s", name)
	}

	out, _, err := transform.Bytes(enc.NewDecoder(), data)
	return out, err
}

// Encode 将 UTF-8 字节转换为指定编码，编码名为空或 UTF-8 时原样返回
func Encode(data []byte, name string) ([]byte, error) {
	if name == "" || name == UTF8 {
		return data, nil
	}
	enc, ok := encodings[name]
	if !ok {
		return nil, fmt.Errorf("unsupported encoding: %s", name)
	}

	out, n, err := transform.Bytes(enc.NewEncoder(), data)
	if err != nil {
		// n 为转换中断处的字节偏移，换算成行号便于用户定位
		r, _ := utf8.DecodeRune(data[n:])
		return nil, &UnsupportedRuneError{Rune: r, Line: bytes.Count(data[:n], []byte("\n")) + 1}
	}

	return out, nil
}

// Detect 检测字节的编码，依次判断 BOM 与 UTF-8 合法性，都不匹配时回退 GB18030
func Detect(data []byte) string {
	boms := []struct {
		prefix []byte
		name   string
	}{
		{[]byte{0xEF, 0xBB, 0xBF}, "utf-8-bom"},
		{[]byte{0xFF, 0xFE}, "utf-16le"},
		{[]byte{0xFE, 0xFF}, "utf-16be"},
	}

	for _, bom := range boms {
		if bytes.HasPrefix(data, bom.prefix) {
			return bom.name
		}
	}
	if utf8.Valid(data) {
		return UTF8
	}

	return GB18030
}
