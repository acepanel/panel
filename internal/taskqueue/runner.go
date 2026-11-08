package taskqueue

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/shell"
)

// Notifier 事件通知，由 biz.NotifyUsecase 实现
type Notifier interface {
	SendEvent(event biz.NotifyEvent, subject, body string)
}

type Runner struct {
	db       *gorm.DB
	log      *slog.Logger
	notifier Notifier
	t        *gotext.Locale
	notify   chan struct{}

	wg sync.WaitGroup // 供 Wait 等待运行协程收尾

	mu            sync.Mutex
	currentID     uint               // 当前运行的任务 ID
	currentCancel context.CancelFunc // 取消当前任务
}

// NewRunner 创建任务运行器
func NewRunner(db *gorm.DB, log *slog.Logger, notifier Notifier, t *gotext.Locale) *Runner {
	return &Runner{
		db:       db,
		log:      log,
		notifier: notifier,
		t:        t,
		notify:   make(chan struct{}, 1),
	}
}

// Notify 非阻塞通知运行器有新任务，供 Push 调用
func (r *Runner) Notify() {
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

// Cancel 取消正在运行的任务，返回是否命中
func (r *Runner) Cancel(id uint) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.currentID != id || r.currentCancel == nil {
		return false
	}
	r.currentCancel()
	return true
}

// Run 启动运行器
func (r *Runner) Run(ctx context.Context) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.clearZombie()

		// 启动时先尝试处理积压的 waiting 任务
		r.Notify()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.notify:
				r.drain(ctx)
			case <-ticker.C:
				r.drain(ctx)
			}
		}
	}()
}

// Wait 等待运行协程收尾，ctx 到期则放弃等待
// 停机时正在跑的任务要写状态和跑清理命令，不等就会和进程退出赛跑
func (r *Runner) Wait(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}

// drain 持续处理 waiting 任务直到队列为空或 ctx 取消
func (r *Runner) drain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !r.processNext(ctx) {
			return
		}
	}
}

// clearZombie 启动时将残留的 running 任务标记为 failed
func (r *Runner) clearZombie() {
	if err := r.db.Model(&biz.Task{}).Where("status = ?", biz.TaskStatusRunning).Update("status", biz.TaskStatusFailed).Error; err != nil {
		r.log.Error(r.t.Get("failed to clear zombie tasks"), slog.Any("err", err))
	}
}

// processNext 取一条 waiting 任务执行，返回是否有任务被处理
func (r *Runner) processNext(ctx context.Context) bool {
	task := new(biz.Task)
	if err := r.db.Where("status = ?", biz.TaskStatusWaiting).Order("id asc").First(task).Error; err != nil {
		return false
	}

	r.execute(ctx, task)
	return true
}

// execute 执行单个任务
func (r *Runner) execute(ctx context.Context, task *biz.Task) {
	// 先登记再抢占：状态一旦变成 running，Cancel 就必须能命中，
	// 反过来会让刚开始跑的任务取消落空，取出后、抢占前的取消也会被忽略
	taskCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	r.currentID, r.currentCancel = task.ID, cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.currentID, r.currentCancel = 0, nil
		r.mu.Unlock()
		cancel()
	}()

	// 原子抢占，任务可能在取出后被取消
	result := r.db.Model(task).Where("status = ?", biz.TaskStatusWaiting).Update("status", biz.TaskStatusRunning)
	if result.Error != nil {
		r.log.Error(r.t.Get("failed to update task status to running"), slog.Any("task_id", task.ID), slog.Any("err", result.Error))
		return
	}
	if result.RowsAffected == 0 {
		return
	}

	// 计算日志路径并保存
	logDir := filepath.Join(app.Root, "panel/storage/logs/task")
	_ = os.MkdirAll(logDir, 0o700)
	logFile := filepath.Join(logDir, fmt.Sprintf("%d.log", task.ID))
	if err := r.db.Model(task).Update("log", logFile).Error; err != nil {
		r.log.Error(r.t.Get("failed to update task log path"), slog.Any("task_id", task.ID), slog.Any("err", err))
		return
	}

	if err := shell.ExecWithLog(taskCtx, task.Shell, logFile); err != nil {
		// 用户取消和面板停机都不是任务本身失败，记为 canceled 并跑清理命令；
		// 停机时 systemd 向整个 cgroup 同时发 SIGTERM，命令可能比 ctx 取消早一步死，
		// 给一个远大于这点调度差的宽限期，否则会被记成失败并发假告警
		status := biz.TaskStatusFailed
		select {
		case <-taskCtx.Done():
			status = biz.TaskStatusCanceled
			r.runCancelShell(ctx, task, logFile)
		case <-time.After(200 * time.Millisecond):
		}
		r.log.Warn(r.t.Get("background task did not finish"), slog.Any("task_id", task.ID), slog.Any("status", status), slog.Any("err", err))
		_ = r.db.Model(task).Update("status", status).Error

		// 用户主动取消不算故障
		if status == biz.TaskStatusFailed {
			r.notifier.SendEvent(biz.NotifyEventTaskFailed, r.t.Get("[AcePanel] Background Task Failed"), biz.NotifyBody(r.t.Get("background task failed"), [][2]string{
				{r.t.Get("Task"), task.Name},
				{r.t.Get("Log"), logFile},
				{r.t.Get("Error"), err.Error()},
				{r.t.Get("Time"), time.Now().Format(time.DateTime)},
			}))
		}
		return
	}

	if err := r.db.Model(task).Update("status", biz.TaskStatusSuccess).Error; err != nil {
		r.log.Error(r.t.Get("failed to update task status to success"), slog.Any("task_id", task.ID), slog.Any("err", err))
	}
}

// runCancelShell 任务被取消后执行清理命令，输出追加到任务日志
func (r *Runner) runCancelShell(ctx context.Context, task *biz.Task, logFile string) {
	if task.CancelShell == "" {
		return
	}

	// 超时不能超过关停总预算，否则停机时清理会跑到一半随进程退出被砍
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()

	if err := shell.ExecWithLogAppend(cleanupCtx, task.CancelShell, logFile); err != nil {
		r.log.Warn(r.t.Get("failed to run task cancel shell"), slog.Any("task_id", task.ID), slog.Any("err", err))
	}
}
