package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAlertService, NewAppService, NewBackupService, NewBackupStorageService,
	NewCertService, NewCertAccountService, NewCertDNSService,
	NewCliService, NewContainerService, NewContainerComposeService,
	NewContainerImageService, NewContainerNetworkService, NewContainerVolumeService,
	NewCronService, NewDatabaseService, NewDatabaseRedisService,
	NewDatabaseElasticsearchService, NewDatabaseServerService, NewDatabaseUserService,
	NewEnvironmentService, NewEnvironmentGoService, NewEnvironmentJavaService,
	NewEnvironmentNodejsService, NewEnvironmentPHPService, NewEnvironmentPythonService,
	NewEnvironmentDotnetService, NewFileService, NewFileShareService, NewFirewallService,
	NewFirewallScanService, NewHomeService, NewLogService,
	NewMonitorService, NewNotifyService, NewProcessService, NewProjectService,
	NewSafeService, NewSettingService, NewSSHService,
	NewSystemctlService, NewTamperService, NewTaskService, NewTemplateService,
	NewUserService, NewUserPasskeyService, NewUserTokenService,
	NewWebHookService, NewWebsiteService, NewWebsiteStatService,
	NewToolboxNetworkService, NewToolboxSystemService, NewToolboxBenchmarkService,
	NewToolboxSSHService, NewToolboxDiskService, NewToolboxLogService,
	NewToolboxMigrationService, NewWsService,
	NewWafService,
)
