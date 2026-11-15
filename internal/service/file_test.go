package service

import (
	stdos "os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestFormatEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	must.NoError(t, stdos.WriteFile(path, []byte("12345"), 0640))
	info, err := stdos.Lstat(path)
	must.NoError(t, err)

	owners := newOwnerNames()
	entry := new(FileService).formatEntry(path, info, owners)
	check.Equal(t, entry["name"], ".env")
	check.Equal(t, entry["full"], any(path))
	check.Equal(t, entry["size"], "5.00 B")
	check.Equal(t, entry["mode"], "0640")
	check.Equal(t, entry["hidden"], true)
	check.Equal(t, entry["dir"], false)
	check.Equal(t, entry["symlink"], false)
	check.Len(t, owners.users, 1)
	check.Len(t, owners.groups, 1)
}

func TestFormatEntryDoesNotOpenFIFO(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	must.NoError(t, syscall.Mkfifo(fifo, 0644))
	link := filepath.Join(dir, "link")
	must.NoError(t, stdos.Symlink(fifo, link))

	// 没有写端的 FIFO 一旦被 open 就永久阻塞；List 用 Lstat，Info 用 Stat
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, path := range []string{fifo, link} {
			for _, stat := range []func(string) (stdos.FileInfo, error){stdos.Lstat, stdos.Stat} {
				info, err := stat(path)
				if err != nil {
					t.Error(err)
					return
				}
				check.Equal(t, new(FileService).formatEntry(path, info, newOwnerNames())["immutable"], false)
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("formatEntry blocked on a FIFO")
	}
}
