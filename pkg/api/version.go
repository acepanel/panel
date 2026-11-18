package api

import (
	"slices"
	"time"

	"github.com/libtnb/utils/env"
)

type Version struct {
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Type        string            `json:"type"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Downloads   []VersionDownload `json:"downloads"`
}

type VersionDownload struct {
	URL      string `json:"url"`
	Arch     string `json:"arch"`
	Checksum string `json:"checksum"`
}

type Versions []Version

// LatestVersion 返回最新版本
func (r *API) LatestVersion(channel string) (*Version, error) {
	version, err := get[Version](r, "/version/latest", map[string]string{"channel": channel})
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
	return get[Versions](r, "/version/intermediate", map[string]string{"start": r.panelVersion, "channel": channel})
}

// Versions 返回指定类型的全部版本，beta 不含稳定版
func (r *API) Versions(channel string) (*Versions, error) {
	return get[Versions](r, "/versions", map[string]string{"channel": channel})
}
