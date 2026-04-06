package request

import "net/http"

type CertUpload struct {
	Cert string `form:"cert" json:"cert" validate:"required"`
	Key  string `form:"key" json:"key" validate:"required"`
}

type CertCreate struct {
	Type        string            `form:"type" json:"type" validate:"required|in:P256,P384,2048,3072,4096"`
	Domains     []string          `form:"domains" json:"domains" validate:"required|isSlice"`
	Alias       map[string]string `form:"alias" json:"alias"`
	AutoRenewal bool              `form:"auto_renewal" json:"auto_renewal"`
	AccountID   uint              `form:"account_id" json:"account_id"`
	DNSID       uint              `form:"dns_id" json:"dns_id"`
	WebsiteID   uint              `form:"website_id" json:"website_id"`
}

func (r *CertCreate) Rules(_ *http.Request) map[string]string {
	return map[string]string{
		"Domains.*": "required",
	}
}

type CertUpdate struct {
	ID          uint              `form:"id" json:"id" validate:"required|exists:certs,id"`
	Type        string            `form:"type" json:"type" validate:"required|in:P256,P384,2048,3072,4096,upload"`
	Domains     []string          `form:"domains" json:"domains" validate:"required|isSlice"`
	Alias       map[string]string `form:"alias" json:"alias"`
	Cert        string            `form:"cert" json:"cert"`
	Key         string            `form:"key" json:"key"`
	Script      string            `form:"script" json:"script"`
	AutoRenewal bool              `form:"auto_renewal" json:"auto_renewal"`
	AccountID   uint              `form:"account_id" json:"account_id"`
	DNSID       uint              `form:"dns_id" json:"dns_id"`
	WebsiteID   uint              `form:"website_id" json:"website_id"`
}

func (r *CertUpdate) Rules(_ *http.Request) map[string]string {
	return map[string]string{
		"Domains.*": "required",
	}
}

type CertDeploy struct {
	ID          uint `form:"id" json:"id" validate:"required|exists:certs,id"`
	WebsiteID   uint `form:"website_id" json:"website_id" validate:"required|exists:websites,id"`
	EnableHTTPS bool `form:"enable_https" json:"enable_https"`
}
