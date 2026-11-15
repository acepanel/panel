package docker

import (
	"encoding/json"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestApplySettingsKeepsUnmanagedKeys(t *testing.T) {
	config := make(map[string]any)
	must.NoError(t, json.Unmarshal([]byte(`{
		"default-ulimits": {"nofile": {"Name": "nofile", "Hard": 65535, "Soft": 65535}},
		"log-driver": "json-file",
		"log-opts": {"max-size": "10m", "max-file": "3", "compress": "true"},
		"exec-opts": ["native.cgroupdriver=cgroupfs", "native.umask=normal"],
		"dns": ["8.8.8.8"],
		"live-restore": true
	}`), &config))

	ipv6 := false
	new(App).applySettings(config, Settings{
		LogDriver:    "json-file",
		LogOpts:      LogOpts{MaxSize: "20m"},
		CgroupDriver: "systemd",
		DataRoot:     "/data/docker",
		IPv6:         &ipv6,
	})

	got, err := json.Marshal(config)
	must.NoError(t, err)
	// 面板不管的键与子项原样保留，未设置的项删掉，指针型开关显式写出 false
	check.Equal(t, string(got), `{"data-root":"/data/docker","default-ulimits":{"nofile":{"Hard":65535,"Name":"nofile","Soft":65535}},"exec-opts":["native.umask=normal","native.cgroupdriver=systemd"],"ipv6":false,"log-driver":"json-file","log-opts":{"compress":"true","max-size":"20m"}}`)
}

func TestApplySettingsDropsEmptyGroups(t *testing.T) {
	config := map[string]any{
		"log-opts":  map[string]any{"max-size": "10m"},
		"exec-opts": []any{"native.cgroupdriver=systemd"},
	}

	new(App).applySettings(config, Settings{})

	check.Empty(t, config)
}
