package benchmark

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"strconv"
	"time"
)

// 基准机单线程每秒执行各项 op 的次数，取多次实测里最好的成绩
const (
	imageBaseline       = 1000
	matrixBaseline      = 1550
	sortBaseline        = 770
	encryptionBaseline  = 1830
	compressionBaseline = 650
	physicsBaseline     = 1180
	jsonBaseline        = 1450
)

// ErrUnknownTest 没有这一项测试
var ErrUnknownTest = errors.New("unknown test")

var cpuTests = map[string]cpuTest{
	"image":       {baseline: imageBaseline, newOp: func() (op, error) { return newImage().run, nil }},
	"matrix":      {baseline: matrixBaseline, newOp: func() (op, error) { return newMatrix().run, nil }},
	"sort":        {baseline: sortBaseline, newOp: func() (op, error) { return newSorting().run, nil }},
	"encryption":  {baseline: encryptionBaseline, newOp: newEncryptionOp},
	"compression": {baseline: compressionBaseline, newOp: newCompressionOp},
	"physics":     {baseline: physicsBaseline, newOp: func() (op, error) { return newPhysics().run, nil }},
	"json":        {baseline: jsonBaseline, newOp: func() (op, error) { return newJSONCodec().run, nil }},
}

// cpuTiming 单线程和多线程各测一次，每次 0.2 秒预热加 5 轮 0.3 秒
var cpuTiming = timing{warmup: 200 * time.Millisecond, window: 300 * time.Millisecond, rounds: 5}

// CPUResult 一项 CPU 测试的得分，多线程按可用核数并行
type CPUResult struct {
	Single  int `json:"single"`
	Multi   int `json:"multi"`
	Threads int `json:"threads"`
}

// cpuTest 一项 CPU 测试，newOp 返回的 op 独占自己的工作区
type cpuTest struct {
	baseline float64
	newOp    func() (op, error)
}

// CPU 运行一项 CPU 测试
func CPU(ctx context.Context, name string) (CPUResult, error) {
	test, ok := cpuTests[name]
	if !ok {
		return CPUResult{}, ErrUnknownTest
	}
	return test.run(ctx, cpuTiming, runtime.GOMAXPROCS(0))
}

func (c cpuTest) run(ctx context.Context, t timing, threads int) (CPUResult, error) {
	ops := make([]op, threads)
	for i := range ops {
		var err error
		if ops[i], err = c.newOp(); err != nil {
			return CPUResult{}, err
		}
	}

	result := CPUResult{Threads: threads}
	result.Single = score(measure(ctx, t, ops[:1]) / c.baseline)
	result.Multi = result.Single
	// 只有一个核时多线程就是单线程，再测一遍只会得到一个带噪声的近似值
	if threads > 1 {
		result.Multi = score(measure(ctx, t, ops) / c.baseline)
	}
	return result, ctx.Err()
}

// imageSize 图像边长，原图和结果各 1MB
const imageSize = 512

// image 对 RGBA 图像做 3×3 均值模糊
type image struct {
	src, dst []uint8
}

func newImage() *image {
	const size = imageSize * imageSize * 4
	i := &image{
		src: make([]uint8, size),
		// 两块缓冲区都按页对齐时，每次写入都和紧接着的读取低 12 位地址相同，部分 CPU 会误判成依赖而变慢
		dst: make([]uint8, size+1024)[1024:],
	}
	noise(i.src, 1)
	return i
}

func (i *image) run() uint64 {
	blur(i.dst, i.src, imageSize, imageSize)
	// 取正中间那个像素，边缘一圈没有结果
	return uint64(binary.LittleEndian.Uint32(i.dst[(imageSize+1)*imageSize*2:]))
}

// blur 边缘一圈像素不处理
func blur(dst, src []uint8, width, height int) {
	stride := width * 4
	for y := 1; y < height-1; y++ {
		up := src[(y-1)*stride:][:stride]
		mid := src[y*stride:][:stride]
		down := src[(y+1)*stride:][:stride]
		out := dst[y*stride:][:stride]
		for x := 4; x < stride-4; x++ {
			sum := uint32(up[x-4]) + uint32(up[x]) + uint32(up[x+4]) +
				uint32(mid[x-4]) + uint32(mid[x]) + uint32(mid[x+4]) +
				uint32(down[x-4]) + uint32(down[x]) + uint32(down[x+4])
			out[x] = uint8(sum / 9) //nolint:gosec
		}
	}
}

// matrixSize 方阵边长，三个矩阵共 384KB
const matrixSize = 128

// matrix 双精度稠密矩阵乘法
type matrix struct {
	a, b, c []float64
}

func newMatrix() *matrix {
	m := &matrix{
		a: make([]float64, matrixSize*matrixSize),
		b: make([]float64, matrixSize*matrixSize),
		c: make([]float64, matrixSize*matrixSize),
	}
	r := rand.New(rand.NewPCG(2, 2)) //nolint:gosec
	for i := range m.a {
		m.a[i], m.b[i] = r.Float64(), r.Float64()
	}
	return m
}

func (m *matrix) run() uint64 {
	multiply(m.c, m.a, m.b, matrixSize)
	return math.Float64bits(m.c[len(m.c)/2])
}

// multiply 计算 n 阶方阵 c = a × b
func multiply(c, a, b []float64, n int) {
	for i := range n {
		row := c[i*n:][:n]
		clear(row)
		for k := range n {
			aik := a[i*n+k]
			col := b[k*n:][:n]
			for j := range n {
				row[j] += aik * col[j]
			}
		}
	}
}

// sortingSize 每次排序的元素个数，原数据和工作区各 256KB
const sortingSize = 1 << 15

// sorting 对随机的 64 位整数排序
type sorting struct {
	data, work []uint64
}

func newSorting() *sorting {
	s := &sorting{
		data: make([]uint64, sortingSize),
		work: make([]uint64, sortingSize),
	}
	r := rand.New(rand.NewPCG(3, 3)) //nolint:gosec
	for i := range s.data {
		s.data[i] = r.Uint64()
	}
	return s
}

func (s *sorting) run() uint64 {
	copy(s.work, s.data)
	slices.Sort(s.work)
	return s.work[len(s.work)/2]
}

const (
	// encryptionChunk 每次加密的明文长度，明文和密文都留在缓存里
	encryptionChunk = 64 << 10
	// encryptionChunks 一次 op 加密的块数，共 4MB
	encryptionChunks = 64
)

// encryption AES-256-GCM 加密
type encryption struct {
	aead       cipher.AEAD
	nonce      []byte
	counter    uint64
	plaintext  []byte
	ciphertext []byte
}

func newEncryption() (*encryption, error) {
	key := make([]byte, 32)
	noise(key, 4)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	e := &encryption{
		aead:       aead,
		nonce:      make([]byte, aead.NonceSize()),
		plaintext:  make([]byte, encryptionChunk),
		ciphertext: make([]byte, 0, encryptionChunk+aead.Overhead()),
	}
	noise(e.plaintext, 5)
	return e, nil
}

func newEncryptionOp() (op, error) {
	e, err := newEncryption()
	if err != nil {
		return nil, err
	}
	return e.run, nil
}

func (e *encryption) run() uint64 {
	var sum uint64
	for range encryptionChunks {
		e.counter++
		binary.LittleEndian.PutUint64(e.nonce, e.counter)
		sealed := e.aead.Seal(e.ciphertext, e.nonce, e.plaintext, nil)
		sum += binary.LittleEndian.Uint64(sealed[len(sealed)-8:])
	}
	return sum
}

// compressionRecords 压缩测试的输入条数，编码成 JSON 约 300KB
const compressionRecords = 800

// compression 用 gzip 默认级别压缩一段 JSON 文本再解压
type compression struct {
	text     []byte
	packed   bytes.Buffer
	unpacked bytes.Buffer
	writer   *gzip.Writer
	reader   gzip.Reader
}

func newCompression() (*compression, error) {
	text, err := json.Marshal(records(compressionRecords))
	if err != nil {
		return nil, err
	}
	c := &compression{text: text}
	c.writer = gzip.NewWriter(&c.packed)
	c.unpacked.Grow(len(text) + bytes.MinRead)
	return c, nil
}

func newCompressionOp() (op, error) {
	c, err := newCompression()
	if err != nil {
		return nil, err
	}
	return c.run, nil
}

func (c *compression) run() uint64 {
	c.packed.Reset()
	c.writer.Reset(&c.packed)
	_, _ = c.writer.Write(c.text)
	_ = c.writer.Close()
	size := c.packed.Len()

	c.unpacked.Reset()
	if err := c.reader.Reset(&c.packed); err != nil {
		return 0
	}
	_, _ = c.unpacked.ReadFrom(&c.reader)
	return uint64(size)<<32 | uint64(c.unpacked.Len()) //nolint:gosec
}

const (
	physicsBodies = 512
	// physicsSteps 一次 op 推进的步数
	physicsSteps = 4
	physicsDelta = 0.01
	// physicsSoftening 避免两个质点靠得太近时受力发散
	physicsSoftening = 1e-3
)

type body struct {
	x, y, z    float64
	vx, vy, vz float64
	mass       float64
}

// physics 引力 N 体仿真，每次都从同一个初始状态出发
type physics struct {
	initial, bodies []body
}

func newPhysics() *physics {
	p := &physics{
		initial: make([]body, physicsBodies),
		bodies:  make([]body, physicsBodies),
	}
	r := rand.New(rand.NewPCG(6, 6)) //nolint:gosec
	for i := range p.initial {
		p.initial[i] = body{
			x: r.Float64(), y: r.Float64(), z: r.Float64(),
			vx: r.Float64() - 0.5, vy: r.Float64() - 0.5, vz: r.Float64() - 0.5,
			mass: r.Float64() + 0.5,
		}
	}
	return p
}

func (p *physics) run() uint64 {
	copy(p.bodies, p.initial)
	for range physicsSteps {
		advance(p.bodies, physicsDelta)
	}
	return math.Float64bits(p.bodies[0].x)
}

// advance 把所有质点推进一个时间步
func advance(bodies []body, dt float64) {
	for i := range bodies {
		// 第 i 个质点的速度增量攒在局部变量里，内层循环每一步只写第 j 个质点
		a := bodies[i]
		var ax, ay, az float64
		for j := i + 1; j < len(bodies); j++ {
			b := &bodies[j]
			dx, dy, dz := a.x-b.x, a.y-b.y, a.z-b.z
			dist2 := dx*dx + dy*dy + dz*dz + physicsSoftening
			mag := dt / (dist2 * math.Sqrt(dist2))
			ax -= dx * b.mass * mag
			ay -= dy * b.mass * mag
			az -= dz * b.mass * mag
			b.vx += dx * a.mass * mag
			b.vy += dy * a.mass * mag
			b.vz += dz * a.mass * mag
		}
		bodies[i].vx += ax
		bodies[i].vy += ay
		bodies[i].vz += az
	}
	for i := range bodies {
		b := &bodies[i]
		b.x += dt * b.vx
		b.y += dt * b.vy
		b.z += dt * b.vz
	}
}

// jsonRecords JSON 测试的输入条数，编码后约 200KB
const jsonRecords = 512

// jsonCodec 把一组访问记录序列化成 JSON，再解析并重排成带缩进和紧凑两种格式
type jsonCodec struct {
	records         []record
	encoder         *json.Encoder
	encoded         bytes.Buffer
	pretty, compact bytes.Buffer
}

func newJSONCodec() *jsonCodec {
	j := &jsonCodec{records: records(jsonRecords)}
	j.encoder = json.NewEncoder(&j.encoded)
	return j
}

// run 不解析成 Go 的值：那样每次要分配几百 KB，成绩取决于垃圾回收而不是 CPU
func (j *jsonCodec) run() uint64 {
	j.encoded.Reset()
	j.pretty.Reset()
	j.compact.Reset()
	if err := j.encoder.Encode(j.records); err != nil {
		return 0
	}
	if err := json.Indent(&j.pretty, j.encoded.Bytes(), "", "  "); err != nil {
		return 0
	}
	if err := json.Compact(&j.compact, j.pretty.Bytes()); err != nil {
		return 0
	}
	return uint64(j.pretty.Len())<<32 | uint64(j.compact.Len()) //nolint:gosec
}

// record 一条访问记录
type record struct {
	ID        uint64   `json:"id"`
	Time      string   `json:"time"`
	Remote    string   `json:"remote"`
	Method    string   `json:"method"`
	Host      string   `json:"host"`
	Path      string   `json:"path"`
	Status    int      `json:"status"`
	Bytes     int64    `json:"bytes"`
	Duration  float64  `json:"duration"`
	Cached    bool     `json:"cached"`
	UserAgent string   `json:"user_agent"`
	Encoding  string   `json:"encoding"`
	Language  string   `json:"language"`
	Tags      []string `json:"tags"`
}

// records 用固定种子生成 n 条访问记录，内容接近真实日志：字段名与常见取值重复，数值各不相同
func records(n int) []record {
	var (
		methods  = []string{"GET", "GET", "GET", "GET", "POST", "HEAD", "PUT", "DELETE"}
		hosts    = []string{"example.com", "www.example.com", "api.example.com", "static.example.net", "blog.example.org"}
		segments = []string{"api", "v1", "v2", "users", "orders", "products", "assets", "images", "static", "search", "login", "admin", "posts", "comments", "uploads", "feed"}
		suffixes = []string{"", "", ".html", ".json", ".css", ".js", ".png", ".webp"}
		statuses = []int{200, 200, 200, 200, 200, 204, 301, 302, 304, 400, 403, 404, 500, 502}
		agents   = []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Safari/605.1.15",
			"Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148",
			"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			"curl/8.14.1",
		}
		tags      = []string{"web", "api", "static", "bot", "mobile", "cdn", "internal", "slow"}
		encodings = []string{"gzip", "br", "zstd", "identity"}
		languages = []string{"zh-CN,zh;q=0.9", "en-US,en;q=0.8", "zh-TW,zh;q=0.9,en;q=0.7", "ja,en;q=0.5"}
	)

	r := rand.New(rand.NewPCG(7, 7)) //nolint:gosec
	list := make([]record, n)
	buf := make([]byte, 0, 64)
	for i := range list {
		buf = buf[:0]
		for range 1 + r.IntN(3) {
			buf = append(buf, '/')
			buf = append(buf, segments[r.IntN(len(segments))]...)
		}
		buf = append(buf, '/')
		buf = strconv.AppendUint(buf, r.Uint64N(1e6), 10)
		buf = append(buf, suffixes[r.IntN(len(suffixes))]...)
		path := string(buf)

		buf = strconv.AppendUint(buf[:0], 1+r.Uint64N(223), 10)
		for range 3 {
			buf = append(buf, '.')
			buf = strconv.AppendUint(buf, r.Uint64N(256), 10)
		}
		remote := string(buf)

		at := time.Unix(1763164800+int64(i)*3+r.Int64N(3), 0).UTC()
		list[i] = record{
			ID:        r.Uint64(),
			Time:      at.Format(time.RFC3339),
			Remote:    remote,
			Method:    methods[r.IntN(len(methods))],
			Host:      hosts[r.IntN(len(hosts))],
			Path:      path,
			Status:    statuses[r.IntN(len(statuses))],
			Bytes:     r.Int64N(1 << 20),
			Duration:  math.Round(r.ExpFloat64()*50000) / 1000,
			Cached:    r.IntN(4) == 0,
			UserAgent: agents[r.IntN(len(agents))],
			Encoding:  encodings[r.IntN(len(encodings))],
			Language:  languages[r.IntN(len(languages))],
			Tags:      []string{tags[r.IntN(len(tags))], tags[r.IntN(len(tags))]},
		}
	}
	return list
}
