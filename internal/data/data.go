package data

import "github.com/google/wire"

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewAppRepo,
	NewBackupRepo,
	NewCacheRepo,
	NewCertRepo,
	NewCertAccountRepo,
	NewCertDNSRepo,
	NewContainerRepo,
	NewContainerComposeRepo,
	NewContainerImageRepo,
	NewContainerNetworkRepo,
	NewContainerVolumeRepo,
	NewCronRepo,
	NewDatabaseRepo,
	NewDatabaseServerRepo,
	NewDatabaseUserRepo,
	NewEnvironmentRepo,
	NewLogRepo,
	NewMonitorRepo,
	NewProjectRepo,
	NewSafeRepo,
	NewSettingRepo,
	NewSSHRepo,
	NewTaskRepo,
	NewUserRepo,
	NewUserTokenRepo,
	NewWebHookRepo,
	NewWebsiteRepo,
)
