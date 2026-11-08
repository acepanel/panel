package stalwart

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// 管理 API 走 JMAP：所有对象都是 x:<Type>/get|set|query，单例对象 id 固定为 singleton
const (
	jmapCore     = "urn:ietf:params:jmap:core"
	jmapStalwart = "urn:stalwart:jmap"
	singleton    = "singleton"
)

type client struct {
	url      string
	username string
	password string
	http     *http.Client
}

func newClient(username, password string) *client {
	return &client{
		url:      fmt.Sprintf("http://127.0.0.1:%d/jmap/", httpPort),
		username: username,
		password: password,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// call 执行单个 JMAP 方法并返回其结果对象
func (c *client) call(ctx context.Context, method string, args any) (json.RawMessage, error) {
	if args == nil {
		args = map[string]any{}
	}
	body, err := json.Marshal(map[string]any{
		"using":       []string{jmapCore, jmapStalwart},
		"methodCalls": []any{[]any{method, args, "c1"}},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var problem struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&problem)
		if problem.Detail == "" {
			problem.Detail = resp.Status
		}
		return nil, fmt.Errorf("%s: %s", method, problem.Detail)
	}

	var envelope struct {
		MethodResponses []json.RawMessage `json:"methodResponses"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if len(envelope.MethodResponses) == 0 {
		return nil, fmt.Errorf("%s: empty response", method)
	}

	var tuple []json.RawMessage
	if err = json.Unmarshal(envelope.MethodResponses[0], &tuple); err != nil || len(tuple) < 2 {
		return nil, fmt.Errorf("%s: malformed response", method)
	}
	var name string
	_ = json.Unmarshal(tuple[0], &name)
	if name == "error" {
		var e struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		}
		_ = json.Unmarshal(tuple[1], &e)
		return nil, fmt.Errorf("%s: %s (%s)", method, e.Description, e.Type)
	}

	return tuple[1], nil
}

// setError 是 /set 响应里 notCreated、notUpdated、notDestroyed 的条目
type setError struct {
	Type             string `json:"type"`
	Description      string `json:"description"`
	ValidationErrors []struct {
		Type     string `json:"type"`
		Property string `json:"property"`
	} `json:"validationErrors"`
	LinkedObjects []struct {
		Object string `json:"object"`
		ID     string `json:"id"`
	} `json:"linkedObjects"`
}

func (e setError) Error() string {
	var parts []string
	if e.Description != "" {
		parts = append(parts, e.Description)
	}
	for _, v := range e.ValidationErrors {
		parts = append(parts, fmt.Sprintf("%s: %s", v.Property, v.Type))
	}
	for _, l := range e.LinkedObjects {
		parts = append(parts, fmt.Sprintf("linked %s %s", l.Object, l.ID))
	}
	if len(parts) == 0 {
		return e.Type
	}
	return e.Type + ": " + strings.Join(parts, ", ")
}

type setResponse struct {
	Created      map[string]struct{ ID string }
	Updated      map[string]json.RawMessage
	Destroyed    []string
	NotCreated   map[string]setError
	NotUpdated   map[string]setError
	NotDestroyed map[string]setError
}

func (c *client) set(ctx context.Context, typ string, args map[string]any) (*setResponse, error) {
	raw, err := c.call(ctx, "x:"+typ+"/set", args)
	if err != nil {
		return nil, err
	}
	var resp setResponse
	if err = json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	for _, e := range resp.NotCreated {
		return nil, e
	}
	for _, e := range resp.NotUpdated {
		return nil, e
	}
	for _, e := range resp.NotDestroyed {
		return nil, e
	}
	return &resp, nil
}

// create 创建单个对象并返回其 id
func (c *client) create(ctx context.Context, typ string, obj any) (string, error) {
	resp, err := c.set(ctx, typ, map[string]any{"create": map[string]any{"new": obj}})
	if err != nil {
		return "", err
	}
	created, ok := resp.Created["new"]
	if !ok {
		return "", errors.New(typ + ": object was not created")
	}
	return created.ID, nil
}

func (c *client) update(ctx context.Context, typ, id string, patch any) error {
	_, err := c.set(ctx, typ, map[string]any{"update": map[string]any{id: patch}})
	return err
}

func (c *client) destroy(ctx context.Context, typ string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := c.set(ctx, typ, map[string]any{"destroy": ids})
	return err
}

// action 触发一个服务端动作，如 ReloadSettings、ReloadTlsCertificates
func (c *client) action(ctx context.Context, name string) error {
	_, err := c.create(ctx, "Action", map[string]any{"@type": name})
	return err
}

// get 按 id 读取对象，ids 为 nil 时读取全部
func get[T any](ctx context.Context, c *client, typ string, ids []string, properties []string) ([]T, error) {
	args := map[string]any{"ids": ids}
	if ids == nil {
		args["ids"] = nil
	}
	if len(properties) > 0 {
		args["properties"] = properties
	}
	raw, err := c.call(ctx, "x:"+typ+"/get", args)
	if err != nil {
		return nil, err
	}
	var resp struct {
		List []T `json:"list"`
	}
	if err = json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return resp.List, nil
}

// getOne 读取单例或指定 id 的对象
func getOne[T any](ctx context.Context, c *client, typ, id string) (*T, error) {
	list, err := get[T](ctx, c, typ, []string{id}, nil)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s %s not found", typ, id)
	}
	return &list[0], nil
}

type queryResult struct {
	IDs   []string `json:"ids"`
	Total uint     `json:"total"`
}

// query 按条件分页查询对象 id，limit 为 0 时不分页
func (c *client) query(ctx context.Context, typ string, filter map[string]any, position, limit uint) (*queryResult, error) {
	args := map[string]any{"calculateTotal": true}
	if len(filter) > 0 {
		args["filter"] = filter
	}
	if limit > 0 {
		args["position"] = position
		args["limit"] = limit
	}
	raw, err := c.call(ctx, "x:"+typ+"/query", args)
	if err != nil {
		return nil, err
	}
	var result queryResult
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// queryGet 查询后直接取回对象，分页由 JMAP 完成
func queryGet[T any](ctx context.Context, c *client, typ string, filter map[string]any, position, limit uint, properties []string) ([]T, uint, error) {
	result, err := c.query(ctx, typ, filter, position, limit)
	if err != nil {
		return nil, 0, err
	}
	if len(result.IDs) == 0 {
		return []T{}, result.Total, nil
	}
	list, err := get[T](ctx, c, typ, result.IDs, properties)
	if err != nil {
		return nil, 0, err
	}
	return list, result.Total, nil
}

// waitReady 轮询直到管理接口可用，服务启动与重启后都需要等待
func (c *client) waitReady(ctx context.Context, method string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := c.call(callCtx, method, map[string]any{"ids": []string{singleton}})
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("mail server did not become ready in %s: %w", timeout, err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
