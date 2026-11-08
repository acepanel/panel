//go:build linux

package scan

import (
	"log/slog"
	"os"
	"testing"

	"github.com/leonelquinteros/gotext"
)

func TestSupportedAndAttach(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("需要 root")
	}
	if !Supported() {
		t.Skip("内核不支持 eBPF 扫描检测(TCX 需 6.6+)")
	}

	s, err := New([]string{"lo"}, gotext.NewLocale("", "en"), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	t.Log("eBPF 扫描检测器: 程序加载 + TCX 挂载 + 端口同步 ✓")
}
