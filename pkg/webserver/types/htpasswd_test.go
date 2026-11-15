package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"golang.org/x/crypto/bcrypt"
)

func TestWriteUserFile(t *testing.T) {
	userFile := filepath.Join(t.TempDir(), "site.htpasswd")
	hashed := "$apr1$abcdefgh$0123456789abcdefghijk."
	must.NoError(t, os.WriteFile(userFile, []byte(strings.Join([]string{
		"# comment",
		"alice:{PLAIN}secret",
		// 面板写入的明文口令以 $ 开头，不能被当成已加密的原样带过
		"bob:{PLAIN}$ecret",
		"carol:" + hashed,
		"",
	}, "\n")), 0600))

	has, err := WriteUserFile(userFile, ".apache", ":")
	must.NoError(t, err)
	check.True(t, has)

	content, err := os.ReadFile(userFile + ".apache")
	must.NoError(t, err)
	got := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(content)), "\n") {
		user, password, _ := strings.Cut(line, ":")
		got[user] = password
	}
	must.Equal(t, len(got), 3)
	check.NoError(t, bcrypt.CompareHashAndPassword([]byte(got["alice"]), []byte("secret")))
	check.NoError(t, bcrypt.CompareHashAndPassword([]byte(got["bob"]), []byte("$ecret")))
	check.Equal(t, got["carol"], hashed)

	// 用户文件缺失当作没有用户
	has, err = WriteUserFile(filepath.Join(t.TempDir(), "missing"), ".apache", ":")
	must.NoError(t, err)
	check.False(t, has)
}
