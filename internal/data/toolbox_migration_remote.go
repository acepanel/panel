package data

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"resty.dev/v3"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

type migrationRemoteRepo struct {
	t *gotext.Locale
}

func NewMigrationRemoteRepo(t *gotext.Locale) biz.MigrationRemoteRepo {
	return &migrationRemoteRepo{t: t}
}

func (r *migrationRemoteRepo) Request(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	method, path string,
	body any,
) ([]byte, error) {
	client := r.client(conn, 30*time.Second)
	defer func() { _ = client.Close() }()

	req := client.R().SetContext(ctx)
	if body != nil {
		if method == http.MethodGet {
			req.SetQueryParams(cast.ToStringMapString(body))
		} else {
			req.SetBody(body)
		}
	}
	resp, err := req.Execute(method, path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode(), strings.TrimSpace(resp.String()))
	}
	return resp.Bytes(), nil
}

// Upload 分片上传文件到目标面板，支持断点续传
func (r *migrationRemoteRepo) Upload(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	local, remote string,
	progress types.MigrationProgress,
) error {
	// migrationChunkSize 分片上传的单片大小
	const migrationChunkSize = 10 << 20

	file, err := os.Open(local)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	chunks := max(int((info.Size()+migrationChunkSize-1)/migrationChunkSize), 1)

	meta := map[string]any{
		"path": filepath.Dir(remote), "file_name": filepath.Base(remote),
		"file_hash": hash, "chunk_count": chunks, "chunk_size": migrationChunkSize, "force": true,
	}
	body, err := r.Request(ctx, conn, http.MethodPost, "/api/file/chunk/start", meta)
	if err != nil {
		return fmt.Errorf("chunk start: %w", err)
	}
	uploaded := r.uploadedChunks(body)

	// ReadFull 保证除末片外读满分片大小，避免短读导致分片错位
	var transferred int64
	buffer := make([]byte, migrationChunkSize)
	for index := range chunks {
		n, readErr := io.ReadFull(file, buffer)
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return fmt.Errorf("read chunk %d: %w", index, readErr)
		}
		// 断点续传时已传分片同样计入进度，否则进度会从中途重新爬
		transferred += int64(n)
		if slices.Contains(uploaded, index) {
			continue
		}
		chunk := buffer[:n]
		sum := sha256.Sum256(chunk)
		if err = r.uploadChunk(ctx, conn, meta, index, hex.EncodeToString(sum[:]), chunk); err != nil {
			return fmt.Errorf("upload chunk %d: %w", index, err)
		}
		if progress != nil {
			progress(transferred, info.Size())
		}
	}

	if _, err = r.Request(ctx, conn, http.MethodPost, "/api/file/chunk/finish", meta); err != nil {
		return fmt.Errorf("chunk finish: %w", err)
	}
	return nil
}

// Exec 调用目标面板的 SSE 接口执行命令，直到收到完成或错误事件
func (r *migrationRemoteRepo) Exec(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	command string,
) error {
	client := r.client(conn, 0)
	defer func() { _ = client.Close() }()

	resp, err := client.R().
		SetContext(ctx).
		SetResponseDoNotParse(true).
		SetBody(map[string]string{"command": command}).
		Post("/api/toolbox_migration/exec")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode() != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode(), strings.TrimSpace(string(body)))
	}

	// 命令报错时单行输出可能很长（如数据库导入错误附带语句内容），加大行缓冲
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		switch line := scanner.Text(); {
		case strings.HasPrefix(line, "event: done"):
			return nil
		case strings.HasPrefix(line, "event: error"):
			if scanner.Scan() {
				return errors.New(strings.TrimPrefix(scanner.Text(), "data: "))
			}
			return errors.New(r.t.Get("target command failed"))
		}
	}
	if err = scanner.Err(); err != nil {
		return err
	}
	return errors.New(r.t.Get("the connection to the target server was interrupted"))
}

// client 构造带 HMAC 签名的目标面板客户端
func (r *migrationRemoteRepo) client(conn *request.ToolboxMigrationConnection, timeout time.Duration) *resty.Client {
	client := resty.New().
		SetBaseURL(strings.TrimRight(conn.URL, "/")).
		SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true}). //nolint:gosec
		SetHeader("Content-Type", "application/json")
	if timeout > 0 {
		client.SetTimeout(timeout)
	}

	// 签名中间件放在 MiddlewareRequestCreate 之后，此时 RawRequest 已构建完毕
	sign := resty.RequestMiddleware(func(_ *resty.Client, req *resty.Request) error {
		raw := req.RawRequest
		var body []byte
		if raw.Body != nil {
			body, _ = io.ReadAll(raw.Body)
			raw.Body = io.NopCloser(bytes.NewReader(body))
		}

		// 签名路径必须是 /api/... 部分（服务端验签时入口前缀已被 strip）
		path := raw.URL.Path
		if index := strings.Index(path, "/api/"); index > 0 {
			path = path[index:]
		}
		canonical := fmt.Sprintf("%s\n%s\n%s\n%s", raw.Method, path, raw.URL.Query().Encode(), r.sha256Hex(body))
		timestamp := time.Now().Unix()
		signature := r.hmacSHA256(fmt.Sprintf("HMAC-SHA256\n%d\n%s", timestamp, r.sha256Hex([]byte(canonical))), conn.Token)

		raw.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
		raw.Header.Set("Authorization", fmt.Sprintf("HMAC-SHA256 Credential=%d, Signature=%s", conn.TokenID, signature))
		return nil
	})
	client.SetRequestMiddlewares(resty.MiddlewareRequestCreate, sign)
	return client
}

// uploadedChunks 解析已上传的分片下标
func (r *migrationRemoteRepo) uploadedChunks(body []byte) []int {
	var response struct {
		Data struct {
			UploadedChunks []int `json:"uploaded_chunks"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &response) != nil {
		return nil
	}
	return response.Data.UploadedChunks
}

// uploadChunk 以 multipart 提交单个分片
func (r *migrationRemoteRepo) uploadChunk(
	ctx context.Context,
	conn *request.ToolboxMigrationConnection,
	meta map[string]any,
	index int,
	hash string,
	chunk []byte,
) error {
	client := r.client(conn, 30*time.Minute)
	defer func() { _ = client.Close() }()

	name := cast.ToString(meta["file_name"])
	resp, err := client.R().
		SetContext(ctx).
		SetFormData(map[string]string{
			"path": cast.ToString(meta["path"]), "file_name": name, "file_hash": cast.ToString(meta["file_hash"]),
			"chunk_index": strconv.Itoa(index), "chunk_hash": hash,
		}).
		SetFileReader("file", name, bytes.NewReader(chunk)).
		Post("/api/file/chunk/upload")
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode(), strings.TrimSpace(resp.String()))
	}
	return nil
}

func (r *migrationRemoteRepo) sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (r *migrationRemoteRepo) hmacSHA256(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
