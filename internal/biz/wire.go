//go:build wireinject

package biz

import "github.com/libtnb/wire"

// Module 装配业务逻辑层
var Module = wire.New().
	Provide(NewAlertUsecase).
	Provide(NewAppUsecase).
	Provide(NewBackupUsecase).
	Provide(NewBackupAccountUsecase).
	Provide(NewCacheUsecase).
	Provide(NewCertUsecase).
	Provide(NewCertAccountUsecase).
	Provide(NewCertDNSUsecase).
	Provide(NewContainerUsecase).
	Provide(NewContainerComposeUsecase).
	Provide(NewContainerImageUsecase).
	Provide(NewContainerNetworkUsecase).
	Provide(NewContainerVolumeUsecase).
	Provide(NewCronUsecase).
	Provide(NewDatabaseUsecase).
	Provide(NewDatabaseRedisUsecase).
	Provide(NewDatabaseElasticsearchUsecase).
	Provide(NewDatabaseServerUsecase).
	Provide(NewDatabaseUserUsecase).
	Provide(NewEnvironmentUsecase).
	Provide(NewFileShareUsecase).
	Provide(NewFileFavoriteUsecase).
	Provide(NewLogUsecase).
	Provide(NewMonitorUsecase).
	Provide(NewNotifyUsecase).
	Provide(NewProjectUsecase).
	Provide(NewSafeUsecase).
	Provide(NewScanEventUsecase).
	Provide(NewSettingUsecase).
	Provide(NewSSHUsecase).
	Provide(NewTamperUsecase).
	Provide(NewTaskUsecase).
	Provide(NewTemplateUsecase).
	Provide(NewUserUsecase).
	Provide(NewUserPasskeyUsecase).
	Provide(NewUserTokenUsecase).
	Provide(NewWebHookUsecase).
	Provide(NewWebsiteUsecase).
	Provide(NewWebsiteStatUsecase).
	Provide(NewToolboxMigrationUsecase).
	Export[*AlertUsecase]().
	Export[*AppUsecase]().
	Export[*BackupUsecase]().
	Export[*BackupAccountUsecase]().
	Export[*CacheUsecase]().
	Export[*CertUsecase]().
	Export[*CertAccountUsecase]().
	Export[*CertDNSUsecase]().
	Export[*ContainerUsecase]().
	Export[*ContainerComposeUsecase]().
	Export[*ContainerImageUsecase]().
	Export[*ContainerNetworkUsecase]().
	Export[*ContainerVolumeUsecase]().
	Export[*CronUsecase]().
	Export[*DatabaseUsecase]().
	Export[*DatabaseRedisUsecase]().
	Export[*DatabaseElasticsearchUsecase]().
	Export[*DatabaseServerUsecase]().
	Export[*DatabaseUserUsecase]().
	Export[*EnvironmentUsecase]().
	Export[*FileShareUsecase]().
	Export[*FileFavoriteUsecase]().
	Export[*LogUsecase]().
	Export[*MonitorUsecase]().
	Export[*NotifyUsecase]().
	Export[*ProjectUsecase]().
	Export[*SafeUsecase]().
	Export[*ScanEventUsecase]().
	Export[*SettingUsecase]().
	Export[*SSHUsecase]().
	Export[*TamperUsecase]().
	Export[*TaskUsecase]().
	Export[*TemplateUsecase]().
	Export[*UserUsecase]().
	Export[*UserPasskeyUsecase]().
	Export[*UserTokenUsecase]().
	Export[*WebHookUsecase]().
	Export[*WebsiteUsecase]().
	Export[*WebsiteStatUsecase]().
	Export[*ToolboxMigrationUsecase]()
