package api

import (
	"fmt"
	"slices"
	"time"

	"github.com/libtnb/utils/env"
)

type VersionDownload struct {
	URL      string `json:"url"`
	Arch     string `json:"arch"`
	Checksum string `json:"checksum"`
}

type Version struct {
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Type        string            `json:"type"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Downloads   []VersionDownload `json:"downloads"`
}

type Versions []Version

// LatestVersion 返回最新版本
func (r *API) LatestVersion(channel string) (*Version, error) {
	resp, err := r.client.R().SetResult(&Response{}).SetQueryParam("channel", channel).Get("/version/latest")
	if err != nil {
		return nil, err
	}
	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf("failed to get latest version: %s", resp.String())
	}

	version, err := getResponseData[Version](resp)
	if err != nil {
		return nil, err
	}

	arch := "amd64"
	if env.IsArm() {
		arch = "arm64"
	}
	version.Downloads = slices.DeleteFunc(version.Downloads, func(item VersionDownload) bool {
		return item.Arch != arch
	})

	return version, nil
}

// IntermediateVersions 返回当前版本之后的所有版本
func (r *API) IntermediateVersions(channel string) (*Versions, error) {
	resp, err := r.client.R().
		SetQueryParam("start", r.panelVersion).
		SetQueryParam("channel", channel).
		SetResult(&Response{}).Get("/version/intermediate")
	if err != nil {
		return nil, err
	}
	if !resp.IsStatusSuccess() {
		return nil, fmt.Errorf("failed to get intermediate versions: %s", resp.String())
	}

	versions, err := getResponseData[Versions](resp)
	if err != nil {
		return nil, err
	}

	return versions, nil
}
