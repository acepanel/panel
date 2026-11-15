package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/samber/lo"
)

func TestClearExpiredKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	// 文件名顺序与新旧相反，确认取舍看的是修改时间
	for i, name := range []string{"site_c.tar.gz", "site_b.tar.gz", "site_a.tar.gz", "other_a.tar.gz", "site_note.txt"} {
		path := filepath.Join(dir, name)
		must.NoError(t, os.WriteFile(path, []byte("x"), 0644))
		modTime := now.Add(-time.Duration(i) * time.Hour)
		must.NoError(t, os.Chtimes(path, modTime, modTime))
	}

	repo := &backupRepo{t: gotext.NewLocale("", "en")}
	must.NoError(t, repo.ClearExpired(dir, "site_", 2))

	entries, err := os.ReadDir(dir)
	must.NoError(t, err)
	names := lo.Map(entries, func(entry os.DirEntry, _ int) string { return entry.Name() })
	check.DeepEqual(t, names, []string{"other_a.tar.gz", "site_b.tar.gz", "site_c.tar.gz", "site_note.txt"})
}
