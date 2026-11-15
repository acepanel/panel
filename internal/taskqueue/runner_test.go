package taskqueue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

func newRunnerForTest(t *testing.T) *Runner {
	t.Helper()
	root := app.Root
	app.Root = t.TempDir()
	t.Cleanup(func() { app.Root = root })

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatal(err)
	}
	// file::memory: 每连接独立库,限单连接以让 runner goroutine 与主 goroutine 共享数据
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err = db.AutoMigrate(&biz.Task{}); err != nil {
		t.Fatal(err)
	}
	return NewRunner(db, slog.New(slog.NewTextHandler(os.Stderr, nil)), stubNotifier{}, gotext.NewLocale("", "en"))
}

type stubNotifier struct{}

func (stubNotifier) SendEvent(biz.NotifyEvent, string, string) {}

// 等待任务进入指定状态
func waitStatus(t *testing.T, db *gorm.DB, id uint, status biz.TaskStatus) *biz.Task {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	task := new(biz.Task)
	for time.Now().Before(deadline) {
		if err := db.First(task, id).Error; err == nil && task.Status == status {
			return task
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %d did not reach status %s, current: %s", id, status, task.Status)
	return nil
}

func TestRunnerCancelRunning(t *testing.T) {
	r := newRunnerForTest(t)
	marker := filepath.Join(t.TempDir(), "cleanup.done")

	task := &biz.Task{
		Name:        "sleep",
		Status:      biz.TaskStatusWaiting,
		Shell:       "sleep 60",
		CancelShell: "touch " + marker,
	}
	if err := r.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	r.Run(t.Context())

	// 等待任务进入运行状态后取消
	waitStatus(t, r.db, task.ID, biz.TaskStatusRunning)
	if !r.Cancel(task.ID) {
		t.Fatal("Cancel should hit the running task")
	}

	// 清理命令在写 canceled 之前同步跑完，状态到位即可断言产物
	got := waitStatus(t, r.db, task.ID, biz.TaskStatusCanceled)
	if got.Log == "" {
		t.Fatal("task log path should be set")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("cancel shell was not executed")
	}
}

// 模拟 systemd 停机：命令被 SIGTERM 杀死，运行器 ctx 几乎同时取消，应记为 canceled 并跑清理
func TestRunnerShutdownCanceled(t *testing.T) {
	r := newRunnerForTest(t)
	marker := filepath.Join(t.TempDir(), "cleanup.done")

	task := &biz.Task{
		Name:        "shutdown",
		Status:      biz.TaskStatusWaiting,
		Shell:       "kill -TERM $$",
		CancelShell: "touch " + marker,
	}
	if err := r.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	r.Run(ctx)

	waitStatus(t, r.db, task.ID, biz.TaskStatusRunning)
	cancel()

	waitStatus(t, r.db, task.ID, biz.TaskStatusCanceled)
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("cancel shell was not executed")
	}
}

// 没有停机信号跟进时，被信号杀死的命令仍是失败
func TestRunnerSignaledFailed(t *testing.T) {
	r := newRunnerForTest(t)

	task := &biz.Task{Name: "signaled", Status: biz.TaskStatusWaiting, Shell: "kill -TERM $$"}
	if err := r.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	r.Run(t.Context())

	waitStatus(t, r.db, task.ID, biz.TaskStatusFailed)
}

func TestRunnerCancelMiss(t *testing.T) {
	r := newRunnerForTest(t)
	if r.Cancel(1) {
		t.Fatal("Cancel should miss when nothing is running")
	}
}

func TestRunnerWaitingCanceledNotExecuted(t *testing.T) {
	r := newRunnerForTest(t)

	// 已被标记取消的任务不应被运行器抢占执行
	task := &biz.Task{Name: "noop", Status: biz.TaskStatusCanceled, Shell: "true"}
	if err := r.db.Create(task).Error; err != nil {
		t.Fatal(err)
	}

	r.Run(t.Context())

	time.Sleep(200 * time.Millisecond)
	got := new(biz.Task)
	if err := r.db.First(got, task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != biz.TaskStatusCanceled {
		t.Fatalf("canceled task should stay canceled, got %s", got.Status)
	}
}

// 抢占写库失败时任务应原样留在 waiting 并让出本轮，不能停在 running，也不能原地重取
func TestRunnerPreemptFailedLeavesWaiting(t *testing.T) {
	r := newRunnerForTest(t)

	task := &biz.Task{Name: "noop", Status: biz.TaskStatusWaiting, Shell: "true"}
	must.NoError(t, r.db.Create(task).Error)

	// 只让带日志路径的写入失败：状态与日志路径分两次写时，状态会先一步变成 running
	failing := true
	must.NoError(t, r.db.Callback().Update().Before("gorm:update").Register("test:fail_log", func(tx *gorm.DB) {
		if dest, ok := tx.Statement.Dest.(map[string]any); ok && failing {
			if _, ok = dest["log"]; ok {
				_ = tx.AddError(errors.New("disk full"))
			}
		}
	}))

	must.False(t, r.processNext(t.Context()))
	got := new(biz.Task)
	must.NoError(t, r.db.First(got, task.ID).Error)
	check.Equal(t, got.Status, biz.TaskStatusWaiting)
	check.Empty(t, got.Log)

	// 写库恢复后同一条任务照常执行
	failing = false
	must.True(t, r.processNext(t.Context()))
	must.NoError(t, r.db.First(got, task.ID).Error)
	check.Equal(t, got.Status, biz.TaskStatusSuccess)
	check.NotEmpty(t, got.Log)
}

// 取出后、抢占前被取消的任务跳过即可，队列继续往下取
func TestRunnerPreemptMissContinues(t *testing.T) {
	r := newRunnerForTest(t)

	task := &biz.Task{Name: "noop", Status: biz.TaskStatusWaiting, Shell: "true"}
	must.NoError(t, r.db.Create(task).Error)
	must.NoError(t, r.db.Model(&biz.Task{}).Where("id = ?", task.ID).Update("status", biz.TaskStatusCanceled).Error)

	must.True(t, r.execute(t.Context(), task))
	got := new(biz.Task)
	must.NoError(t, r.db.First(got, task.ID).Error)
	check.Equal(t, got.Status, biz.TaskStatusCanceled)
	check.Empty(t, got.Log)
}
