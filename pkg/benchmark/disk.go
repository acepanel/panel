package benchmark

import "errors"

var (
	// ErrMemoryFS 目录在内存文件系统上，测出来的不是硬盘
	ErrMemoryFS = errors.New("directory is on a memory file system")
	// ErrDirectIO 文件系统不支持直写，读写只会落在页缓存里
	ErrDirectIO = errors.New("file system does not support direct io")
	// ErrNoSpace 剩余空间放不下测试文件
	ErrNoSpace = errors.New("not enough free disk space")
)

// DiskResult 硬盘测试结果
type DiskResult struct {
	Score int `json:"score"`
	// SeqRead、SeqWrite 1MB 块顺序读写的速度，字节/秒
	SeqRead  float64 `json:"seq_read"`
	SeqWrite float64 `json:"seq_write"`
	// RandRead、RandWrite 4K 块随机读写的 IOPS
	RandRead  float64 `json:"rand_read"`
	RandWrite float64 `json:"rand_write"`
}
