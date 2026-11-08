package stalwart

type Init struct {
	Hostname string `form:"hostname" json:"hostname" validate:"required && regex:\"^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$\""`
	Domain   string `form:"domain" json:"domain" validate:"required && regex:\"^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$\""`
}

type DomainCreate struct {
	Name string `form:"name" json:"name" validate:"required && regex:\"^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$\""`
}

type DomainID struct {
	ID string `form:"id" json:"id" validate:"required"`
}

type DomainCatchAll struct {
	ID      string `form:"id" json:"id" validate:"required"`
	Address string `form:"address" json:"address"`
}

type DomainDNSPublish struct {
	ID      string   `form:"id" json:"id" validate:"required"`
	DNSID   uint     `form:"dns_id" json:"dns_id" validate:"required"`
	Records []string `form:"records" json:"records" validate:"required"`
}

type AccountList struct {
	Page     uint   `form:"page" json:"page" query:"page"`
	Limit    uint   `form:"limit" json:"limit" query:"limit"`
	DomainID string `form:"domain_id" json:"domain_id" query:"domain_id"`
	Keyword  string `form:"keyword" json:"keyword" query:"keyword"`
}

type AccountCreate struct {
	Name        string   `form:"name" json:"name" validate:"required && regex:\"^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$\""`
	DomainID    string   `form:"domain_id" json:"domain_id" validate:"required"`
	Password    string   `form:"password" json:"password" validate:"required && password"`
	Description string   `form:"description" json:"description"`
	Quota       uint64   `form:"quota" json:"quota"`
	Aliases     []string `form:"aliases" json:"aliases"`
}

type AccountUpdate struct {
	ID          string   `form:"id" json:"id" validate:"required"`
	Description string   `form:"description" json:"description"`
	Quota       uint64   `form:"quota" json:"quota"`
	Aliases     []string `form:"aliases" json:"aliases"`
}

type AccountPassword struct {
	ID       string `form:"id" json:"id" validate:"required"`
	Password string `form:"password" json:"password" validate:"required && password"`
}

type AccountID struct {
	ID string `form:"id" json:"id" validate:"required"`
}

type QueueList struct {
	Page    uint   `form:"page" json:"page" query:"page"`
	Limit   uint   `form:"limit" json:"limit" query:"limit"`
	Keyword string `form:"keyword" json:"keyword" query:"keyword"`
}

type QueueID struct {
	ID string `form:"id" json:"id" validate:"required"`
}

type SettingHostname struct {
	Hostname string `form:"hostname" json:"hostname" validate:"required && regex:\"^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?(\\.[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+$\""`
}

type SettingCert struct {
	CertID uint `form:"cert_id" json:"cert_id"`
}

type SettingListeners struct {
	Submission bool `form:"submission" json:"submission"`
	IMAP       bool `form:"imap" json:"imap"`
	POP3       bool `form:"pop3" json:"pop3"`
}

type SettingRelay struct {
	Enabled     bool   `form:"enabled" json:"enabled"`
	Host        string `form:"host" json:"host" validate:"required_if:Enabled,true"`
	Port        uint   `form:"port" json:"port" validate:"required_if:Enabled,true && min:0 && max:65535"`
	Username    string `form:"username" json:"username"`
	Password    string `form:"password" json:"password"`
	ImplicitTLS bool   `form:"implicit_tls" json:"implicit_tls"`
}

type ProxyCreate struct {
	CertID uint `form:"cert_id" json:"cert_id"`
}
