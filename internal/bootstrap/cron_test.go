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

func TestNewCronSkipsJobsUnlessNormal(t *testing.T) {
	t.Cleanup(func() { app.Status.Store(app.StatusNormal) })

	ran := make(map[string]int)
	record := func(name string) cron.Job {
		return cron.JobFunc(func(context.Context) error {
			ran[name]++
			return nil
		})
	}

	c, err := NewCron(slog.New(slog.DiscardHandler), []job.Job{
		{Spec: "0 0 1 1 *", Task: record("normal")},
		{Spec: "0 0 1 1 *", Task: record("any"), AnyStatus: true},
	})
	must.NoError(t, err)
	must.NoError(t, c.Start())
	t.Cleanup(func() { _ = c.Stop(context.Background()) })

	for _, status := range []int32{app.StatusMaintain, app.StatusUpgrade, app.StatusNormal} {
		app.Status.Store(status)
		for entry := range c.Entries() {
			must.NoError(t, c.TriggerAndWait(t.Context(), entry.ID))
		}
	}

	check.DeepEqual(t, ran, map[string]int{"normal": 1, "any": 3})
}
