package nginx

import "time"

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

type StreamServer struct {
	Name                string        `form:"name" json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"` // 配置名称，用于文件命名
	Listen              string        `form:"listen" json:"listen" validate:"required"`                    // 监听地址，如: "12345", "0.0.0.0:12345", "[::]:12345"
	UDP                 bool          `form:"udp" json:"udp"`                                              // 是否 UDP 协议
	ProxyPass           string        `form:"proxy_pass" json:"proxy_pass" validate:"required"`            // 代理地址，如: "127.0.0.1:3306", "upstream_name"
	ProxyProtocol       bool          `form:"proxy_protocol" json:"proxy_protocol"`                        // 是否启用 PROXY 协议
	ProxyTimeout        time.Duration `form:"proxy_timeout" json:"proxy_timeout"`                          // 代理超时时间
	ProxyConnectTimeout time.Duration `form:"proxy_connect_timeout" json:"proxy_connect_timeout"`          // 代理连接超时时间
	SSL                 bool          `form:"ssl" json:"ssl"`                                              // 是否启用 SSL
	SSLCertificate      string        `form:"ssl_certificate" json:"ssl_certificate"`                      // SSL 证书路径
	SSLCertificateKey   string        `form:"ssl_certificate_key" json:"ssl_certificate_key"`              // SSL 私钥路径
}

type StreamUpstream struct {
	Name            string            `form:"name" json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"` // 上游名称
	Servers         map[string]string `form:"servers" json:"servers" validate:"required"`                  // 上游服务器及配置，如: map["127.0.0.1:3306"] = "weight=5"
	Algo            string            `form:"algo" json:"algo"`                                            // 负载均衡算法，如: "least_conn", "hash $remote_addr"
	Resolver        []string          `form:"resolver" json:"resolver"`                                    // DNS 解析器，如: ["8.8.8.8", "ipv6=off"]
	ResolverTimeout time.Duration     `form:"resolver_timeout" json:"resolver_timeout"`                    // DNS 解析超时时间
}
