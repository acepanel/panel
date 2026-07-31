package data

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAlertRepo, NewAppRepo, NewBackupRepo, NewBackupAccountRepo,
	NewCacheRepo, NewCertRepo, NewCertAccountRepo,
	NewCertDNSRepo, NewContainerRepo, NewContainerComposeRepo,
	NewContainerImageRepo, NewContainerNetworkRepo, NewContainerVolumeRepo,
	NewCronRepo, NewDatabaseRepo, NewDatabaseRedisRepo,
	NewDatabaseElasticsearchRepo, NewDatabaseServerRepo, NewDatabaseUserRepo,
	NewEnvironmentRepo, NewFileShareRepo, NewLogRepo, NewMonitorRepo,
	NewNotifyChannelRepo,
	NewProjectRepo, NewSafeRepo, NewScanEventRepo,
	NewSettingRepo, NewSSHRepo, NewTamperRepo, NewTaskRepo,
	NewTemplateRepo, NewUserRepo, NewUserPasskeyRepo,
	NewUserTokenRepo, NewWebHookRepo, NewWebsiteRepo,
	NewWebsiteStatRepo, NewWafRepo,
)
