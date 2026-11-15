package docker

// DaemonConfig daemon.json 中由面板管理的配置项
type DaemonConfig struct {
	RegistryMirrors    []string          `json:"registry-mirrors,omitempty"`
	InsecureRegistries []string          `json:"insecure-registries,omitempty"`
	LiveRestore        bool              `json:"live-restore,omitempty"`
	LogDriver          string            `json:"log-driver,omitempty"`
	LogOpts            map[string]string `json:"log-opts,omitempty"`
	ExecOpts           []string          `json:"exec-opts,omitempty"`
	Hosts              []string          `json:"hosts,omitempty"`
	DataRoot           string            `json:"data-root,omitempty"`
	StorageDriver      string            `json:"storage-driver,omitempty"`
	DNS                []string          `json:"dns,omitempty"`
	FirewallBackend    string            `json:"firewall-backend,omitempty"`
	Iptables           *bool             `json:"iptables,omitempty"`
	Ip6tables          *bool             `json:"ip6tables,omitempty"`
	IpForward          *bool             `json:"ip-forward,omitempty"`
	IPv6               *bool             `json:"ipv6,omitempty"`
	Bip                string            `json:"bip,omitempty"`
}
