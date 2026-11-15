package benchmark

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestMemory(t *testing.T) {
	result, err := memory(t.Context(), 1<<20, 5*time.Millisecond)
	must.NoError(t, err)

	check.Greater(t, result.Score, 0)
	check.Greater(t, result.Bandwidth, 0.0)
	check.False(t, math.IsInf(result.Bandwidth, 0))
	check.Greater(t, result.Latency, 0.0)
	// 一次访问不可能慢到 1 毫秒
	check.Less(t, result.Latency, 1e6)
}

func TestMemoryCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := memory(ctx, 1<<20, 5*time.Millisecond)
	check.ErrorIs(t, err, context.Canceled)
}

// 拷贝确实发生了：目标区的内容和源一致
func TestMemoryBandwidth(t *testing.T) {
	src, dst := make([]byte, 1<<16), make([]byte, 1<<16)
	noise(src, 3)
	check.Greater(t, memoryBandwidth(t.Context(), dst, src, time.Millisecond), 0.0)
	check.DeepEqual(t, dst, src)
}
