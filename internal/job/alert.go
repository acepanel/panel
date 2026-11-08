package job

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

// Alert 告警规则评估任务
type Alert struct {
	log       *slog.Logger
	alertRepo *biz.AlertUsecase
	t         *gotext.Locale
}

// NewAlert 构造告警评估任务
func NewAlert(alertUsecase *biz.AlertUsecase, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "* * * * *",
		Task: &Alert{
			log:       log,
			alertRepo: alertUsecase,
			t:         t,
		},
	}
}

func (r *Alert) Run(ctx context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	if err := r.alertRepo.Evaluate(ctx); err != nil {
		r.log.Warn(r.t.Get("failed to evaluate alert rules"), slog.Any("err", err))
	}

	return nil
}
