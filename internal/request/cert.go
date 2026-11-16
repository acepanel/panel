package request

type CertUpload struct {
	Cert     string `form:"cert" json:"cert" validate:"required_without:CertPath"`
	Key      string `form:"key" json:"key" validate:"required_without:CertPath"`
	CertPath string `form:"cert_path" json:"cert_path"`
	KeyPath  string `form:"key_path" json:"key_path" validate:"required_with:CertPath"`
}

type CertCreate struct {
	Type        string            `form:"type" json:"type" validate:"required && in:P256,P384,2048,3072,4096"`
	Domains     []string          `form:"domains" json:"domains" validate:"required && unique && dive && required"`
	Alias       map[string]string `form:"alias" json:"alias"`
	AutoRenewal bool              `form:"auto_renewal" json:"auto_renewal"`
	AccountID   uint              `form:"account_id" json:"account_id"`
	DNSID       uint              `form:"dns_id" json:"dns_id"`
	WebsiteIDs  []uint            `form:"website_ids" json:"website_ids" validate:"unique"`
}

type CertUpdate struct {
	ID          uint              `form:"id" json:"id" validate:"required && exists:certs,id"`
	Type        string            `form:"type" json:"type" validate:"required && in:P256,P384,2048,3072,4096,upload"`
	Domains     []string          `form:"domains" json:"domains" validate:"required && unique && dive && required"`
	Alias       map[string]string `form:"alias" json:"alias"`
	Cert        string            `form:"cert" json:"cert"`
	Key         string            `form:"key" json:"key"`
	CertPath    string            `form:"cert_path" json:"cert_path"`
	KeyPath     string            `form:"key_path" json:"key_path" validate:"required_with:CertPath"`
	Script      string            `form:"script" json:"script"`
	AutoRenewal bool              `form:"auto_renewal" json:"auto_renewal"`
	AccountID   uint              `form:"account_id" json:"account_id"`
	DNSID       uint              `form:"dns_id" json:"dns_id"`
	WebsiteIDs  []uint            `form:"website_ids" json:"website_ids" validate:"unique"`
}

type CertDeploy struct {
	ID          uint   `form:"id" json:"id" validate:"required && exists:certs,id"`
	WebsiteIDs  []uint `form:"website_ids" json:"website_ids" validate:"required && unique"`
	EnableHTTPS bool   `form:"enable_https" json:"enable_https"`
}
