//go:build wireinject

package data

import (
	"github.com/libtnb/wire"

	"github.com/acepanel/panel/v3/internal/biz"
)

// Module 装配数据访问层，对外导出 biz 中定义的仓储接口
var Module = wire.New().
	Provide(NewAlertRepo).
	Provide(NewAppRepo).
	Provide(NewBackupRepo).
	Provide(NewBackupAccountRepo).
	Provide(NewCacheRepo).
	Provide(NewCertRepo).
	Provide(NewCertAccountRepo).
	Provide(NewCertDNSRepo).
	Provide(NewContainerRepo).
	Provide(NewContainerComposeRepo).
	Provide(NewContainerImageRepo).
	Provide(NewContainerNetworkRepo).
	Provide(NewContainerVolumeRepo).
	Provide(NewCronRepo).
	Provide(NewDatabaseRepo).
	Provide(NewDatabaseRedisRepo).
	Provide(NewDatabaseElasticsearchRepo).
	Provide(NewDatabaseServerRepo).
	Provide(NewDatabaseUserRepo).
	Provide(NewEnvironmentRepo).
	Provide(NewFileShareRepo).
	Provide(NewFileFavoriteRepo).
	Provide(NewLogRepo).
	Provide(NewMonitorRepo).
	Provide(NewNotifyChannelRepo).
	Provide(NewProjectRepo).
	Provide(NewSafeRepo).
	Provide(NewScanEventRepo).
	Provide(NewSettingRepo).
	Provide(NewSSHRepo).
	Provide(NewTamperRepo).
	Provide(NewTaskRepo).
	Provide(NewTemplateRepo).
	Provide(NewUserRepo).
	Provide(NewUserPasskeyRepo).
	Provide(NewUserTokenRepo).
	Provide(NewWebHookRepo).
	Provide(NewWebsiteRepo).
	Provide(NewWebsiteStatRepo).
	Provide(NewMigrationSourceRepo).
	Provide(NewMigrationRemoteRepo).
	Provide(NewMigrationArchiveRepo).
	Export[biz.AlertRepo]().
	Export[biz.AppRepo]().
	Export[biz.BackupRepo]().
	Export[biz.BackupAccountRepo]().
	Export[biz.CacheRepo]().
	Export[biz.CertRepo]().
	Export[biz.CertAccountRepo]().
	Export[biz.CertDNSRepo]().
	Export[biz.ContainerRepo]().
	Export[biz.ContainerComposeRepo]().
	Export[biz.ContainerImageRepo]().
	Export[biz.ContainerNetworkRepo]().
	Export[biz.ContainerVolumeRepo]().
	Export[biz.CronRepo]().
	Export[biz.DatabaseRepo]().
	Export[biz.DatabaseRedisRepo]().
	Export[biz.DatabaseElasticsearchRepo]().
	Export[biz.DatabaseServerRepo]().
	Export[biz.DatabaseUserRepo]().
	Export[biz.EnvironmentRepo]().
	Export[biz.FileShareRepo]().
	Export[biz.FileFavoriteRepo]().
	Export[biz.LogRepo]().
	Export[biz.MonitorRepo]().
	Export[biz.NotifyChannelRepo]().
	Export[biz.ProjectRepo]().
	Export[biz.SafeRepo]().
	Export[biz.ScanEventRepo]().
	Export[biz.SettingRepo]().
	Export[biz.SSHRepo]().
	Export[biz.TamperRepo]().
	Export[biz.TaskRepo]().
	Export[biz.TemplateRepo]().
	Export[biz.UserRepo]().
	Export[biz.UserPasskeyRepo]().
	Export[biz.UserTokenRepo]().
	Export[biz.WebHookRepo]().
	Export[biz.WebsiteRepo]().
	Export[biz.WebsiteStatRepo]().
	Export[biz.MigrationSourceRepo]().
	Export[biz.MigrationRemoteRepo]().
	Export[biz.MigrationArchiveRepo]()
