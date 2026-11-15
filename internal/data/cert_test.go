package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
)

func TestCertRunScript(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	out := filepath.Join(dir, "out")
	script := "printf '%s' '{cert}|{key}' > " + out
	cert := &biz.Cert{Cert: "CERT", Key: "KEY", Script: script}

	must.NoError(t, new(certRepo).RunScript(t.Context(), cert))

	got, err := os.ReadFile(out)
	must.NoError(t, err)
	check.Equal(t, string(got), "CERT|KEY")
	// 同一个对象随后还要交给钩子，脚本模板不能被展开后的证书和私钥覆盖
	check.Equal(t, cert.Script, script)

	// 带私钥的临时脚本不能留在磁盘上
	entries, err := os.ReadDir(dir)
	must.NoError(t, err)
	check.Equal(t, len(entries), 1)
}
