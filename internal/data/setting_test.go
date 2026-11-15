package data

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
)

// 列表类设置为空串时不应让整个面板设置读取失败
func TestSettingGetPanelEmptyList(t *testing.T) {
	repo := &settingRepo{db: newDBForTest(t), conf: &config.Config{}}
	must.NoError(t, repo.Set(biz.SettingKeyPublicIPs, ""))
	must.NoError(t, repo.Set(biz.SettingHiddenMenu, ""))

	panel, err := repo.GetPanel()
	must.NoError(t, err)
	check.DeepEqual(t, panel.PublicIP, []string{})
	check.DeepEqual(t, panel.HiddenMenu, []string{})
}
