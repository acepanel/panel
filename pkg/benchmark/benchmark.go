// Package benchmark 面板跑分的各项测试
package benchmark

import (
	"context"
	"math"
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// op 执行一次工作并返回结果摘要
type op func() uint64

// timing 一次测量的时间安排：预热后连续计时 rounds 轮，每轮至少 window
type timing struct {
	warmup time.Duration
	window time.Duration
	rounds int
}

// measure 让每个 op 在各自的 goroutine 里同时反复执行，返回各轮中最高的每秒总执行次数
func measure(ctx context.Context, t timing, ops []op) float64 {
	start := time.Now().Add(t.warmup)
	rates := make([][]float64, len(ops))
	var measuring atomic.Int64
	measuring.Store(int64(len(ops)))

	var wg sync.WaitGroup
	for i, run := range ops {
		rates[i] = make([]float64, t.rounds)
		wg.Go(func() {
			var sink uint64
			for time.Now().Before(start) && ctx.Err() == nil {
				sink += run()
			}
			// 每一轮从上一轮结束的时刻算起跑满 window：按固定时刻切轮的话，被 CPU 配额限流的线程会切出只含全速片段的短轮
			for round := range t.rounds {
				begin, count := time.Now(), 0
				for ctx.Err() == nil {
					sink += run()
					count++
					if elapsed := time.Since(begin); elapsed >= t.window {
						rates[i][round] = float64(count) / elapsed.Seconds()
						break
					}
				}
			}
			// 自己测完后陪跑到所有线程都测完，慢的线程收尾时负载不变
			measuring.Add(-1)
			for measuring.Load() > 0 && ctx.Err() == nil {
				sink += run()
			}
			// 摘要没人用的话，编译器可以把 op 里的计算当死代码消掉
			runtime.KeepAlive(sink)
		})
	}
	wg.Wait()

	best := 0.0
	for round := range t.rounds {
		total := 0.0
		for i := range ops {
			total += rates[i][round]
		}
		best = max(best, total)
	}
	return best
}

// score 把相对基准机的倍数换算成分数
func score(ratio float64) int {
	// baseScore 基准机上每一项的得分
	const baseScore = 1000

	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 0
	}
	return int(math.Round(ratio * baseScore))
}

// geomean 等权几何平均，任何一项都压不倒其它项
func geomean(ratios ...float64) float64 {
	sum := 0.0
	for _, ratio := range ratios {
		sum += math.Log(ratio)
	}
	return math.Exp(sum / float64(len(ratios)))
}

// noise 用固定种子的伪随机字节填满 buf，各项测试的输入每次都相同
func noise(buf []byte, seed byte) {
	_, _ = rand.NewChaCha8([32]byte{seed}).Read(buf)
}
