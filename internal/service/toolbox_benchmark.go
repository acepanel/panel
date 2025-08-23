package service

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"github.com/gofiber/fiber/v2"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"math/big"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/tnborg/panel/internal/http/request"
	"github.com/tnborg/panel/pkg/shell"
)

type ToolboxBenchmarkService struct {
	t *gotext.Locale
}

func NewToolboxBenchmarkService(t *gotext.Locale) *ToolboxBenchmarkService {
	return &ToolboxBenchmarkService{
		t: t,
	}
}

// Test 运行测试
func (s *ToolboxBenchmarkService) Test(c fiber.Ctx) error {
	req, err := Bind[request.ToolboxBenchmarkTest](c)
	if err != nil {
		return Error(c, http.StatusUnprocessableEntity, "%v", err)
	}

	switch req.Name {
	case "image":
		result := s.imageProcessing()
		return Success(c, result)
	case "machine":
		result := s.machineLearning()
		return Success(c, result)
	case "compile":
		result := s.compileSimulationSingle()
		return Success(c, result)
	case "encryption":
		result := s.encryptionTest()
		return Success(c, result)
	case "compression":
		result := s.compressionTest()
		return Success(c, result)
	case "physics":
		result := s.physicsSimulation()
		return Success(c, result)
	case "json":
		result := s.jsonProcessing()
		return Success(c, result)
	case "disk":
		result := s.diskTestTask()
		return Success(c, result)
	case "memory":
		result := s.memoryTestTask()
		return Success(c, result)
	default:
		return Error(c, http.StatusUnprocessableEntity, s.t.Get("unknown test type"))
	}
}

// calculateCpuScore 计算CPU成绩
func (s *ToolboxBenchmarkService) calculateCpuScore(duration time.Duration) int {
	score := int((10 / duration.Seconds()) * float64(3000))

	if score < 0 {
		score = 0
	}
	return score
}

// calculateScore 计算内存/硬盘成绩
func (s *ToolboxBenchmarkService) calculateScore(duration time.Duration) int {
	score := int((20 / duration.Seconds()) * float64(30000))

	if score < 0 {
		score = 0
	}
	return score
}

// 图像处理

func (s *ToolboxBenchmarkService) imageProcessing() int {
	start := time.Now()
	if err := s.imageProcessingTask(); err != nil {
		return 0
	}
	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

func (s *ToolboxBenchmarkService) imageProcessingTask() error {
	img := image.NewRGBA(image.Rect(0, 0, 4000, 4000))
	for x := 0; x < 4000; x++ {
		for y := 0; y < 4000; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), A: 255})
		}
	}

	dx := img.Bounds().Dx()
	dy := img.Bounds().Dy()

	for x := 1; x < dx-1; x++ {
		for y := 1; y < dy-1; y++ {
			// 卷积操作（模糊）
			rTotal, gTotal, bTotal := 0, 0, 0
			for k := -1; k <= 1; k++ {
				for l := -1; l <= 1; l++ {
					r, g, b, _ := img.At(x+k, y+l).RGBA()
					rTotal += int(r)
					gTotal += int(g)
					bTotal += int(b)
				}
			}
			rAvg := uint8(rTotal / 9 / 256)
			gAvg := uint8(gTotal / 9 / 256)
			bAvg := uint8(bTotal / 9 / 256)
			img.Set(x, y, color.RGBA{R: rAvg, G: gAvg, B: bAvg, A: 255})
		}
	}

	return nil
}

// 机器学习（矩阵乘法）

func (s *ToolboxBenchmarkService) machineLearning() int {
	start := time.Now()
	s.machineLearningTask()
	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

func (s *ToolboxBenchmarkService) machineLearningTask() {
	size := 900
	a := make([][]float64, size)
	b := make([][]float64, size)
	for i := 0; i < size; i++ {
		a[i] = make([]float64, size)
		b[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			a[i][j] = rand.Float64()
			b[i][j] = rand.Float64()
		}
	}

	c := make([][]float64, size)
	for i := 0; i < size; i++ {
		c[i] = make([]float64, size)
	}

	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			sum := 0.0
			for l := 0; l < size; l++ {
				sum += a[i][l] * b[l][j]
			}
			c[i][j] = sum
		}
	}
}

// 数学问题（计算斐波那契数）

func (s *ToolboxBenchmarkService) compileSimulationSingle() int {
	start := time.Now()
	totalCalculations := 1000
	fibNumber := 20000

	for j := 0; j < totalCalculations; j++ {
		s.fib(fibNumber)
	}

	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

// 斐波那契函数
func (s *ToolboxBenchmarkService) fib(n int) *big.Int {
	if n < 2 {
		return big.NewInt(int64(n))
	}
	a := big.NewInt(0)
	b := big.NewInt(1)
	temp := big.NewInt(0)
	for i := 2; i <= n; i++ {
		temp.Add(a, b)
		a.Set(b)
		b.Set(temp)
	}
	return b
}

// AES加密

func (s *ToolboxBenchmarkService) encryptionTest() int {
	start := time.Now()
	if err := s.encryptionTestTask(); err != nil {
		return 0
	}
	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

func (s *ToolboxBenchmarkService) encryptionTestTask() error {
	key := []byte("abcdefghijklmnopqrstuvwxyz123456")
	dataSize := 1024 * 1024 * 512 // 512 MB
	plaintext := []byte(strings.Repeat("A", dataSize))
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = cryptorand.Read(nonce); err != nil {
		return err
	}

	aesGCM.Seal(nil, nonce, plaintext, nil)
	return nil
}

// 压缩/解压缩

func (s *ToolboxBenchmarkService) compressionTest() int {
	start := time.Now()
	s.compressionTestTask()
	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

func (s *ToolboxBenchmarkService) compressionTestTask() {
	data := []byte(strings.Repeat("AcePanel", 50000000))

	// 压缩
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(data)
	_ = w.Close()

	// 解压缩
	r, err := gzip.NewReader(&buf)
	if err != nil {
	}
	_, err = io.Copy(io.Discard, r)
	if err != nil {
	}
	_ = r.Close()
}

// 物理仿真（N体问题）

func (s *ToolboxBenchmarkService) physicsSimulation() int {
	start := time.Now()
	s.physicsSimulationTask()
	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

func (s *ToolboxBenchmarkService) physicsSimulationTask() {
	const (
		numBodies = 4000
		steps     = 30
	)

	type Body struct {
		x, y, z, vx, vy, vz float64
	}

	bodies := make([]Body, numBodies)
	for i := 0; i < numBodies; i++ {
		bodies[i] = Body{
			x:  rand.Float64(),
			y:  rand.Float64(),
			z:  rand.Float64(),
			vx: rand.Float64(),
			vy: rand.Float64(),
			vz: rand.Float64(),
		}
	}

	for step := 0; step < steps; step++ {
		// 更新速度
		for i := 0; i < numBodies; i++ {
			bi := &bodies[i]
			for j := 0; j < numBodies; j++ {
				if i == j {
					continue
				}
				bj := &bodies[j]
				dx := bj.x - bi.x
				dy := bj.y - bi.y
				dz := bj.z - bi.z
				dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
				if dist == 0 {
					continue
				}
				force := 1 / (dist * dist)
				bi.vx += force * dx / dist
				bi.vy += force * dy / dist
				bi.vz += force * dz / dist
			}
		}

		// 更新位置
		for i := 0; i < numBodies; i++ {
			bi := &bodies[i]
			bi.x += bi.vx
			bi.y += bi.vy
			bi.z += bi.vz
		}
	}
}

// JSON解析

func (s *ToolboxBenchmarkService) jsonProcessing() int {
	start := time.Now()
	s.jsonProcessingTask()
	duration := time.Since(start)
	return s.calculateCpuScore(duration)
}

func (s *ToolboxBenchmarkService) jsonProcessingTask() {
	numElements := 500000

	elements := make([]map[string]any, 0, numElements)
	for j := 0; j < numElements; j++ {
		elements = append(elements, map[string]any{
			"id":    j,
			"value": fmt.Sprintf("Value%d", j),
		})
	}

	encoded, err := json.Marshal(elements)
	if err != nil {
	}

	var parsed []map[string]any
	err = json.Unmarshal(encoded, &parsed)
	if err != nil {
	}
}

// 内存性能

func (s *ToolboxBenchmarkService) memoryTestTask() map[string]any {
	results := make(map[string]any)
	data := make([]byte, 100*1024*1024) // 100 MB
	_, _ = cryptorand.Read(data)

	start := time.Now()
	// 内存读写速度
	results["bandwidth"] = s.memoryBandwidthTest(data)
	// 内存访问延迟
	results["latency"] = s.memoryLatencyTest(data)
	duration := time.Since(start)
	results["score"] = s.calculateScore(duration)

	return results
}

func (s *ToolboxBenchmarkService) memoryBandwidthTest(data []byte) string {
	dataSize := len(data)

	startTime := time.Now()
	for i := 0; i < dataSize; i++ {
		data[i] ^= 0xFF
	}

	duration := time.Since(startTime).Seconds()
	if duration == 0 {
		return "N/A"
	}
	speed := float64(dataSize) / duration / (1024 * 1024)
	return fmt.Sprintf("%.2f MB/s", speed)
}

func (s *ToolboxBenchmarkService) memoryLatencyTest(data []byte) string {
	dataSize := len(data)
	indices := rand.Perm(dataSize)

	startTime := time.Now()
	sum := byte(0)
	for _, idx := range indices {
		sum ^= data[idx]
	}
	duration := time.Since(startTime).Seconds()
	if duration == 0 {
		return "N/A"
	}
	avgLatency := duration * 1e9 / float64(dataSize)
	return fmt.Sprintf("%.2f ns", avgLatency)
}

// 硬盘IO

func (s *ToolboxBenchmarkService) diskTestTask() map[string]any {
	results := make(map[string]any)
	blockSizes := []int64{4 * 1024, 64 * 1024, 1 * 1024 * 1024} // 4K, 64K, 1M

	tmpDir, err := os.MkdirTemp("", "disk_benchmark")
	if err != nil {
		return results
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	testFile := filepath.Join(tmpDir, "testfile")
	start := time.Now()
	for _, blockSize := range blockSizes {
		blockSizeKB := blockSize / 1024
		result := s.diskIOTest(testFile, blockSize)
		results[fmt.Sprintf("%d", blockSizeKB)] = result
	}
	duration := time.Since(start)
	results["score"] = s.calculateScore(duration)

	return results
}

func (s *ToolboxBenchmarkService) diskIOTest(testFile string, blockSize int64) map[string]any {
	result := make(map[string]any)

	// 确定测试参数
	count := int64(3000)
	if blockSize >= 64*1024 {
		count = 2000
	}
	if blockSize >= 1*1024*1024 {
		count = 500
	}

	// 写测试
	result["write_speed"] = s.diskWriteTest(testFile, blockSize, count)
	// 读测试
	result["read_speed"] = s.diskReadTest(testFile, blockSize, count)

	return result
}

func (s *ToolboxBenchmarkService) diskWriteTest(fileName string, blockSize int64, count int64) string {
	var output string
	var err error

	blockSizeKB := blockSize / 1024
	output, err = shell.Execf("dd if=/dev/zero of=%s bs=%dk count=%d oflag=direct 2>&1",
		fileName, blockSizeKB, count)

	if err != nil {
		return ""
	}

	return s.parseCommandOutput(output)
}

func (s *ToolboxBenchmarkService) diskReadTest(fileName string, blockSize int64, count int64) string {
	var output string
	var err error

	blockSizeKB := blockSize / 1024
	output, err = shell.Execf("dd if=%s of=/dev/null bs=%dk count=%d iflag=direct 2>&1",
		fileName, blockSizeKB, count)
	if err != nil {
		return ""
	}

	return s.parseCommandOutput(output)
}

func (s *ToolboxBenchmarkService) parseCommandOutput(output string) string {
	speed := "N/A"
	mbRegex := regexp.MustCompile(`(\d+\.?\d*)\s*[MG]B/s`)
	if matches := mbRegex.FindStringSubmatch(output); len(matches) > 1 {
		speed = strings.TrimSpace(matches[0])
	}

	return speed
}
