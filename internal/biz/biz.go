package biz

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAlertUsecase, NewAppUsecase, NewBackupUsecase, NewBackupAccountUsecase,
	NewCacheUsecase, NewCertUsecase, NewCertAccountUsecase,
	NewCertDNSUsecase, NewContainerUsecase, NewContainerComposeUsecase,
	NewContainerImageUsecase, NewContainerNetworkUsecase, NewContainerVolumeUsecase,
	NewCronUsecase, NewDatabaseUsecase, NewDatabaseRedisUsecase,
	NewDatabaseElasticsearchUsecase, NewDatabaseServerUsecase, NewDatabaseUserUsecase,
	NewEnvironmentUsecase, NewFileShareUsecase, NewLogUsecase, NewMonitorUsecase,
	NewNotifyUsecase, NewProjectUsecase, NewSafeUsecase, NewScanEventUsecase,
	NewSettingUsecase, NewSSHUsecase, NewTamperUsecase, NewTaskUsecase,
	NewTemplateUsecase, NewUserUsecase, NewUserPasskeyUsecase,
	NewUserTokenUsecase, NewWebHookUsecase, NewWebsiteUsecase,
	NewWebsiteStatUsecase, NewWafUsecase,
)
