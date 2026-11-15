package benchmark

import (
	"context"
	"errors"
	"math"
	"runtime"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

const (
	// memoryBlock 一次拷贝的字节数，源和目标合计要大过末级缓存才测得到内存
	memoryBlock = 48 << 20
	// memoryReserve 测试缓冲区之外要给系统留出的可用内存
	memoryReserve = 64 << 20
	// memoryBatch 延迟测试每一轮的访问次数
	memoryBatch = 1 << 18

	// memoryWindow 带宽和延迟各自的测量时长
	memoryWindow = time.Second

	// 1000 分对应的拷贝带宽（字节/秒）与随机访问延迟（纳秒），按主流云服务器估算
	memoryBandwidthBaseline = 10 << 30
	memoryLatencyBaseline   = 100
)

// ErrNoMemory 可用内存放不下测试缓冲区，硬跑只会测到交换分区或者触发 OOM
var ErrNoMemory = errors.New("not enough available memory")

// MemoryResult 内存测试结果
type MemoryResult struct {
	Score int `json:"score"`
	// Bandwidth 大块拷贝的速度，字节/秒
	Bandwidth float64 `json:"bandwidth"`
	// Latency 随机访问一次的耗时，纳秒
	Latency float64 `json:"latency"`
}

// Memory 内存性能
func Memory(ctx context.Context) (MemoryResult, error) {
	stat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return MemoryResult{}, err
	}
	if stat.Available < 2*memoryBlock+memoryReserve {
		return MemoryResult{}, ErrNoMemory
	}
	return memory(ctx, memoryBlock, memoryWindow)
}

func memory(ctx context.Context, block int, window time.Duration) (MemoryResult, error) {
	// 直接向系统要内存，用完立刻归还，不占着堆等垃圾回收
	buf, err := syscall.Mmap(-1, 0, 2*block, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return MemoryResult{}, err
	}
	defer func() { _ = syscall.Munmap(buf) }()
	smallPages(buf)

	src, dst := buf[:block], buf[block:]
	// 没写过的页都映射到同一个零页，读它测不到内存；随机内容也让延迟测试读到的值无法预测
	noise(src, 8)
	copy(dst, src)

	result := MemoryResult{
		Bandwidth: memoryBandwidth(ctx, dst, src, window),
		Latency:   memoryLatency(ctx, buf, window),
	}
	result.Score = score(geomean(result.Bandwidth/memoryBandwidthBaseline, memoryLatencyBaseline/result.Latency))
	return result, ctx.Err()
}

// memoryBandwidth 反复把 src 拷到 dst，取最快的一轮
func memoryBandwidth(ctx context.Context, dst, src []byte, window time.Duration) float64 {
	best := time.Duration(math.MaxInt64)
	for start := time.Now(); ctx.Err() == nil; {
		begin := time.Now()
		copy(dst, src)
		now := time.Now()
		best = min(best, now.Sub(begin))
		if now.Sub(start) >= window {
			break
		}
	}
	return float64(len(src)) / best.Seconds()
}

// memoryLatency 在 data 里随机访问，取最快的一轮里平均每次的耗时
func memoryLatency(ctx context.Context, data []byte, window time.Duration) float64 {
	size := uint64(len(data))
	best := time.Duration(math.MaxInt64)
	var x uint64
	for start := time.Now(); ctx.Err() == nil; {
		begin := time.Now()
		for range memoryBatch {
			// 下一次的地址依赖上一次读到的值，CPU 无法并行发起多次读取，测到的才是单次访问的延迟
			x = (x+uint64(data[(x>>32)*size>>32]))*6364136223846793005 + 1442695040888963407
		}
		now := time.Now()
		best = min(best, now.Sub(begin))
		if now.Sub(start) >= window {
			break
		}
	}
	// 读到的值没人用的话，编译器会把整段内存读取当死代码消掉
	runtime.KeepAlive(x)
	return float64(best.Nanoseconds()) / memoryBatch
}
