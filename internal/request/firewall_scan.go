package request

// FirewallScanSetting 扫描感知设置
type FirewallScanSetting struct {
	Enabled        bool     `json:"enabled"`
	Days           uint     `json:"days" validate:"min:1 && max:365"`
	Interfaces     []string `json:"interfaces" validate:"unique"`
	AutoBlock      bool     `json:"auto_block"`
	BlockThreshold uint     `json:"block_threshold" validate:"min:1 && max:100000"`
	BlockWindow    uint     `json:"block_window" validate:"min:1 && max:1440"`
	BlockDuration  uint     `json:"block_duration" validate:"max:87600"`
	BlockOutbound  bool     `json:"block_outbound"`
	Whitelist      []string `json:"whitelist" validate:"unique && dive && ipcidr"`
}

// FirewallScanUnblock 解封扫描感知自动屏蔽的 IP
type FirewallScanUnblock struct {
	IPs []string `json:"ips" validate:"required && dive && ip"`
}
