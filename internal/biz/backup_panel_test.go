package biz

import (
	"errors"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/pkg/api"
	"github.com/acepanel/panel/v3/pkg/config"
)

type stubSettingRepo struct {
	SettingRepo
	values map[SettingKey]string
}

func (s stubSettingRepo) Get(key SettingKey, _ ...string) (string, error) {
	return s.values[key], nil
}

func TestPanelReleaseNewer(t *testing.T) {
	old := app.Version
	t.Cleanup(func() { app.Version = old })
	app.Version = "3.1.0"

	for _, tt := range []struct {
		latest string
		want   bool
	}{
		{"3.1.1", true},
		{"3.2.0-beta.1", true},
		{"3.1.0", false},
		{"3.0.9", false},
		{"3.1.0-rc.1", false},
	} {
		newer, err := (&PanelRelease{Version: tt.latest}).Newer()
		must.NoError(t, err)
		check.Equal(t, newer, tt.want, check.Msgf("latest=%s", tt.latest))
	}

	_, err := (&PanelRelease{Version: "not a version"}).Newer()
	check.Error(t, err)

	app.Version = "dev build"
	_, err = (&PanelRelease{Version: "3.1.1"}).Newer()
	check.Error(t, err)
}

func TestBackupUsecaseLatestPanel(t *testing.T) {
	cache := &stubCacheRepo{latest: &api.Version{
		Version:   "3.1.1",
		Downloads: []api.VersionDownload{{URL: "/panel/3.1.1/panel.zip", Checksum: "/panel/3.1.1/panel.zip.sha256"}},
	}}
	conf := &config.Config{}
	conf.App.DownloadEndpoint = "dl.example.com"
	uc := &BackupUsecase{
		cache:   cache,
		setting: stubSettingRepo{values: map[SettingKey]string{SettingKeyChannel: "beta"}},
		conf:    conf,
	}

	release, err := uc.LatestPanel()
	must.NoError(t, err)
	check.Equal(t, cache.channel, "beta")
	check.DeepEqual(t, release, &PanelRelease{
		Version:  "3.1.1",
		URL:      "https://dl.example.com/panel/3.1.1/panel.zip",
		Checksum: "https://dl.example.com/panel/3.1.1/panel.zip.sha256",
	})

	// 本机架构没有安装包
	cache.latest = &api.Version{Version: "3.1.2"}
	release, err = uc.LatestPanel()
	must.NoError(t, err)
	check.DeepEqual(t, release, &PanelRelease{Version: "3.1.2"})

	cache.err = errors.New("boom")
	_, err = uc.LatestPanel()
	check.Error(t, err)
}
