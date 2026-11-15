package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

// 每一项在极短的时间窗口下都能跑通并给出非零的分数
func TestCPUTests(t *testing.T) {
	check.DeepEqual(t, slices.Sorted(maps.Keys(cpuTests)), []string{"compression", "encryption", "image", "json", "matrix", "physics", "sort"})

	for name, test := range cpuTests {
		t.Run(name, func(t *testing.T) {
			result, err := test.run(t.Context(), fast, 3)
			must.NoError(t, err)
			check.Greater(t, result.Single, 0)
			check.Greater(t, result.Multi, 0)
			check.Equal(t, result.Threads, 3)

			// 只有一个核时不再重复测量
			result, err = test.run(t.Context(), fast, 1)
			must.NoError(t, err)
			check.Greater(t, result.Single, 0)
			check.Equal(t, result.Multi, result.Single)
		})
	}
}

// 每个 op 的结果摘要非零，且同样的输入每次都得到同样的结果
func TestCPUOpsDeterministic(t *testing.T) {
	for name, test := range cpuTests {
		t.Run(name, func(t *testing.T) {
			first, err := test.newOp()
			must.NoError(t, err)
			second, err := test.newOp()
			must.NoError(t, err)

			digest := first()
			check.NotEqual(t, digest, 0)
			check.Equal(t, second(), digest)
		})
	}
}

func TestCPUUnknown(t *testing.T) {
	_, err := CPU(t.Context(), "compile")
	check.ErrorIs(t, err, ErrUnknownTest)
}

func TestCPUCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := cpuTests["sort"].run(ctx, fast, 2)
	check.ErrorIs(t, err, context.Canceled)
}

func TestNoise(t *testing.T) {
	a, b, c := make([]byte, 64), make([]byte, 64), make([]byte, 64)
	noise(a, 1)
	noise(b, 1)
	noise(c, 2)
	check.DeepEqual(t, a, b)
	check.False(t, bytes.Equal(a, c))
	check.False(t, bytes.Equal(a, make([]byte, 64)))
}

func TestBlur(t *testing.T) {
	const width, height = 5, 4
	src := make([]uint8, width*height*4)
	noise(src, 1)
	dst := make([]uint8, len(src))
	blur(dst, src, width, height)

	for y := range height {
		for x := range width {
			for channel := range 4 {
				want := 0
				if x > 0 && x < width-1 && y > 0 && y < height-1 {
					for dy := -1; dy <= 1; dy++ {
						for dx := -1; dx <= 1; dx++ {
							want += int(src[((y+dy)*width+x+dx)*4+channel])
						}
					}
					want /= 9
				}
				check.Equal(t, int(dst[(y*width+x)*4+channel]), want, check.Msgf("x=%d y=%d channel=%d", x, y, channel))
			}
		}
	}
}

func TestMultiply(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	b := []float64{9, 8, 7, 6, 5, 4, 3, 2, 1}
	// 结果区里的旧值不能带进新结果
	c := []float64{1, 1, 1, 1, 1, 1, 1, 1, 1}
	multiply(c, a, b, 3)
	check.DeepEqual(t, c, []float64{30, 24, 18, 84, 69, 54, 138, 114, 90})
}

func TestSorting(t *testing.T) {
	s := newSorting()
	digest := s.run()

	check.True(t, slices.IsSorted(s.work))
	check.DeepEqual(t, s.work, slices.Sorted(slices.Values(s.data)))
	check.Equal(t, digest, s.work[sortingSize/2])
	// 输入必须是乱序的，否则排序什么都不用做
	check.False(t, slices.IsSorted(s.data))
}

func TestEncryption(t *testing.T) {
	e, err := newEncryption()
	must.NoError(t, err)
	first := e.run()
	check.NotEqual(t, e.run(), first)

	// 密文区里留着最后一块，用最后一个 nonce 能解回明文
	sealed := e.ciphertext[:encryptionChunk+e.aead.Overhead()]
	plaintext, err := e.aead.Open(nil, e.nonce, sealed, nil)
	must.NoError(t, err)
	check.True(t, bytes.Equal(plaintext, e.plaintext))
	check.False(t, bytes.Equal(sealed[:encryptionChunk], e.plaintext))
}

func TestCompression(t *testing.T) {
	c, err := newCompression()
	must.NoError(t, err)
	digest := c.run()

	check.True(t, bytes.Equal(c.unpacked.Bytes(), c.text))
	check.Equal(t, int(digest&math.MaxUint32), len(c.text))
	// 输入既不是随机数据也不是重复字符，压缩比应落在真实文本的范围里
	ratio := float64(len(c.text)) / float64(digest>>32)
	check.Greater(t, ratio, 3.0)
	check.Less(t, ratio, 12.0)
	// 压缩器和解压器复用后结果不变
	check.Equal(t, c.run(), digest)
}

func TestAdvance(t *testing.T) {
	p := newPhysics()
	momentum := func(bodies []body) (x, y, z float64) {
		for _, b := range bodies {
			x += b.mass * b.vx
			y += b.mass * b.vy
			z += b.mass * b.vz
		}
		return x, y, z
	}

	copy(p.bodies, p.initial)
	px, py, pz := momentum(p.bodies)
	for range 10 {
		advance(p.bodies, physicsDelta)
	}
	// 质点间的作用力成对抵消，总动量守恒
	qx, qy, qz := momentum(p.bodies)
	check.Less(t, math.Abs(qx-px)+math.Abs(qy-py)+math.Abs(qz-pz), 1e-9)

	moved := false
	for i, b := range p.bodies {
		must.False(t, math.IsNaN(b.x+b.y+b.z+b.vx+b.vy+b.vz) || math.IsInf(b.x+b.y+b.z+b.vx+b.vy+b.vz, 0))
		moved = moved || b.x != p.initial[i].x
	}
	check.True(t, moved)
}

func TestJSONCodec(t *testing.T) {
	j := newJSONCodec()
	digest := j.run()

	// 带缩进的版本再压紧，应该和直接序列化的结果一字不差
	encoded := bytes.TrimSuffix(j.encoded.Bytes(), []byte("\n"))
	check.True(t, bytes.Equal(j.compact.Bytes(), encoded))
	check.Greater(t, j.pretty.Len(), j.compact.Len())
	check.Equal(t, digest, uint64(j.pretty.Len())<<32|uint64(j.compact.Len())) //nolint:gosec

	var decoded []record
	must.NoError(t, json.Unmarshal(j.pretty.Bytes(), &decoded))
	check.DeepEqual(t, decoded, j.records)
	check.Equal(t, j.run(), digest)
}

func TestRecords(t *testing.T) {
	list := records(50)
	check.Len(t, list, 50)
	check.DeepEqual(t, records(50), list)
	check.NotEqual(t, list[0].Path, list[1].Path)
}
