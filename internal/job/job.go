package job

import (
	"log/slog"

	"github.com/google/wire"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

var ProviderSet = wire.NewSet(NewJobs)

type Jobs struct {
	conf        *config.Config
	db          *gorm.DB
	log         *slog.Logger
	aggregator  *websitestat.Aggregator
	setting     biz.SettingRepo
	cert        biz.CertRepo
	certAccount biz.CertAccountRepo
	backup      biz.BackupRepo
	cache       biz.CacheRepo
	task        biz.TaskRepo
	scan        biz.ScanEventRepo
	stat        biz.WebsiteStatRepo
	website     biz.WebsiteRepo
}

func NewJobs(conf *config.Config, db *gorm.DB, log *slog.Logger, aggregator *websitestat.Aggregator, setting biz.SettingRepo, cert biz.CertRepo, certAccount biz.CertAccountRepo, backup biz.BackupRepo, cache biz.CacheRepo, task biz.TaskRepo, scan biz.ScanEventRepo, stat biz.WebsiteStatRepo, website biz.WebsiteRepo) *Jobs {
	return &Jobs{
		conf:        conf,
		db:          db,
		log:         log,
		aggregator:  aggregator,
		setting:     setting,
		cert:        cert,
		certAccount: certAccount,
		backup:      backup,
		cache:       cache,
		task:        task,
		scan:        scan,
		stat:        stat,
		website:     website,
	}
}

func (r *Jobs) Register(c *cron.Cron) error {
	if _, err := c.AddJob("* * * * *", NewMonitoring(r.db, r.log, r.setting)); err != nil {
		return err
	}
	if _, err := c.AddJob("*/2 * * * *", NewFirewallScan(r.log, r.setting, r.scan)); err != nil {
		return err
	}
	if _, err := c.AddJob("0 4 * * *", NewCertRenew(r.conf, r.db, r.log, r.setting, r.cert, r.certAccount)); err != nil {
		return err
	}
	if _, err := c.AddJob("0 2 * * *", NewPanelTask(r.conf, r.db, r.log, r.backup, r.cache, r.task, r.setting, r.scan, r.stat)); err != nil {
		return err
	}
	if _, err := c.AddJob("* * * * *", NewWebsiteStat(r.log, r.setting, r.stat, r.aggregator)); err != nil {
		return err
	}
	if _, err := c.AddJob("* * * * *", NewWebsiteExpire(r.db, r.log, r.website)); err != nil {
		return err
	}

	return nil
}
