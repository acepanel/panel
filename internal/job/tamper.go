package job

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

// Tamper 防篡改后台任务:落库拦截日志、清理过期、面板重启后恢复保护
type Tamper struct {
	log        *slog.Logger
	tamperRepo *biz.TamperUsecase
	t          *gotext.Locale
	reconciled bool
}

// NewTamper 构造防篡改任务
func NewTamper(tamperUsecase *biz.TamperUsecase, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "* * * * *",
		// 启动后立即恢复保护,不给防篡改留一分钟的空窗
		Immediate: true,
		Task: &Tamper{
			log:        log,
			tamperRepo: tamperUsecase,
			t:          t,
		},
	}
}

func (r *Tamper) Run(_ context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	// 首次运行按持久化设置恢复保护(面板重启后 eBPF 程序需重新挂载)
	if !r.reconciled {
		if err := r.tamperRepo.Reconcile(); err != nil {
			r.log.Warn(r.t.Get("failed to reconcile tamper protection"), slog.Any("err", err))
		}
		r.reconciled = true
	}

	// 落库拦截日志并清理过期
	r.tamperRepo.FlushLogs()
	r.tamperRepo.CleanupLogs()
	return nil
}
