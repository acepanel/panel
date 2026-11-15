package s3sdk

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// minPartSize 是 S3 规定的最小分片大小（最后一片除外）
	minPartSize = 5 * 1024 * 1024
	// defaultPartSize 是默认分片大小
	defaultPartSize = 5 * 1024 * 1024
	// maxParts 是 S3 规定的单次分片上传的最大分片数
	maxParts = 10000
	// maxBuffered 是分片被放大后的内存预算，超出时压低并发
	maxBuffered = 128 * 1024 * 1024

	defaultMaxRetries = 3
)

// defaultClient 未配置 HTTP 客户端时的兜底
// 不设 Client.Timeout：分片上传耗时随文件大小变化，只能靠分阶段超时兜住对端黑洞
var defaultClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: time.Minute,
	},
}

// Config 是创建 S3 客户端的配置
type Config struct {
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string

	Endpoint  string // 可选：自定义端点（不含 bucket），可带或不带协议
	Scheme    string // 可选：Endpoint 不含协议时使用，默认 https
	PathStyle bool   // true 为 path-style，false 为 virtual-hosted（默认）

	PartSize    int64 // 可选：分片大小，默认 5MB，最小 5MB
	Concurrency int   // 可选：分片并发数，默认 1
	MaxRetries  int   // 可选：单分片重试次数，默认 3

	Client *http.Client // 可选：自定义 HTTP 客户端
}

// S3 是绑定到单个 bucket 的客户端
type S3 struct {
	region    string
	accessKey string
	secretKey string
	base      string // 预计算的基础 URL（已包含寻址风格的处理）

	partSize    int64
	concurrency int
	maxRetries  int
	client      *http.Client

	mu       sync.Mutex
	keyDate  string // 签名密钥缓存对应的日期（UTC yyyymmdd）
	keyCache []byte // 按天缓存的签名密钥
}

// New 按配置创建一个 S3 客户端。
func New(cfg Config) *S3 {
	partSize := cfg.PartSize
	if partSize < minPartSize {
		partSize = defaultPartSize
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	return &S3{
		region:      cfg.Region,
		accessKey:   cfg.AccessKey,
		secretKey:   cfg.SecretKey,
		base:        computeBase(cfg),
		partSize:    partSize,
		concurrency: concurrency,
		maxRetries:  maxRetries,
		client:      cfg.Client,
	}
}

// do 对请求签名并执行，对可重试错误（5xx、网络错误）按指数退避重试
// 读取完整响应体；状态码不等于 wantStatus 时返回 *apiError
// 返回的 *http.Response 仅用于读取响应头，其 Body 已被读尽并关闭
func (c *S3) do(ctx context.Context, req *http.Request, wantStatus int) ([]byte, *http.Response, error) {
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			// 重置请求体以便重放（bytes/strings reader 经 http.NewRequest 会自动提供 GetBody）
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, nil, err
				}
				req.Body = body
			}
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}

		body, res, err := c.doOnce(req, wantStatus)
		if err == nil || attempt >= c.maxRetries || !isRetryable(err) {
			return body, res, err
		}
	}
}

func (c *S3) doOnce(req *http.Request, wantStatus int) ([]byte, *http.Response, error) {
	c.signRequest(req)

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, res, err
	}
	if res.StatusCode != wantStatus {
		return body, res, &apiError{status: res.StatusCode, statusText: res.Status, body: body}
	}
	return body, res, nil
}

func (c *S3) httpClient() *http.Client {
	if c.client == nil {
		return defaultClient
	}
	return c.client
}

// objectURL 返回对象的请求 URL
func (c *S3) objectURL(key string) string {
	return c.base + "/" + uriEncode(key, false)
}

// apiError 表示一个非预期状态码的 S3 响应
type apiError struct {
	status     int
	statusText string
	body       []byte
}

func (e *apiError) Error() string {
	// 优先解析 S3 的 XML 错误体以给出更友好的信息
	var x struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	}
	if xml.Unmarshal(e.body, &x) == nil && x.Code != "" {
		return fmt.Sprintf("s3: %s: %s", x.Code, x.Message)
	}
	if trimmed := bytes.TrimSpace(e.body); len(trimmed) > 0 {
		return fmt.Sprintf("s3: unexpected status %s: %s", e.statusText, trimmed)
	}
	return "s3: unexpected status " + e.statusText
}

// computeBase 根据寻址风格与端点预计算请求基础 URL
// path-style：     {base} = {endpoint|aws}/{bucket}，对象 URL 为 {base}/{key}
// virtual-hosted： {base} = {scheme}://{bucket}.{host}，对象 URL 为 {base}/{key}
func computeBase(cfg Config) string {
	scheme := cfg.Scheme
	if scheme == "" {
		scheme = "https"
	}

	if cfg.Endpoint != "" {
		ep := cfg.Endpoint
		if !strings.Contains(ep, "://") {
			ep = scheme + "://" + ep
		}
		ep = strings.TrimRight(ep, "/")

		if cfg.PathStyle {
			return ep + "/" + cfg.Bucket
		}
		// virtual-hosted：把 bucket 插入 host 最前
		if u, err := url.Parse(ep); err == nil {
			u.Host = cfg.Bucket + "." + u.Host
			return strings.TrimRight(u.String(), "/")
		}
		return ep
	}

	// 无自定义端点，使用 AWS S3 默认端点
	if cfg.PathStyle {
		return fmt.Sprintf("https://s3.%s.amazonaws.com/%s", cfg.Region, cfg.Bucket)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com", cfg.Bucket, cfg.Region)
}

// backoff 返回第 attempt 次重试前的等待时长（指数退避，上限 5 秒）
func backoff(attempt int) time.Duration {
	if d := time.Duration(1<<(attempt-1)) * 100 * time.Millisecond; d < 5*time.Second {
		return d
	}
	return 5 * time.Second
}

// isRetryable 判断错误是否值得重试：5xx 服务端错误或网络层错误
func isRetryable(err error) bool {
	if ae, ok := errors.AsType[*apiError](err); ok {
		return ae.status >= 500
	}
	var ne net.Error
	return errors.As(err, &ne)
}
