package rocketmq

// ConfigTune RocketMQ 配置调整
type ConfigTune struct {
	// Broker 基础
	BrokerName    string `form:"broker_name" json:"broker_name"`                                              // brokerName
	ListenPort    string `form:"listen_port" json:"listen_port" validate:"number && min:1 && max:65535"`      // listenPort
	NamesrvAddr   string `form:"namesrv_addr" json:"namesrv_addr"`                                            // namesrvAddr
	BrokerRole    string `form:"broker_role" json:"broker_role" validate:"in:ASYNC_MASTER,SYNC_MASTER,SLAVE"` // brokerRole
	FlushDiskType string `form:"flush_disk_type" json:"flush_disk_type" validate:"in:ASYNC_FLUSH,SYNC_FLUSH"` // flushDiskType
	// 存储
	StorePathRootDir   string `form:"store_path_root_dir" json:"store_path_root_dir" validate:"unix_path"`     // storePathRootDir
	StorePathCommitLog string `form:"store_path_commit_log" json:"store_path_commit_log" validate:"unix_path"` // storePathCommitLog
	MaxMessageSize     string `form:"max_message_size" json:"max_message_size" validate:"number"`              // maxMessageSize
	// JVM - NameServer
	NamesrvHeapInitSize string `form:"namesrv_heap_init_size" json:"namesrv_heap_init_size"` // -Xms (namesrv)
	NamesrvHeapMaxSize  string `form:"namesrv_heap_max_size" json:"namesrv_heap_max_size"`   // -Xmx (namesrv)
	// JVM - Broker
	BrokerHeapInitSize string `form:"broker_heap_init_size" json:"broker_heap_init_size"` // -Xms (broker)
	BrokerHeapMaxSize  string `form:"broker_heap_max_size" json:"broker_heap_max_size"`   // -Xmx (broker)
}
