package benchmark

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const (
	// diskFileSize 测试文件的大小上限，顺序写到头后从头覆盖
	diskFileSize  = 512 << 20
	diskSeqBlock  = 1 << 20
	diskRandBlock = 4 << 10
	// diskRandWorkers 随机读写的并发数
	diskRandWorkers = 16
	// diskAlign 直写要求缓冲区地址、偏移和长度都按逻辑扇区对齐，4096 覆盖 512 与 4K 两种扇区
	diskAlign = 4096
	// diskWindow 顺序写、顺序读、随机写、随机读各自的测量时长
	diskWindow = 2 * time.Second

	tmpfsMagic = 0x01021994
	ramfsMagic = 0x858458f6

	// 基准机的顺序读写速度（字节/秒）与 4K 随机读写 IOPS，测的是 Docker Desktop 虚拟机的虚拟磁盘
	diskSeqReadBaseline   = 7500 << 20
	diskSeqWriteBaseline  = 7000 << 20
	diskRandReadBaseline  = 95000
	diskRandWriteBaseline = 25000
)

// Disk 测试 dir 所在硬盘的读写性能
func Disk(ctx context.Context, dir string) (DiskResult, error) {
	return disk(ctx, dir, diskFileSize, diskWindow)
}

func disk(ctx context.Context, dir string, fileSize int64, window time.Duration) (DiskResult, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return DiskResult{}, err
	}
	if stat.Type == tmpfsMagic || stat.Type == ramfsMagic {
		return DiskResult{}, ErrMemoryFS
	}
	// 给测试文件之外留出同样大的余量
	if stat.Bavail*uint64(stat.Bsize) < 2*uint64(fileSize) { //nolint:gosec
		return DiskResult{}, ErrNoSpace
	}

	file, err := createDirect(dir)
	if err != nil {
		return DiskResult{}, err
	}
	defer func() { _ = file.Close() }()

	// 顺序写在先，写出来的部分就是后面三项的读写范围
	var result DiskResult
	var size int64
	if size, result.SeqWrite, err = sequentialWrite(ctx, file, fileSize, window); err != nil {
		return DiskResult{}, err
	}
	if result.SeqRead, err = sequentialRead(ctx, file, size, window); err != nil {
		return DiskResult{}, err
	}
	if result.RandWrite, err = random(ctx, size, window, file.WriteAt); err != nil {
		return DiskResult{}, err
	}
	if err = file.Sync(); err != nil {
		return DiskResult{}, err
	}
	if result.RandRead, err = random(ctx, size, window, file.ReadAt); err != nil {
		return DiskResult{}, err
	}

	result.Score = score(geomean(
		result.SeqRead/diskSeqReadBaseline,
		result.SeqWrite/diskSeqWriteBaseline,
		result.RandRead/diskRandReadBaseline,
		result.RandWrite/diskRandWriteBaseline,
	))
	return result, nil
}

// createDirect 在 dir 下创建直写的测试文件并立即删除，之后只凭描述符读写，进程中途退出也不会残留
func createDirect(dir string) (*os.File, error) {
	path := filepath.Join(dir, ".benchmark-"+strconv.FormatUint(rand.Uint64(), 36)) //nolint:gosec
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL|syscall.O_DIRECT, 0o600)
	if err != nil {
		// 文件系统不支持直写时，打开失败也会留下一个空文件
		_ = os.Remove(path)
		if errors.Is(err, syscall.EINVAL) {
			return nil, ErrDirectIO
		}
		return nil, err
	}
	if err = os.Remove(path); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// sequentialWrite 从头按块写到 limit 后回到开头继续，返回写出的文件大小和速度，落盘的时间也算在内
func sequentialWrite(ctx context.Context, file *os.File, limit int64, window time.Duration) (int64, float64, error) {
	block := alignedBlock(diskSeqBlock, 9)
	var written int64
	start := time.Now()
	for written == 0 || time.Since(start) < window {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		if _, err := file.WriteAt(block, written%limit); err != nil {
			return 0, 0, err
		}
		written += diskSeqBlock
	}
	if err := file.Sync(); err != nil {
		return 0, 0, err
	}
	return min(written, limit), float64(written) / time.Since(start).Seconds(), nil
}

// sequentialRead 从头按块读到 size 后回到开头继续
func sequentialRead(ctx context.Context, file *os.File, size int64, window time.Duration) (float64, error) {
	block := alignedBlock(diskSeqBlock, 10)
	var read int64
	start := time.Now()
	for read == 0 || time.Since(start) < window {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if _, err := file.ReadAt(block, read%size); err != nil {
			return 0, err
		}
		read += diskSeqBlock
	}
	return float64(read) / time.Since(start).Seconds(), nil
}

// random 多个 worker 同时在文件的前 size 字节里随机读或写 4K 块，返回 IOPS
func random(
	ctx context.Context,
	size int64,
	window time.Duration,
	transfer func([]byte, int64) (int, error),
) (float64, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	ops := make([]op, diskRandWorkers)
	for i := range ops {
		block := alignedBlock(diskRandBlock, byte(i))
		r := rand.New(rand.NewPCG(uint64(i), 11)) //nolint:gosec
		ops[i] = func() uint64 {
			n, err := transfer(block, r.Int64N(size/diskRandBlock)*diskRandBlock)
			if err != nil {
				cancel(err)
			}
			return uint64(n) //nolint:gosec
		}
	}
	iops := measure(ctx, timing{window: window, rounds: 1}, ops)
	return iops, context.Cause(ctx)
}

// alignedBlock 返回一块地址按 diskAlign 对齐、填满随机内容的缓冲区，全零的数据会被带压缩或精简配置的存储优化掉
func alignedBlock(size int, seed byte) []byte {
	buf := make([]byte, size+diskAlign)
	skip := -int(uintptr(unsafe.Pointer(unsafe.SliceData(buf)))) & (diskAlign - 1) //nolint:gosec
	buf = buf[skip:][:size:size]
	noise(buf, seed)
	return buf
}
