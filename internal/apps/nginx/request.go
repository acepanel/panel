package nginx

import "time"

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

// ConfigTune Nginx 配置调整
type ConfigTune struct {
	// 常规设置
	WorkerProcesses          string `form:"worker_processes" json:"worker_processes"`
	WorkerConnections        string `form:"worker_connections" json:"worker_connections"`
	KeepaliveTimeout         string `form:"keepalive_timeout" json:"keepalive_timeout"`
	ClientMaxBodySize        string `form:"client_max_body_size" json:"client_max_body_size"`
	ClientBodyBufferSize     string `form:"client_body_buffer_size" json:"client_body_buffer_size"`
	ClientHeaderBufferSize   string `form:"client_header_buffer_size" json:"client_header_buffer_size"`
	ServerNamesHashBucketSize string `form:"server_names_hash_bucket_size" json:"server_names_hash_bucket_size"`
	ServerTokens             string `form:"server_tokens" json:"server_tokens"`
	// Gzip 压缩
	Gzip          string `form:"gzip" json:"gzip"`
	GzipMinLength string `form:"gzip_min_length" json:"gzip_min_length"`
	GzipCompLevel string `form:"gzip_comp_level" json:"gzip_comp_level"`
	GzipTypes     string `form:"gzip_types" json:"gzip_types"`
	GzipVary      string `form:"gzip_vary" json:"gzip_vary"`
	GzipProxied   string `form:"gzip_proxied" json:"gzip_proxied"`
	// Brotli 压缩
	Brotli          string `form:"brotli" json:"brotli"`
	BrotliMinLength string `form:"brotli_min_length" json:"brotli_min_length"`
	BrotliCompLevel string `form:"brotli_comp_level" json:"brotli_comp_level"`
	BrotliTypes     string `form:"brotli_types" json:"brotli_types"`
	BrotliStatic    string `form:"brotli_static" json:"brotli_static"`
	// Zstd 压缩
	Zstd          string `form:"zstd" json:"zstd"`
	ZstdMinLength string `form:"zstd_min_length" json:"zstd_min_length"`
	ZstdCompLevel string `form:"zstd_comp_level" json:"zstd_comp_level"`
	ZstdTypes     string `form:"zstd_types" json:"zstd_types"`
	ZstdStatic    string `form:"zstd_static" json:"zstd_static"`
}

type StreamServer struct {
	Name                string        `form:"name" json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"`                   // 配置名称，用于文件命名
	Listen              string        `form:"listen" json:"listen" validate:"required"`                                      // 监听地址，如: "12345", "0.0.0.0:12345", "[::]:12345"
	UDP                 bool          `form:"udp" json:"udp"`                                                                // 是否 UDP 协议
	ProxyPass           string        `form:"proxy_pass" json:"proxy_pass" validate:"required"`                              // 代理地址，如: "127.0.0.1:3306", "upstream_name"
	ProxyProtocol       bool          `form:"proxy_protocol" json:"proxy_protocol"`                                          // 是否启用 PROXY 协议
	ProxyTimeout        time.Duration `form:"proxy_timeout" json:"proxy_timeout"`                                            // 代理超时时间
	ProxyConnectTimeout time.Duration `form:"proxy_connect_timeout" json:"proxy_connect_timeout"`                            // 代理连接超时时间
	SSL                 bool          `form:"ssl" json:"ssl"`                                                                // 是否启用 SSL
	SSLCertificate      string        `form:"ssl_certificate" json:"ssl_certificate" validate:"requiredIf:SSL,true"`         // SSL 证书路径
	SSLCertificateKey   string        `form:"ssl_certificate_key" json:"ssl_certificate_key" validate:"requiredIf:SSL,true"` // SSL 私钥路径
}

type StreamUpstream struct {
	Name            string            `form:"name" json:"name" validate:"required|regex:^[a-zA-Z0-9_-]+$"` // 上游名称
	Servers         map[string]string `form:"servers" json:"servers" validate:"required"`                  // 上游服务器及配置，如: map["127.0.0.1:3306"] = "weight=5"
	Algo            string            `form:"algo" json:"algo"`                                            // 负载均衡算法，如: "least_conn", "hash $remote_addr"
	Resolver        []string          `form:"resolver" json:"resolver"`                                    // DNS 解析器，如: ["8.8.8.8", "ipv6=off"]
	ResolverTimeout time.Duration     `form:"resolver_timeout" json:"resolver_timeout"`                    // DNS 解析超时时间
}
