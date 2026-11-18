package types

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestHtaccess(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".htaccess")
	h := Htaccess(root)

	check.Equal(t, h.Read(), "")
	// 没有文件时写空内容不应凭空建一个
	must.NoError(t, h.Write(""))
	_, err := os.Stat(path)
	check.True(t, os.IsNotExist(err))

	rules := "RewriteEngine On\nRewriteRule ^ /index.php [L]\n"
	must.NoError(t, h.Write(rules))
	check.Equal(t, h.Read(), rules)
	info, err := os.Stat(path)
	must.NoError(t, err)
	check.Equal(t, info.Mode().Perm(), os.FileMode(0644))

	// 内容没变不重写文件
	old := time.Now().Add(-time.Hour)
	must.NoError(t, os.Chtimes(path, old, old))
	must.NoError(t, h.Write(rules))
	info, err = os.Stat(path)
	must.NoError(t, err)
	check.True(t, info.ModTime().Equal(old))

	must.NoError(t, h.Write(""))
	_, err = os.Stat(path)
	check.True(t, os.IsNotExist(err))
}
