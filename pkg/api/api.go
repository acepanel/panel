package api

import (
	"fmt"
	"log"
	"time"

	"github.com/go-rat/utils/copier"
	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/host"
)

type API struct {
	panelVersion string
	client       *resty.Client
}

type Response struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func NewAPI(panelVersion, locale string, url ...string) *API {
	if len(panelVersion) == 0 {
		panic("panel version is required")
	}
	if len(url) == 0 {
		url = append(url, "https://panel.haozi.net/api")
	}

	hostInfo, err := host.Info()
	if err != nil {
		log.Fatalf("failed to get host info: %v", err)
	}

	client := resty.New()
	client.SetTimeout(10 * time.Second)
	client.SetBaseURL(url[0])
	client.SetHeader("User-Agent", fmt.Sprintf("rat-panel/%s %s/%s", panelVersion, hostInfo.Platform, hostInfo.PlatformVersion))
	client.SetQueryParam("locale", locale)

	return &API{
		panelVersion: panelVersion,
		client:       client,
	}
}

func getResponseData[T any](resp *resty.Response) (*T, error) {
	raw, ok := resp.Result().(*Response)
	if !ok {
		return nil, fmt.Errorf("failed to get response data: %s", resp.String())
	}

	res, err := copier.Copy[T](raw.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to copy response data: %w", err)
	}

	return res, nil
}
