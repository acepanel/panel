package api

import "fmt"

type Category struct {
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}

type Categories []*Category

// Categories 返回所有分类
func (r *API) Categories() (*Categories, error) {
	resp, err := r.client.R().SetResult(&Response{}).Get("/categories")
	if err != nil {
		return nil, err
	}
	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf("failed to get categories: %s", resp.String())
	}

	categories, err := getResponseData[Categories](resp)
	if err != nil {
		return nil, err
	}

	return categories, nil
}
