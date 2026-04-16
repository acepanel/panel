package job

import (
	"log/slog"
	"time"

	"github.com/acepanel/panel/v3/internal/app"
	"github.com/acepanel/panel/v3/internal/biz"
)

// WebsiteExpire 网站到期自动关闭任务
type WebsiteExpire struct {
	log         *slog.Logger
	websiteRepo biz.WebsiteRepo
}

// NewWebsiteExpire 创建网站到期检查任务
func NewWebsiteExpire(log *slog.Logger, websiteRepo biz.WebsiteRepo) *WebsiteExpire {
	return &WebsiteExpire{
		log:         log,
		websiteRepo: websiteRepo,
	}
}

func (r *WebsiteExpire) Run() {
	if app.Status != app.StatusNormal {
		return
	}

	websites, _, err := r.websiteRepo.List("all", 1, 10000)
	if err != nil {
		r.log.Warn("获取网站列表失败", slog.Any("err", err))
		return
	}

	now := time.Now()
	for _, website := range websites {
		if website.ExpireAt == nil || !website.Status {
			continue
		}
		if now.After(*website.ExpireAt) {
			if err = r.websiteRepo.UpdateStatus(website.ID, false); err != nil {
				r.log.Warn("关闭到期网站失败", slog.String("name", website.Name), slog.Any("err", err))
				continue
			}
			r.log.Info("网站已到期自动关闭", slog.String("name", website.Name), slog.Time("expire_at", *website.ExpireAt))
		}
	}
}
