package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// ExecWithStdinProgress 以外部命令消费 stdin,同时按周期通过 progress 回调输出进度
// name/args 为命令与参数; stdin 是被喂给命令的字节流; total 为已知总字节数(<=0 表示未知)
// interval 是进度回调周期; progress(written, total, rate) 中 rate 单位是 bytes/s
// pipe 存在背压,written 增速 ≈ 命令处理速率,可作为大概进度使用
func ExecWithStdinProgress(
	ctx context.Context,
	name string, args []string,
	stdin io.Reader, total int64,
	interval time.Duration,
	progress func(written, total int64, rate float64),
) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	pr := &progressReader{r: stdin}
	cmd.Stdin = pr

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	done := make(chan struct{})
	if progress != nil && interval > 0 {
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			start := time.Now()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					w := pr.written.Load()
					elapsed := time.Since(start).Seconds()
					rate := 0.0
					if elapsed > 0 {
						rate = float64(w) / elapsed
					}
					progress(w, total, rate)
				}
			}
		}()
	}

	err := cmd.Wait()
	close(done)
	if err != nil {
		return strings.TrimSpace(stdout.String()), fmt.Errorf("run %s failed, err: %w, stderr: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

type progressReader struct {
	r       io.Reader
	written atomic.Int64
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.written.Add(int64(n))
	}
	return n, err
}
