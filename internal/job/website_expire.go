package job

import (
	"context"
	"log/slog"
	"time"

	"github.com/leonelquinteros/gotext"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

// WebsiteExpire 网站到期自动关闭任务
type WebsiteExpire struct {
	db          *gorm.DB
	log         *slog.Logger
	websiteRepo *biz.WebsiteUsecase
	notifyRepo  *biz.NotifyUsecase
	t           *gotext.Locale
}

// NewWebsiteExpire 构造网站到期检查任务
func NewWebsiteExpire(notifyUsecase *biz.NotifyUsecase, websiteUsecase *biz.WebsiteUsecase, db *gorm.DB, t *gotext.Locale, log *slog.Logger) Job {
	return Job{
		Spec: "* * * * *",
		Task: &WebsiteExpire{
			db:          db,
			log:         log,
			websiteRepo: websiteUsecase,
			notifyRepo:  notifyUsecase,
			t:           t,
		},
	}
}

func (r *WebsiteExpire) Run(ctx context.Context) error {
	if app.Status != app.StatusNormal {
		return nil
	}

	var websites []biz.Website
	now := time.Now()
	// 直接查询已到期且仍在运行的网站
	if err := r.db.Where("expire_at IS NOT NULL AND expire_at <= ? AND status = ?", now, true).Find(&websites).Error; err != nil {
		r.log.Warn(r.t.Get("failed to query expired websites"), slog.Any("err", err))
		return nil
	}

	for _, website := range websites {
		if err := r.websiteRepo.UpdateStatus(ctx, website.ID, false); err != nil {
			r.log.Warn(r.t.Get("failed to disable expired website"), slog.String("name", website.Name), slog.Any("err", err))
			continue
		}
		r.notifyRepo.SendEvent(biz.NotifyEventWebsiteExpire, r.t.Get("[AcePanel] Website Expired"), biz.NotifyBody(r.t.Get("website expired and has been disabled"), [][2]string{
			{r.t.Get("Website"), website.Name},
			{r.t.Get("Expire Time"), website.ExpireAt.Format(time.DateTime)},
		}))
	}
	return nil
}
