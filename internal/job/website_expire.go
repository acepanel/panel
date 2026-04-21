package job

import (
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

// WebsiteExpire 网站到期自动关闭任务
type WebsiteExpire struct {
	db          *gorm.DB
	log         *slog.Logger
	websiteRepo biz.WebsiteRepo
}

// NewWebsiteExpire 创建网站到期检查任务
func NewWebsiteExpire(db *gorm.DB, log *slog.Logger, websiteRepo biz.WebsiteRepo) *WebsiteExpire {
	return &WebsiteExpire{
		db:          db,
		log:         log,
		websiteRepo: websiteRepo,
	}
}

func (r *WebsiteExpire) Run() {
	if app.Status != app.StatusNormal {
		return
	}

	var websites []biz.Website
	now := time.Now()
	// 直接查询已到期且仍在运行的网站
	if err := r.db.Where("expire_at IS NOT NULL AND expire_at <= ? AND status = ?", now, true).Find(&websites).Error; err != nil {
		r.log.Warn("查询到期网站失败", slog.Any("err", err))
		return
	}

	for _, website := range websites {
		if err := r.websiteRepo.UpdateStatus(website.ID, false); err != nil {
			r.log.Warn("关闭到期网站失败", slog.String("name", website.Name), slog.Any("err", err))
			continue
		}
		r.log.Info("网站已到期自动关闭", slog.String("name", website.Name), slog.Time("expire_at", *website.ExpireAt))
	}
}
