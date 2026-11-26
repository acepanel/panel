package job

import (
	"context"
	"log/slog"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
)

// UserSessionClean 过期登录会话清理任务
type UserSessionClean struct {
	log             *slog.Logger
	userSessionRepo *biz.UserSessionUsecase
	t               *gotext.Locale
}

// NewUserSessionClean 构造过期登录会话清理任务
func NewUserSessionClean(userSessionUsecase *biz.UserSessionUsecase, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "0 * * * *",
		Task: &UserSessionClean{
			log:             log,
			userSessionRepo: userSessionUsecase,
			t:               t,
		},
	}
}

func (r *UserSessionClean) Run(_ context.Context) error {
	if _, err := r.userSessionRepo.ClearExpired(); err != nil {
		r.log.Warn(r.t.Get("failed to clear expired user sessions"), slog.Any("err", err))
	}
	return nil
}
