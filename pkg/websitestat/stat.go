package websitestat

// LogEntry 解析后的访问日志条目
type LogEntry struct {
	Site        string  // 来自 syslog tag
	URI         string  // 请求 URI
	Status      int     // HTTP 状态码
	Bytes       uint64  // 响应体大小
	UA          string  // User-Agent
	IP          string  // 客户端 IP
	Method      string  // 请求方法
	Body        string  // 请求体（仅 400-599 状态码时保留）
	ContentType string  // 响应 Content-Type（PV 判定用）
	ReqLength   uint64  // 请求大小（入站流量）
	RequestTime float64 // 请求耗时（秒）
}

// Counters 一组流量计数
type Counters struct {
	PV               uint64 `json:"pv"`
	UV               uint64 `json:"uv"`
	IP               uint64 `json:"ip"`
	Bandwidth        uint64 `json:"bandwidth"`
	BandwidthIn      uint64 `json:"bandwidth_in"`
	Requests         uint64 `json:"requests"`
	Errors           uint64 `json:"errors"`
	Spiders          uint64 `json:"spiders"`
	RequestTimeSum   uint64 `json:"request_time_sum"`
	RequestTimeCount uint64 `json:"request_time_count"`
	Status2xx        uint64 `json:"status_2xx"`
	Status3xx        uint64 `json:"status_3xx"`
	Status4xx        uint64 `json:"status_4xx"`
	Status5xx        uint64 `json:"status_5xx"`
}

// Add 逐项累加
func (c *Counters) Add(o Counters) {
	c.PV += o.PV
	c.UV += o.UV
	c.IP += o.IP
	c.Bandwidth += o.Bandwidth
	c.BandwidthIn += o.BandwidthIn
	c.Requests += o.Requests
	c.Errors += o.Errors
	c.Spiders += o.Spiders
	c.RequestTimeSum += o.RequestTimeSum
	c.RequestTimeCount += o.RequestTimeCount
	c.Status2xx += o.Status2xx
	c.Status3xx += o.Status3xx
	c.Status4xx += o.Status4xx
	c.Status5xx += o.Status5xx
}

// HourSnapshot 小时粒度快照
type HourSnapshot struct {
	Counters
}

// SiteSnapshot 站点快照（用于 DB flush）
type SiteSnapshot struct {
	Counters
	Hours [24]*HourSnapshot `json:"-"`
}

// RealtimeStats 实时统计
type RealtimeStats struct {
	Bandwidth   float64 `json:"bandwidth"`    // 出站字节/秒
	BandwidthIn float64 `json:"bandwidth_in"` // 入站字节/秒
	RPS         float64 `json:"rps"`          // 请求/秒
}

// ErrorEntry 错误日志条目
type ErrorEntry struct {
	Site   string
	URI    string
	Method string
	IP     string
	UA     string
	Body   string
	Status int
}

// SiteDetailSnapshot 站点详细统计快照（用于 DrainDetailStats）
type SiteDetailSnapshot struct {
	Spiders map[string]uint64       // spider_name → requests
	Clients map[string]*ClientCount // "browser|os" → count
	IPs     map[string]*IPCount     // ip → count
	URIs    map[string]*URICount    // uri → count
}

// ClientCount 客户端计数
type ClientCount struct {
	Requests uint64
}

// IPCount IP 计数
type IPCount struct {
	Requests  uint64
	Bandwidth uint64
}

// URICount URI 计数
type URICount struct {
	Requests         uint64
	Bandwidth        uint64
	Errors           uint64
	RequestTimeSum   uint64
	RequestTimeCount uint64
}
