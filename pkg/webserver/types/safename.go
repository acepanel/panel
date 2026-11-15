package types

import (
	"fmt"
	"strings"
)

// SafeName 将名称转换为标识符安全形式，编码可逆，避免 a-b 与 a_b、demo.com 与 demo-com 撞名
func SafeName(name string) string {
	var b strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			b.WriteRune(char)
		case char == '_':
			b.WriteString("__")
		default:
			_, _ = fmt.Fprintf(&b, "_%02x", char)
		}
	}
	return b.String()
}
