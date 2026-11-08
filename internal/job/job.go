package job

import (
	"log/slog"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/cron"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/config"
	"github.com/acepanel/panel/v3/pkg/websitestat"
)

// Job 声明一个定时任务
type Job struct {
	Spec      string   // cron 表达式
	Task      cron.Job // 任务体
	Immediate bool     // 调度器启动后立即执行一次,不等首个调度点
}

// Dependencies 汇总定时任务依赖，Wire 会在生成期校验完整性。
type Dependencies struct {
	Alert       *biz.AlertUsecase
	Backup      *biz.BackupUsecase
	Cache       *biz.CacheUsecase
	Cert        *biz.CertUsecase
	CertAccount *biz.CertAccountUsecase
	FileShare   *biz.FileShareUsecase
	Monitor     *biz.MonitorUsecase
	Notify      *biz.NotifyUsecase
	ScanEvent   *biz.ScanEventUsecase
	Setting     *biz.SettingUsecase
	Tamper      *biz.TamperUsecase
	Task        *biz.TaskUsecase
	Website     *biz.WebsiteUsecase
	WebsiteStat *biz.WebsiteStatUsecase
	Conf        *config.Config
	DB          *gorm.DB
	T           *gotext.Locale
	Log         *slog.Logger
	Aggregator  *websitestat.Aggregator
}

func NewJobs(d *Dependencies) []Job {
	return []Job{
		NewAlert(d.Alert, d.T, d.Log),
		NewMonitoring(d.Setting, d.Monitor, d.T, d.Log),
		NewFirewallScan(d.ScanEvent, d.Setting, d.T, d.Log),
		NewCertRenew(d.CertAccount, d.Cert, d.Notify, d.Setting, d.Conf, d.DB, d.T, d.Log),
		NewFileShareClean(d.FileShare, d.T, d.Log),
		NewPanelTask(d.Backup, d.Cache, d.Monitor, d.ScanEvent, d.Setting, d.Tamper, d.Task, d.WebsiteStat, d.Conf, d.DB, d.T, d.Log),
		NewWebsiteStat(d.Setting, d.WebsiteStat, d.T, d.Log, d.Aggregator),
		NewWebsiteExpire(d.Notify, d.Website, d.DB, d.T, d.Log),
		NewTamper(d.Tamper, d.T, d.Log),
	}
}
