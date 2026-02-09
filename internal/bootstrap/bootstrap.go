package bootstrap

import "github.com/google/wire"

// ProviderSet is bootstrap providers.
var ProviderSet = wire.NewSet(NewConf, NewT, NewLog, NewCli, NewValidator, NewRouter, NewTLSReloader, NewHttp, NewHTTP3, NewDB, NewMigrate, NewLoader, NewSession, NewCron, NewQueue)
