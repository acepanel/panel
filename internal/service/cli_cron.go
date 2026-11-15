package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cast"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/shell"
)

func (s *CliService) CronList(ctx context.Context, cmd *cli.Command) error {
	crons, _, err := s.cronRepo.List(1, math.MaxUint32)
	if err != nil {
		return err
	}

	return s.printList(cmd, crons, func() {
		for _, cron := range crons {
			status := s.t.Get("stopped")
			if cron.Status {
				status = s.t.Get("running")
			}
			fmt.Println(s.t.Get("ID: %d, Name: %s, Type: %s, Schedule: %s, Status: %s", cron.ID, cron.Name, cron.Type, cron.Time, status))
		}
	})
}

// CronRun 立即执行一次计划任务，直接跑任务脚本，不经 wrapper 上报失败
func (s *CliService) CronRun(ctx context.Context, cmd *cli.Command) error {
	cron, err := s.cronRepo.Get(cmd.Uint("id"))
	if err != nil {
		return err
	}
	if !io.Exists(cron.Shell) {
		return errors.New(s.t.Get("Cron task script %s not exists", cron.Shell))
	}

	fmt.Println(s.t.Get("|-Running cron task: %s", cron.Name))
	out, err := shell.Execf(ctx, "bash %s", cron.Shell)
	if out != "" {
		fmt.Println(out)
	}
	if err != nil {
		return errors.New(s.t.Get("Cron task %s failed: %v", cron.Name, err))
	}

	fmt.Println(s.t.Get("Cron task %s executed successfully", cron.Name))
	return nil
}

func (s *CliService) CronStatus(ctx context.Context, cmd *cli.Command) error {
	status := !cmd.Bool("off")
	if err := s.cronRepo.Status(ctx, cmd.Uint("id"), status); err != nil {
		return err
	}

	msg := s.t.Get("Cron task %d disabled", cmd.Uint("id"))
	if status {
		msg = s.t.Get("Cron task %d enabled", cmd.Uint("id"))
	}

	fmt.Println(msg)
	return nil
}

// CronFailed 上报计划任务执行失败，由任务 wrapper 脚本调用
func (s *CliService) CronFailed(ctx context.Context, cmd *cli.Command) error {
	cron, err := s.cronRepo.Get(cmd.Uint("id"))
	if err != nil {
		return err
	}

	// 附带日志尾部，便于直接定位问题
	tail, _ := shell.Execf(ctx, "tail -n 20 %s", cron.Log)

	return s.notifyRepo.SendEventSync(ctx, biz.NotifyEventCronFailed, s.t.Get("[AcePanel] Cron Task Failed"),
		biz.NotifyBody(s.t.Get("cron task exited abnormally"), [][2]string{
			{s.t.Get("Task"), cron.Name},
			{s.t.Get("Schedule"), cron.Time},
			{s.t.Get("Exit Code"), cast.ToString(cmd.Int("code"))},
			{s.t.Get("Log"), cron.Log},
			{s.t.Get("Output"), tail},
			{s.t.Get("Time"), time.Now().Format(time.DateTime)},
		}))
}
