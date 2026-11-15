package job

import (
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/app"
)

func TestPanelTaskMaintainRestoresStatusOnPanic(t *testing.T) {
	t.Cleanup(func() { app.Status.Store(app.StatusNormal) })

	// 未注入数据库，整理数据库时必然 panic
	task := new(PanelTask)
	must.Panics(t, func() { task.maintain(t.Context()) })
	check.Equal(t, app.Status.Load(), app.StatusNormal)
}

func TestPanelTaskMaintainSkipsWhileUpgrading(t *testing.T) {
	t.Cleanup(func() { app.Status.Store(app.StatusNormal) })
	app.Status.Store(app.StatusUpgrade)

	// 未注入数据库，真进了维护段必然 panic
	new(PanelTask).maintain(t.Context())
	check.Equal(t, app.Status.Load(), app.StatusUpgrade)
}
