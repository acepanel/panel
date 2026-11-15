package benchmark

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libtnb/assert/check"
)

// fast 测试用的极短时间安排
var fast = timing{warmup: time.Millisecond, window: 5 * time.Millisecond, rounds: 2}

func TestScore(t *testing.T) {
	tests := []struct {
		name  string
		ratio float64
		want  int
	}{
		{"与基准持平", 1, 1000},
		{"一半", 0.5, 500},
		{"两倍半", 2.5, 2500},
		{"四舍五入", 0.12345, 123},
		{"没测出来", 0, 0},
		{"除以零得到的无穷大", math.Inf(1), 0},
		{"零除以零", math.NaN(), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check.Equal(t, score(tt.ratio), tt.want)
		})
	}
}

func TestGeomean(t *testing.T) {
	near := func(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

	check.True(t, near(geomean(3), 3))
	check.True(t, near(geomean(2, 8), 4))
	check.True(t, near(geomean(1, 1, 1, 1), 1))
	// 一项高出一百倍，七项的合成结果也只到两倍左右
	check.True(t, near(geomean(1, 1, 1, 1, 1, 1, 100), math.Pow(100, 1.0/7)))
	check.Less(t, geomean(1, 1, 1, 1, 1, 1, 100), 2.0)
	// 有一项没测出来，整体就不该有分
	check.Equal(t, score(geomean(1, 0, 1)), 0)
}

func TestMeasure(t *testing.T) {
	// 每次 op 固定睡 2ms，速率应接近每线程 500 次/秒
	sleeper := func(calls *atomic.Int64) op {
		return func() uint64 {
			calls.Add(1)
			time.Sleep(2 * time.Millisecond)
			return 1
		}
	}
	plan := timing{warmup: 5 * time.Millisecond, window: 40 * time.Millisecond, rounds: 2}

	var calls atomic.Int64
	single := measure(t.Context(), plan, []op{sleeper(&calls)})
	check.Greater(t, single, 50.0)
	check.LessOrEqual(t, single, 500.0)
	check.Greater(t, calls.Load(), int64(0))

	// 四个线程同时跑，总速率是各线程之和
	quad := measure(t.Context(), plan, []op{sleeper(&calls), sleeper(&calls), sleeper(&calls), sleeper(&calls)})
	check.Greater(t, quad, single*2)
	check.LessOrEqual(t, quad, 2000.0)
}

// op 比时间窗口还长时，每一轮也要完整跑完一次再算速率
func TestMeasureSlowOp(t *testing.T) {
	var calls atomic.Int64
	rate := measure(t.Context(), timing{window: time.Millisecond, rounds: 3}, []op{func() uint64 {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		return 1
	}})
	check.Equal(t, calls.Load(), int64(3))
	check.Greater(t, rate, 10.0)
	check.LessOrEqual(t, rate, 100.0)
}

func TestMeasureCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var calls atomic.Int64
	rate := measure(ctx, fast, []op{func() uint64 {
		calls.Add(1)
		return 1
	}})
	check.Equal(t, rate, 0.0)
	check.Equal(t, calls.Load(), int64(0))
}
