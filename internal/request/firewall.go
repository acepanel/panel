package request

type FirewallStatus struct {
	Status bool `json:"status" form:"status"`
}

type FirewallRule struct {
	Type      string `json:"type" validate:"in:rich,normal"`
	Family    string `json:"family" validate:"required && in:ipv4,ipv6"`
	PortStart uint   `json:"port_start" validate:"required && min:1 && max:65535"`
	PortEnd   uint   `json:"port_end" validate:"required && min:1 && max:65535 && gtefield:PortStart"`
	Protocol  string `json:"protocol" validate:"required && in:tcp,udp,tcp/udp"`
	Address   string `json:"address" validate:"ipcidr"`
	Strategy  string `json:"strategy" validate:"required && in:accept,drop,reject"`
	Direction string `json:"direction" validate:"required && in:in,out"`
}

type FirewallRuleUpdate struct {
	FirewallRule
	NewStrategy string `json:"new_strategy" validate:"required && in:accept,drop,reject"`
}

type FirewallIPRule struct {
	Family    string `json:"family" validate:"required && in:ipv4,ipv6"`
	Protocol  string `json:"protocol" validate:"required && in:tcp,udp,tcp/udp"`
	Address   string `json:"address" validate:"ipcidr"`
	Strategy  string `json:"strategy" validate:"required && in:accept,drop,reject"`
	Direction string `json:"direction" validate:"required && in:in,out"`
}

type FirewallForward struct {
	Protocol   string `json:"protocol" validate:"required && in:tcp,udp,tcp/udp"`
	Port       uint   `json:"port" validate:"required && min:1 && max:65535"`
	TargetIP   string `json:"target_ip" validate:"required && ip"`
	TargetPort uint   `json:"target_port" validate:"required && min:1 && max:65535"`
}

type FirewallSubscriptionCreate struct {
	Name     string `json:"name" validate:"required"`
	URL      string `json:"url" validate:"required && url"`
	Type     string `json:"type" validate:"required && in:block,allow"`
	Schedule string `json:"schedule" validate:"required && cron"`
	Status   bool   `json:"status"`
}

type FirewallSubscriptionUpdate struct {
	ID uint `json:"id" uri:"id" validate:"required && exists:firewall_subscriptions,id"`
	FirewallSubscriptionCreate
}
