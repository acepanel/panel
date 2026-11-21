package bootstrap

import (
	"context"
	"log/slog"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/cron"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/job"
)

func TestNewCronSkipsJobsWhileUpgrading(t *testing.T) {
	t.Cleanup(func() { app.Status.Store(app.StatusNormal) })

	var ran int
	c, err := NewCron(slog.New(slog.DiscardHandler), []job.Job{
		{Spec: "0 0 1 1 *", Task: cron.JobFunc(func(context.Context) error {
			ran++
			return nil
		})},
	})
	must.NoError(t, err)
	must.NoError(t, c.Start())
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	for _, status := range []int32{app.StatusUpgrade, app.StatusNormal} {
		app.Status.Store(status)
		for entry := range c.Entries() {
			must.NoError(t, c.TriggerAndWait(t.Context(), entry.ID))
		}
	}

	check.Equal(t, ran, 1)
}
