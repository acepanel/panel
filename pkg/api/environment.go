package api

import "fmt"

type Environment struct {
	Type        string `json:"type"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type Environments []*Environment

// Environments 返回所有环境
func (r *API) Environments() (*Environments, error) {
	resp, err := r.client.R().SetResult(&Response{}).Get("/environments")
	if err != nil {
		return nil, err
	}
	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf("failed to get environments: %s", resp.String())
	}

	environments, err := getResponseData[Environments](resp)
	if err != nil {
		return nil, err
	}

	return environments, nil
}

// EnvironmentCallback 环境下载回调
func (r *API) EnvironmentCallback(typ, slug string) error {
	resp, err := r.client.R().
		SetResult(&Response{}).
		Post(fmt.Sprintf("/environments/%s/%s/callback", typ, slug))
	if err != nil {
		return err
	}
	if !resp.IsStatusSuccess() {
		return fmt.Errorf("failed to callback environment: %s", resp.String())
	}

	return nil
}
