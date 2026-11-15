package s3sdk

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

const unsignedPayload = "UNSIGNED-PAYLOAD"

type completedPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

// Put 上传一个对象：内容不足一个分片时走单次 PUT，否则流式分片上传
func (c *S3) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	// 用动态 buffer 读取首块，避免小对象也分配整个 partSize
	var first bytes.Buffer
	n, err := io.CopyN(&first, body, c.partSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if n < c.partSize {
		// 全部内容不足一个分片 → 单次 PUT（含空对象）
		return c.putObject(ctx, key, first.Bytes(), contentType)
	}
	// 已读满一个分片，可能还有更多 → 流式分片上传
	return c.uploadMultipart(ctx, key, first.Bytes(), body, contentType)
}

func (c *S3) putObject(ctx context.Context, key string, data []byte, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.objectURL(key), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(data))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("x-amz-content-sha256", c.payloadHash(data))

	_, _, err = c.do(ctx, req, http.StatusOK) //nolint:bodyclose
	return err
}

func (c *S3) uploadMultipart(ctx context.Context, key string, first []byte, body io.Reader, contentType string) error {
	uploadID, err := c.initiate(ctx, key, contentType)
	if err != nil {
		return err
	}

	parts, err := c.uploadParts(ctx, key, uploadID, first, body)
	if err != nil {
		_ = c.abort(ctx, key, uploadID)
		return err
	}
	if err := c.complete(ctx, key, uploadID, parts); err != nil {
		_ = c.abort(ctx, key, uploadID)
		return err
	}
	return nil
}

func (c *S3) initiate(ctx context.Context, key, contentType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.objectURL(key)+"?uploads", nil)
	if err != nil {
		return "", err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	body, _, err := c.do(ctx, req, http.StatusOK) //nolint:bodyclose
	if err != nil {
		return "", err
	}

	var result struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("s3: parse initiate response: %w", err)
	}
	if result.UploadID == "" {
		return "", errors.New("s3: empty upload id in initiate response")
	}
	return result.UploadID, nil
}

// uploadParts 顺序读取、并发上传分片。errgroup 的并发上限会对读取形成背压，
// 因此常驻内存约为 concurrency × partSize，而非整个文件大小
func (c *S3) uploadParts(
	ctx context.Context,
	key, uploadID string,
	first []byte,
	body io.Reader,
) ([]completedPart, error) {
	partSize, concurrency := c.planParts(body)
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	var mu sync.Mutex
	var parts []completedPart

	submit := func(partNum int, chunk []byte) {
		g.Go(func() error {
			etag, err := c.putPart(ctx, key, uploadID, partNum, chunk)
			if err != nil {
				return err
			}
			mu.Lock()
			parts = append(parts, completedPart{PartNumber: partNum, ETag: etag})
			mu.Unlock()
			return nil
		})
	}

	submit(1, first)

	// 某分片失败时 errgroup 会取消 ctx，循环随即停止继续读取大文件
	for partNum := 2; ctx.Err() == nil; partNum++ {
		buf := make([]byte, partSize)
		n, err := io.ReadFull(body, buf)
		if n > 0 {
			if partNum > maxParts {
				_ = g.Wait()
				return nil, fmt.Errorf("s3: object exceeds %d parts", maxParts)
			}
			submit(partNum, buf[:n])
		}
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				_ = g.Wait()
				return nil, err
			}
			break
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 并发完成顺序不定，complete 要求按分片号升序
	slices.SortFunc(parts, func(a, b completedPart) int { return a.PartNumber - b.PartNumber })
	return parts, nil
}

// planParts 已知大小时把分片放大到装得进 maxParts 片，否则 5MB 一片的对象超过约 50GB 就传不完；
// 分片变大后压低并发，常驻内存不超过 maxBuffered
func (c *S3) planParts(body io.Reader) (int64, int) {
	file, ok := body.(interface{ Stat() (fs.FileInfo, error) })
	if !ok {
		return c.partSize, c.concurrency
	}
	info, err := file.Stat()
	if err != nil {
		return c.partSize, c.concurrency
	}

	// 首片已按默认大小读走，其余内容要装进剩下的分片
	partSize := max(c.partSize, info.Size()/(maxParts-1)+1)
	concurrency := max(1, min(c.concurrency, int(maxBuffered/partSize)))
	return partSize, concurrency
}

func (c *S3) putPart(ctx context.Context, key, uploadID string, partNum int, chunk []byte) (string, error) {
	query := url.Values{
		"partNumber": {strconv.Itoa(partNum)},
		"uploadId":   {uploadID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.objectURL(key)+"?"+query.Encode(), bytes.NewReader(chunk))
	if err != nil {
		return "", err
	}
	req.ContentLength = int64(len(chunk))
	req.Header.Set("x-amz-content-sha256", c.payloadHash(chunk))

	_, res, err := c.do(ctx, req, http.StatusOK) //nolint:bodyclose
	if err != nil {
		return "", err
	}
	etag := res.Header.Get("ETag")
	if etag == "" {
		return "", errors.New("s3: missing ETag in upload part response")
	}
	return etag, nil
}

func (c *S3) complete(ctx context.Context, key, uploadID string, parts []completedPart) error {
	payload := struct {
		XMLName xml.Name        `xml:"CompleteMultipartUpload"`
		XMLNS   string          `xml:"xmlns,attr"`
		Parts   []completedPart `xml:"Part"`
	}{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Parts: parts}

	body, err := xml.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.objectURL(key)+"?uploadId="+url.QueryEscape(uploadID), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("x-amz-content-sha256", sha256Hex(body))
	sum := md5.Sum(body) //nolint:gosec
	req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))

	_, _, err = c.do(ctx, req, http.StatusOK) //nolint:bodyclose
	return err
}

// abort 清理未完成的分片上传，取消导致的失败也要清，否则残片会一直占用存储
func (c *S3) abort(ctx context.Context, key, uploadID string) error {
	ctx = context.WithoutCancel(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.objectURL(key)+"?uploadId="+url.QueryEscape(uploadID), nil)
	if err != nil {
		return err
	}
	_, _, err = c.do(ctx, req, http.StatusNoContent) //nolint:bodyclose
	return err
}

func (c *S3) payloadHash(data []byte) string {
	if strings.HasPrefix(c.base, "https://") {
		return unsignedPayload
	}
	return sha256Hex(data)
}
