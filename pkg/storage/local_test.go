package storage

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestJoinPath(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		{"", "", ""},
		{"", "website/a.tar.gz", "website/a.tar.gz"},
		{"", "/website/a.tar.gz", "website/a.tar.gz"},
		{"backup", "", "backup"},
		{"backup", "website/a.tar.gz", "backup/website/a.tar.gz"},
		{"/data/backup", "/website/a.tar.gz", "/data/backup/website/a.tar.gz"},
		{"/", "/data/backup/website", "/data/backup/website"},
	}
	for _, tc := range cases {
		check.Equal(t, joinPath(tc.base, tc.path), tc.want, check.Msgf("joinPath(%q, %q)", tc.base, tc.path))
	}
}

func TestLocalList(t *testing.T) {
	dir := t.TempDir()
	must.NoError(t, os.WriteFile(filepath.Join(dir, "a.tar.gz"), []byte("12345"), 0644))
	must.NoError(t, os.WriteFile(filepath.Join(dir, "b.tar.gz"), []byte("123"), 0644))
	must.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0755))

	local, err := NewLocal(dir)
	must.NoError(t, err)
	files, err := local.List(t.Context(), "")
	must.NoError(t, err)
	slices.SortFunc(files, func(a, b File) int { return strings.Compare(a.Name, b.Name) })

	must.Len(t, files, 2, check.Msgf("目录不应出现在结果里"))
	check.Equal(t, files[0].Name, "a.tar.gz")
	check.Equal(t, files[0].Size, int64(5))
	check.Equal(t, files[1].Name, "b.tar.gz")
	check.Equal(t, files[1].Size, int64(3))
	check.False(t, files[0].ModTime.IsZero(), check.Msgf("应带上修改时间"))
}

// sizedReader 冒充一个大小已知的文件
type sizedReader struct {
	*strings.Reader
	size int64
}

func (r sizedReader) Stat() (fs.FileInfo, error) { return sizedInfo{size: r.size}, nil }

type sizedInfo struct {
	fs.FileInfo
	size int64
}

func (i sizedInfo) Size() int64 { return i.size }

func TestLocalPutChecksFreeSpace(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocal(dir)
	must.NoError(t, err)

	// 装不下的在动笔之前就拒绝，不留半截文件
	err = local.Put(t.Context(), "website/huge.tar.xz", sizedReader{Reader: strings.NewReader("x"), size: 1 << 62})
	check.ErrorContains(t, err, "insufficient backup directory space")
	_, statErr := os.Stat(filepath.Join(dir, "website", "huge.tar.xz"))
	check.True(t, os.IsNotExist(statErr))

	must.NoError(t, local.Put(t.Context(), "website/ok.tar.xz", sizedReader{Reader: strings.NewReader("data"), size: 4}))
	content, err := os.ReadFile(filepath.Join(dir, "website", "ok.tar.xz"))
	must.NoError(t, err)
	check.Equal(t, string(content), "data")

	// 大小未知的照常写
	must.NoError(t, local.Put(t.Context(), "website/stream.tar.xz", strings.NewReader("data")))
}
