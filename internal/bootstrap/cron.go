package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/libtnb/cron"
	"github.com/libtnb/cron/wrap"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/job"
)

func NewCron(log *slog.Logger, jobs []job.Job) (*cron.Cron, error) {
	// 面板任务均为 5 段表达式，不启用 WithSecondsField
	c, err := cron.New(
		cron.WithLogger(log),
		cron.WithChain(wrap.Recover(), wrap.SkipIfRunning()),
	)
	if err != nil {
		return nil, err
	}

	for _, j := range jobs {
		task := j.Task
		if !j.AnyStatus {
			task = normalOnly(task)
		}
		id, err := c.Add(j.Spec, task)
		if err != nil {
			return nil, err
		}
		if j.Immediate {
			// 调度器随面板启动(数据库迁移完成)后立即触发一次,未启动前重试等待
			go func(id cron.EntryID) {
				for {
					if err := c.Trigger(id); !errors.Is(err, cron.ErrSchedulerNotRunning) {
						return
					}
					time.Sleep(time.Second)
				}
			}(id)
		}
	}

	return c, nil
}

// normalOnly 面板处于维护或升级状态时跳过任务
func normalOnly(j cron.Job) cron.Job {
	return cron.JobFunc(func(ctx context.Context) error {
		if app.Status.Load() != app.StatusNormal {
			return nil
		}
		return j.Run(ctx)
	})
}
