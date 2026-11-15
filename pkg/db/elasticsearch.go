package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"resty.dev/v3"
)

// Elasticsearch 通过 REST API 操作 Elasticsearch/OpenSearch
type Elasticsearch struct {
	client *resty.Client
}

// ESIndex 索引信息
type ESIndex struct {
	Name      string `json:"name"`
	Health    string `json:"health"`
	Status    string `json:"status"`
	DocsCount string `json:"docs_count"`
	StoreSize string `json:"store_size"`
}

// ESDocument 文档信息
type ESDocument struct {
	ID     string `json:"id"`
	Index  string `json:"index"`
	Source string `json:"source"`
}

// NewElasticsearch 创建 Elasticsearch 连接
func NewElasticsearch(ctx context.Context, address, username, password string) (*Elasticsearch, error) {
	client := resty.New()
	client.SetBaseURL("http://" + address)
	client.SetTimeout(10 * 1000 * 1000 * 1000) // 10s
	if username != "" && password != "" {
		client.SetBasicAuth(username, password)
	}

	es := &Elasticsearch{client: client}
	if err := es.Ping(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to elasticsearch failed: %w", err)
	}

	return es, nil
}

func (r *Elasticsearch) Close() {
	_ = r.client.Close()
}

func (r *Elasticsearch) Ping(ctx context.Context) error {
	resp, err := r.client.R().SetContext(ctx).Get("/")
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("elasticsearch ping failed: %s", resp.String())
	}

	return nil
}

// Indices 获取所有索引
func (r *Elasticsearch) Indices(ctx context.Context) ([]ESIndex, error) {
	resp, err := r.client.R().SetContext(ctx).Get("/_cat/indices?format=json&h=index,health,status,docs.count,store.size")
	if err != nil {
		return nil, fmt.Errorf("failed to get indices: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to get indices: %s", resp.String())
	}

	var raw []struct {
		Index     string `json:"index"`
		Health    string `json:"health"`
		Status    string `json:"status"`
		DocsCount string `json:"docs.count"`
		StoreSize string `json:"store.size"`
	}
	if err = json.Unmarshal(resp.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse indices: %w", err)
	}

	indices := make([]ESIndex, 0, len(raw))
	for _, item := range raw {
		// 过滤系统索引
		if strings.HasPrefix(item.Index, ".") {
			continue
		}
		indices = append(indices, ESIndex{
			Name:      item.Index,
			Health:    item.Health,
			Status:    item.Status,
			DocsCount: item.DocsCount,
			StoreSize: item.StoreSize,
		})
	}

	return indices, nil
}

// IndexCreate 创建索引
func (r *Elasticsearch) IndexCreate(ctx context.Context, name string) error {
	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		Put("/" + name)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to create index: %s", resp.String())
	}
	return nil
}

// IndexDelete 删除索引
func (r *Elasticsearch) IndexDelete(ctx context.Context, name string) error {
	resp, err := r.client.R().SetContext(ctx).Delete("/" + name)
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to delete index: %s", resp.String())
	}
	return nil
}

// Search 搜索文档
func (r *Elasticsearch) Search(
	ctx context.Context,
	index, query string,
	page, pageSize int,
) ([]ESDocument, int64, error) {
	from := (page - 1) * pageSize
	body := map[string]any{
		"from": from,
		"size": pageSize,
	}
	if query != "" {
		body["query"] = map[string]any{
			"query_string": map[string]any{
				"query": query,
			},
		}
	}

	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post("/" + index + "/_search")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, 0, fmt.Errorf("search failed: %s", resp.String())
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				ID     string          `json:"_id"`
				Index  string          `json:"_index"`
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.Unmarshal(resp.Bytes(), &result); err != nil {
		return nil, 0, fmt.Errorf("failed to parse search result: %w", err)
	}

	docs := make([]ESDocument, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		docs = append(docs, ESDocument{
			ID:     hit.ID,
			Index:  hit.Index,
			Source: string(hit.Source),
		})
	}

	return docs, result.Hits.Total.Value, nil
}

// DocumentGet 获取文档
func (r *Elasticsearch) DocumentGet(ctx context.Context, index, id string) (*ESDocument, error) {
	resp, err := r.client.R().SetContext(ctx).Get("/" + index + "/_doc/" + id)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("document not found: %s", resp.String())
	}

	var result struct {
		ID     string          `json:"_id"`
		Index  string          `json:"_index"`
		Source json.RawMessage `json:"_source"`
	}
	if err = json.Unmarshal(resp.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse document: %w", err)
	}

	return &ESDocument{
		ID:     result.ID,
		Index:  result.Index,
		Source: string(result.Source),
	}, nil
}

// DocumentCreate 创建文档（自动生成 ID）
func (r *Elasticsearch) DocumentCreate(ctx context.Context, index, body string) error {
	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("refresh", "true").
		SetBody(body).
		Post("/" + index + "/_doc")
	if err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}
	if resp.StatusCode() != 201 {
		return fmt.Errorf("failed to create document: %s", resp.String())
	}
	return nil
}

// DocumentUpdate 更新文档
func (r *Elasticsearch) DocumentUpdate(ctx context.Context, index, id, body string) error {
	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("refresh", "true").
		SetBody(body).
		Put("/" + index + "/_doc/" + id)
	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}
	if resp.StatusCode() != 200 && resp.StatusCode() != 201 {
		return fmt.Errorf("failed to update document: %s", resp.String())
	}
	return nil
}

// DocumentDelete 删除文档
func (r *Elasticsearch) DocumentDelete(ctx context.Context, index, id string) error {
	resp, err := r.client.R().
		SetContext(ctx).
		SetQueryParam("refresh", "true").
		Delete("/" + index + "/_doc/" + id)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("failed to delete document: %s", resp.String())
	}
	return nil
}
