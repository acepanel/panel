package stalwart

import "time"

// Stalwart 对象（字段名与管理 API 一致）

type swDomain struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Aliases         map[string]bool `json:"aliases"`
	Enable          bool            `json:"enable"`
	CreatedAt       time.Time       `json:"createdAt"`
	CatchAllAddress *string         `json:"catchAllAddress"`
	DNSZoneFile     string          `json:"dnsZoneFile"`
}

type swAlias struct {
	Enable   bool   `json:"enable"`
	Name     string `json:"name"`
	DomainID string `json:"domainId"`
}

type swAccount struct {
	ID            string             `json:"id"`
	Type          string             `json:"@type"`
	Name          string             `json:"name"`
	DomainID      string             `json:"domainId"`
	EmailAddress  string             `json:"emailAddress"`
	Description   *string            `json:"description"`
	Quotas        map[string]uint64  `json:"quotas"`
	UsedDiskQuota uint64             `json:"usedDiskQuota"`
	Aliases       map[string]swAlias `json:"aliases"`
	CreatedAt     time.Time          `json:"createdAt"`
}

type swListener struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Bind        map[string]bool `json:"bind"`
	Protocol    string          `json:"protocol"`
	UseTLS      bool            `json:"useTls"`
	ImplicitTLS bool            `json:"implicitTls"`
}

type swSystemSettings struct {
	DefaultHostname      string  `json:"defaultHostname"`
	DefaultDomainID      string  `json:"defaultDomainId"`
	DefaultCertificateID *string `json:"defaultCertificateId"`
}

type swApplication struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	ResourceURL string          `json:"resourceUrl"`
	URLPrefix   map[string]bool `json:"urlPrefix"`
}

type swRoute struct {
	ID           string  `json:"id"`
	Type         string  `json:"@type"`
	Name         string  `json:"name"`
	Host         string  `json:"host"`
	Port         uint    `json:"port"`
	AuthUsername *string `json:"authUsername"`
	ImplicitTLS  bool    `json:"implicitTls"`
}

type swExpression struct {
	Match map[string]struct {
		If   string `json:"if"`
		Then string `json:"then"`
	} `json:"match"`
	Else string `json:"else"`
}

type swOutboundStrategy struct {
	Route swExpression `json:"route"`
}

type swRecipientStatus struct {
	Type            string  `json:"@type"`
	ErrorType       string  `json:"errorType"`
	ErrorMessage    *string `json:"errorMessage"`
	ResponseCode    *uint   `json:"responseCode"`
	ResponseMessage *string `json:"responseMessage"`
}

type swRecipient struct {
	RetryCount uint              `json:"retryCount"`
	RetryDue   time.Time         `json:"retryDue"`
	Status     swRecipientStatus `json:"status"`
}

type swQueuedMessage struct {
	ID         string                 `json:"id"`
	CreatedAt  time.Time              `json:"createdAt"`
	NextRetry  *time.Time             `json:"nextRetry"`
	ReturnPath string                 `json:"returnPath"`
	Recipients map[string]swRecipient `json:"recipients"`
	Size       uint64                 `json:"size"`
}

// 面板返回给前端的结构

type Status struct {
	Installed   bool   `json:"installed"`
	Initialized bool   `json:"initialized"`
	Running     bool   `json:"running"`
	Hostname    string `json:"hostname"`
	DomainCount uint   `json:"domain_count"`
	HTTPPort    int    `json:"http_port"`
	LogPath     string `json:"log_path"`
	// 一键创建的反代网站，0 表示尚未创建；缺失的主机名说明新增域名后还没同步
	ProxyWebsiteID      uint     `json:"proxy_website_id"`
	ProxyWebsiteName    string   `json:"proxy_website_name"`
	ProxyDomains        []string `json:"proxy_domains"`
	ProxyMissingDomains []string `json:"proxy_missing_domains"`
}

type Domain struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	CatchAll  string    `json:"catch_all"`
	Aliases   []string  `json:"aliases"`
	IsDefault bool      `json:"is_default"`
}

type Account struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Description string    `json:"description"`
	Quota       uint64    `json:"quota"`
	Used        uint64    `json:"used"`
	Aliases     []string  `json:"aliases"`
	CreatedAt   time.Time `json:"created_at"`
}

type QueueRecipient struct {
	Address    string    `json:"address"`
	Status     string    `json:"status"`
	Error      string    `json:"error"`
	RetryCount uint      `json:"retry_count"`
	RetryDue   time.Time `json:"retry_due"`
}

type QueueMessage struct {
	ID         string           `json:"id"`
	CreatedAt  time.Time        `json:"created_at"`
	NextRetry  *time.Time       `json:"next_retry"`
	ReturnPath string           `json:"return_path"`
	Size       uint64           `json:"size"`
	Recipients []QueueRecipient `json:"recipients"`
}

// DNSRecord 是从 Stalwart 区域文件解析出的一条待发布记录及其线上状态
type DNSRecord struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	Value  string `json:"value"`
	Group  string `json:"group"`  // required / optional
	Status string `json:"status"` // ok / missing / mismatch / unknown
	Found  string `json:"found"`
	// 发布用的结构化字段，不返回前端
	priority uint16
	weight   uint16
	port     uint16
	target   string
}

type DNSResult struct {
	Zone    string      `json:"zone"`
	Records []DNSRecord `json:"records"`
}

type CheckItem struct {
	Key    string `json:"key"`
	Status string `json:"status"` // ok / warn / fail / skip
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type Listeners struct {
	Submission bool `json:"submission"` // 587 STARTTLS
	IMAP       bool `json:"imap"`       // 143 STARTTLS
	POP3       bool `json:"pop3"`       // 110 STARTTLS
}

type Relay struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        uint   `json:"port"`
	Username    string `json:"username"`
	ImplicitTLS bool   `json:"implicit_tls"`
}

type Settings struct {
	Hostname  string    `json:"hostname"`
	CertID    uint      `json:"cert_id"`
	Listeners Listeners `json:"listeners"`
	Relay     Relay     `json:"relay"`
}
