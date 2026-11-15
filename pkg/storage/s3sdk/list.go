package s3sdk

import (
	"context"
	"encoding/xml"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"time"
)

// Object 是列举结果中的一个对象
type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass string
}

type listPage struct {
	objects    []Object
	truncated  bool
	nextMarker string
}

// List 返回一个自动翻页、产出 prefix 下所有对象的迭代器
// delimiter 非空（通常为 "/"）时按目录层级分组
// 迭代中若出错，会以 (零值 Object, err) 产出一次随后结束
func (c *S3) List(ctx context.Context, prefix, delimiter string) iter.Seq2[Object, error] {
	return func(yield func(Object, error) bool) {
		marker := ""
		for {
			page, err := c.listPage(ctx, prefix, delimiter, marker)
			if err != nil {
				yield(Object{}, err)
				return
			}
			for _, obj := range page.objects {
				if !yield(obj, nil) {
					return
				}
			}
			if !page.truncated || page.nextMarker == "" {
				return
			}
			marker = page.nextMarker
		}
	}
}

// listPage 用 V1 列举：部分兼容实现（如华为云 OBS）不支持 list-type=2
func (c *S3) listPage(ctx context.Context, prefix, delimiter, marker string) (listPage, error) {
	query := url.Values{}
	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if delimiter != "" {
		query.Set("delimiter", delimiter)
	}
	if marker != "" {
		query.Set("marker", marker)
	}
	target := c.base
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return listPage{}, err
	}

	body, _, err := c.do(ctx, req, http.StatusOK) //nolint:bodyclose
	if err != nil {
		return listPage{}, err
	}

	var result struct {
		IsTruncated bool   `xml:"IsTruncated"`
		NextMarker  string `xml:"NextMarker"`
		Contents    []struct {
			Key          string `xml:"Key"`
			Size         int64  `xml:"Size"`
			LastModified string `xml:"LastModified"`
			ETag         string `xml:"ETag"`
			StorageClass string `xml:"StorageClass"`
		} `xml:"Contents"`
		CommonPrefixes []struct {
			Prefix string `xml:"Prefix"`
		} `xml:"CommonPrefixes"`
	}
	if err := xml.Unmarshal(body, &result); err != nil {
		return listPage{}, fmt.Errorf("s3: parse list response: %w", err)
	}

	page := listPage{truncated: result.IsTruncated, nextMarker: result.NextMarker}
	for _, o := range result.Contents {
		t, _ := time.Parse(time.RFC3339, o.LastModified)
		page.objects = append(page.objects, Object{
			Key:          o.Key,
			Size:         o.Size,
			LastModified: t,
			ETag:         o.ETag,
			StorageClass: o.StorageClass,
		})
	}
	// NextMarker 只在带 delimiter 时才保证返回，缺失时取本页字典序最大的键
	if page.truncated && page.nextMarker == "" {
		if n := len(result.Contents); n > 0 {
			page.nextMarker = result.Contents[n-1].Key
		}
		if n := len(result.CommonPrefixes); n > 0 {
			page.nextMarker = max(page.nextMarker, result.CommonPrefixes[n-1].Prefix)
		}
	}
	return page, nil
}
