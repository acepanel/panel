package api

import (
	"fmt"
	"log"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"resty.dev/v3"
)

type API struct {
	panelVersion string
	client       *resty.Client
}

func NewAPI(panelVersion, locale string, url ...string) *API {
	if len(panelVersion) == 0 {
		panic("panel version is required")
	}
	if len(url) == 0 {
		url = append(url, "https://api.acepanel.net")
	}

	hostInfo, err := host.Info()
	if err != nil {
		log.Fatalf("failed to get host info: %v", err)
	}

	client := resty.New()
	client.SetRetryCount(3)
	client.SetTimeout(10 * time.Second)
	client.SetBaseURL(url[0])
	client.SetHeader(
		"User-Agent",
		fmt.Sprintf("acepanel/%s/%s %s/%s arch/%s kernel/%s",
			panelVersion, locale, hostInfo.Platform, hostInfo.PlatformVersion, hostInfo.KernelArch, hostInfo.KernelVersion,
		),
	)
	client.SetQueryParam("locale", locale)

	return &API{
		panelVersion: panelVersion,
		client:       client,
	}
}

func (r *API) callback(path string) error {
	resp, err := r.client.R().Post(path)
	if err != nil {
		return err
	}
	if !resp.IsStatusSuccess() {
		return fmt.Errorf("failed to post %s: %s", path, resp.String())
	}

	return nil
}

func get[T any](r *API, path string, query map[string]string) (*T, error) {
	var result struct {
		Data T `json:"data"`
	}
	resp, err := r.client.R().SetQueryParams(query).SetResult(&result).Get(path)
	if err != nil {
		return nil, err
	}
	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf("failed to get %s: %s", path, resp.String())
	}

	return &result.Data, nil
}
