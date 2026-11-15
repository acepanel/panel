package benchmark

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

const (
	testFileSize = 4 << 20
	testWindow   = 20 * time.Millisecond
)

// diskDir 返回一个可以做直写测试的临时目录，临时目录在内存文件系统上时跳过
func diskDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var stat syscall.Statfs_t
	must.NoError(t, syscall.Statfs(dir, &stat))
	if stat.Type == tmpfsMagic || stat.Type == ramfsMagic {
		t.Skip("临时目录在内存文件系统上")
	}
	return dir
}

func entries(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	list, err := os.ReadDir(dir)
	must.NoError(t, err)
	return list
}

func TestDisk(t *testing.T) {
	dir := diskDir(t)
	result, err := disk(t.Context(), dir, testFileSize, testWindow)
	must.NoError(t, err)

	check.Greater(t, result.Score, 0)
	check.Greater(t, result.SeqRead, 0.0)
	check.Greater(t, result.SeqWrite, 0.0)
	check.Greater(t, result.RandRead, 0.0)
	check.Greater(t, result.RandWrite, 0.0)
	check.Empty(t, entries(t, dir))
}

func TestDiskMemoryFS(t *testing.T) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/dev/shm", &stat); err != nil || stat.Type != tmpfsMagic {
		t.Skip("没有 tmpfs 可用")
	}
	t.Setenv("TMPDIR", "/dev/shm")
	dir := t.TempDir()

	result, err := disk(t.Context(), dir, testFileSize, testWindow)
	check.ErrorIs(t, err, ErrMemoryFS)
	check.DeepEqual(t, result, DiskResult{})
	check.Empty(t, entries(t, dir))
}

func TestDiskNoSpace(t *testing.T) {
	dir := diskDir(t)
	result, err := disk(t.Context(), dir, 1<<50, testWindow)
	check.ErrorIs(t, err, ErrNoSpace)
	check.DeepEqual(t, result, DiskResult{})
	check.Empty(t, entries(t, dir))
}

func TestDiskMissingDir(t *testing.T) {
	_, err := disk(t.Context(), "/nonexistent/acepanel", testFileSize, testWindow)
	check.ErrorIs(t, err, fs.ErrNotExist)
}

// 中途取消要立刻返回错误，且不留下测试文件
func TestDiskCanceled(t *testing.T) {
	dir := diskDir(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := disk(ctx, dir, testFileSize, testWindow)
	check.ErrorIs(t, err, context.Canceled)
	check.Empty(t, entries(t, dir))

	ctx, cancel = context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = disk(ctx, dir, testFileSize, 10*time.Second)
	check.ErrorIs(t, err, context.DeadlineExceeded)
	check.Less(t, time.Since(start), 5*time.Second)
	check.Empty(t, entries(t, dir))
}

// 读写出错时返回的是那个错误，而不是一个假成绩
func TestRandomError(t *testing.T) {
	failure := errors.New("io failure")
	iops, err := random(t.Context(), testFileSize, 10*time.Second, func([]byte, int64) (int, error) {
		return 0, failure
	})
	check.ErrorIs(t, err, failure)
	check.Equal(t, iops, 0.0)
}

func TestCreateDirect(t *testing.T) {
	dir := diskDir(t)
	file, err := createDirect(dir)
	must.NoError(t, err)
	defer func() { _ = file.Close() }()

	// 文件已经从目录里删掉，但描述符照常可写可读
	check.Empty(t, entries(t, dir))
	block := alignedBlock(diskRandBlock, 1)
	_, err = file.WriteAt(block, 0)
	must.NoError(t, err)
	got := alignedBlock(diskRandBlock, 2)
	_, err = file.ReadAt(got, 0)
	must.NoError(t, err)
	check.DeepEqual(t, got, block)
}

// ramfs 不支持直写：要报 ErrDirectIO，并清掉内核在打开失败前已经建好的空文件
func TestCreateDirectUnsupported(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mount("ramfs", dir, "ramfs", 0, ""); err != nil {
		t.Skipf("挂载 ramfs 需要特权：%v", err)
	}
	t.Cleanup(func() { _ = syscall.Unmount(dir, 0) })

	_, err := createDirect(dir)
	check.ErrorIs(t, err, ErrDirectIO)
	check.Empty(t, entries(t, dir))

	_, err = disk(t.Context(), dir, testFileSize, testWindow)
	check.ErrorIs(t, err, ErrMemoryFS)
}

func TestAlignedBlock(t *testing.T) {
	for _, size := range []int{diskRandBlock, diskSeqBlock} {
		block := alignedBlock(size, 1)
		check.Len(t, block, size)
		check.Equal(t, uintptr(unsafe.Pointer(unsafe.SliceData(block)))%diskAlign, 0) //nolint:gosec
		// 写出去的不能是全零
		check.NotEqual(t, block[0]|block[1]|block[2]|block[3], 0)
	}
}
