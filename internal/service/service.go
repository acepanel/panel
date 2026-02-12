package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewAppService,
	NewBackupService,
	NewCertService,
	NewCertAccountService,
	NewCertDNSService,
	NewCliService,
	NewContainerService,
	NewContainerComposeService,
	NewContainerImageService,
	NewContainerNetworkService,
	NewContainerVolumeService,
	NewCronService,
	NewDashboardService,
	NewDatabaseService,
	NewDatabaseServerService,
	NewDatabaseUserService,
	NewFileService,
	NewFirewallService,
	NewMonitorService,
	NewProcessService,
	NewSafeService,
	NewSettingService,
	NewSSHService,
	NewSystemctlService,
	NewTaskService,
	NewUserService,
	NewUserTokenService,
	NewWebsiteService,
	NewToolboxSystemService,
	NewToolboxBenchmarkService,
	NewToolboxMigrationService,
	NewWsService,
)
