package s3sdk

import (
	"bytes"
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"slices"
)

// Delete 删除一个或多个对象，自动按每批 1000 个分批请求
func (c *S3) Delete(ctx context.Context, keys ...string) error {
	for batch := range slices.Chunk(keys, 1000) {
		if err := c.deleteBatch(ctx, batch); err != nil {
			return err
		}
	}
	return nil
}

func (c *S3) deleteBatch(ctx context.Context, keys []string) error {
	type object struct {
		Key string `xml:"Key"`
	}
	payload := struct {
		XMLName xml.Name `xml:"Delete"`
		XMLNS   string   `xml:"xmlns,attr"`
		Quiet   bool     `xml:"Quiet"`
		Objects []object `xml:"Object"`
	}{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Quiet: true}
	for _, key := range keys {
		payload.Objects = append(payload.Objects, object{Key: key})
	}

	body, err := xml.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"?delete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("x-amz-content-sha256", sha256Hex(body))
	// S3 对 DeleteObjects 强制要求 Content-MD5
	sum := md5.Sum(body) //nolint:gosec
	req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))

	respBody, _, err := c.do(ctx, req, http.StatusOK) //nolint:bodyclose
	if err != nil {
		return err
	}

	// Quiet 模式下响应只含失败项
	var result struct {
		Errors []struct {
			Key     string `xml:"Key"`
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		} `xml:"Error"`
	}
	if err := xml.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("s3: parse delete response: %w", err)
	}
	if len(result.Errors) > 0 {
		e := result.Errors[0]
		return fmt.Errorf("s3: delete failed for %q: %s: %s (%d errors total)", e.Key, e.Code, e.Message, len(result.Errors))
	}
	return nil
}
